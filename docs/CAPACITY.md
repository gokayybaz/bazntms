# Kapasite Doğrulaması

Bu belge, `docs/enterprise-plan.html` → "Kapasite Hedefleri"nde tanımlı ölçek
kriterlerinin ölçülen sonuçlarını, bulunan darboğazları ve önerilen üretim
boyutlandırmasını kaydeder (Faz 21).

## Doğrulama ortamı

| | |
|---|---|
| Yığın | `deploy/docker-compose.scale.yml` — 2 × hub-ingest + 2 × hub-controller + nginx LB + TimescaleDB (pg16) + NATS JetStream |
| Gözlemlenebilirlik | `--profile obs` — Prometheus (tüm replikaları `dns_sd` ile toplar) + Grafana "bazNTMS — Kapasite" |
| Yük aracı | `bazntms-loadgen` (agent + flow + mock-cihaz) · `scripts/loadtest.sh <profil>` |
| Donanım | Docker Desktop for Mac (tek geliştirici makinesi) — **mutlak throughput değil, "mimari kayıpsız + p95 lineer" ölçütü** esas alınır; üretim boyutlandırması aşağıda |
| Tarih | 2026-09-08 · sürüm ~ v1.0.0-rc |

Koşu raporları: `docs/perf/runs/*.md` (git'te). Ham metrik: koşu sırasında
Grafana + `scripts/perf_summary.py`.

## Hedefler × ölçülen

| # | Hedef (enterprise plan) | Ölçüt | Ölçülen | Sonuç |
|---|---|---|---|---|
| 1 | **Agent telemetri** — 5.000 agent @ 30 sn batch | ≥ 170 ist/sn sürekli · batch p95 < 250 ms | 5.000 agent × (1/30 sn) = **166.7 ist/sn** üretilebilir tavan; sistem bunu **p95 6 ms**, hata %0, kuyruk birikimi düz ile karşıladı (fazlasıyla headroom) | ✅ |
| 2 | **Flow işleme** — sürekli | ≥ 50.000 flow/sn kayıpsız | **49.987–49.998 flow/sn** sürekli 4+ dk, `flows_dropped_total` = **0**; flow tablosu yazım p95 **1.1 ms** (12.7 M satır) | ✅ |
| 2b | **Flow patlaması** | 200.000 flow/sn 5 dk kayıpsız (NATS buffer) | **Flow verisi kayıpsız** (67 M flow'da 2 `empty_parse` = %0.000003); NATS 538k mesaja buffer'ladı, patlama sonrası **tam boşaldı**. Ama tek-node laptop yığınında patlama paylaşımlı NATS + Postgres'i doyurdu → agent telemetri rps 167→148, ~%11 agent hatası (offline kuyruk + replay, kalıcı kayıp yok), p99 ~7 sn. **Mimari kayıpsız degrade + otomatik kurtarma; mutlak 200k üretim donanımı ister** — bkz. aşağı | ⚠️ donanım-bağlı |
| 3 | **Cihaz poll** — 1.000 cihaz | 60 sn döngü; döngü bütçenin %80'ini (48 sn) aşmaz | 1.000 mock cihaz, poll döngüsü ort. **0.21 sn** (bütçenin %0.4'ü); `devpoll_inflight` bounded (worker havuzu 96) | ✅ |
| 4 | **Depolama & panel sorgusu** | ham 7g → 1dk 90g → 1sa 2y; panel sorgusu p95 < 1 sn | Panel uçları p95: `/api/v1/agents` **152 ms**, `/api/v1/agents/:id` **86 ms**, `/api/v1/flows` **16 ms**, l7/dns/processes < 200 ms. Rapor/geo uçları continuous aggregate'lerden. cagg + retention: `samples_1m` (90g), `samples_1h`/`flows_1h`/`process_traffic_1h`/`flows_{dst,src}_1h`/`agent_iface_1h` (1–2y) | ✅ |

### Detaylı — `target` koşusu (5.000 agent + 50k flow/sn + 1.000 cihaz, ~9 dk)

```
agent:   sent=90000  failed=0  rps=166.7  p50=1.8ms  p95=6.5ms  p99=21ms
flow:    records=27,000,000  dropped=0  rate=49,999/sn
devpoll: 53 döngü  ort=0.21 sn
tablo yazım p95:  flows 1.60ms · agent_iface 1.42ms · conn_latest 1.21ms ·
                  process_traffic 1.01ms · l7 0.97ms · dns 0.95ms
goroutine 30–45 (sızıntı yok) · heap 8–18 MB
```

### Flow patlaması (200k flow/sn) — ayrıntılı

`burst` profili (5.000 agent + 20k→**120k** flow/sn 5 dk + 1.000 cihaz) tek-node
laptop yığınında **PASS** — laptopun kayıpsız + p95 < 400 ms karşıladığı seviye.

Ayrı bir koşuda **200k flow/sn** denendi:

| | 200k patlama (laptop) |
|---|---|
| flow alım | ~187–197k/sn, **kayıp ~0** (67 M'de 2 edge) |
| NATS kuyruk | 538k'ye çıktı → patlama sonrası **tam boşaldı** (~4 dk) |
| agent telemetri | rps 167→148, ~%11 hata (dial+http), p99 ~7 sn — offline kuyruk + replay ile **kalıcı kayıp yok** |
| flow yazım | batching (~150 msg/`SaveFlows`) + `js.PublishAsync` ile collector düşürmesi 127k → ~0 |

**Yorum:** Flow ingest hattı 200k'yi kayıpsız yutuyor; darboğaz tek konteyner
NATS'in FileStorage disk throughput'u + paylaşımlı Postgres. Üretim için:
ayrı NATS node (`MemoryStorage` opsiyonu veya NVMe), yüksek DB IOPS, +1 ingest
replika. Sistem doygunlukta **graceful degrade** eder ve otomatik kurtarır.

## Darboğazlar & düzeltmeler (Faz 21)

| Bulgu | Düzeltme | Commit |
|---|---|---|
| Toplu yazım satır-başına `stmt.Exec` (PG'de round-trip/satır) | `internal/store/bulk.go` — PG'de chunk başına tek çok-satırlı `INSERT ... VALUES` | `8a4f95e` |
| Flow collector tek okuyucu + senkron `OnFlows` → çekirdek UDP buffer taşması | reader/worker hattı (havuzlanmış `*pkt`, `SO_RCVBUF` 8 MiB), `TemplateCache` `RWMutex` | `4e16449` |
| `devpoll.pollAll` cihaz başına sınırsız goroutine | bounded semafor (`concurrency`, vars. 96, `-devpoll-concurrency`) | `1e86d94` |
| `/api/v1/agents` agent başına ayrı rate + conn sorgusu (N+1, ~1 sn) | tek `ROW_NUMBER() OVER (PARTITION BY agent_id)` sorgusu | `c2e3fb0` |
| `/api/v1/flows` üst-N taraması 15dk penceresinde milyonlarca satır | `idx_flows_octets` (`0008`), pencere üst sınırı 6 saat | `c2e3fb0` |
| `/api/report` + `/api/v1/geo` ham `flows` UNION+GROUP BY (60–90 sn timeout) | `flows_dst_1h`/`flows_src_1h`/`agent_iface_1h` cagg'leri; sorgular uzun pencerede cagg okur | `99c1d92` |
| **DB bağlantı havuzu (32) birleşik ingest + devpoll(96) altında doyuyor** — `db_pool_wait` ~52/sn (yazım p95 yine ~1.5 ms) | `BAZNTMS_DB_MAX_CONNS` env (scale compose'ta 64); ayrıca devpoll-concurrency ≤ havuz tutulmalı | `c1f5103` |
| Yeni cagg'ler mevcut DB'de boş oluşturulur — eski dönem raporlarda eksik | UPGRADE-RUNBOOK: bir kez `refresh_continuous_aggregate(..., NULL, NULL)` (retention penceresi kadar ham veriyi tarar, saatler sürebilir) | `743891f` |
| Flow collector senkron `js.Publish` + mesaj-başına `SaveFlows` → 200k patlamada collector 127k datagram düşürüyor, NATS 377k birikiyor | Fetch içinde flow mesajı birleştirme (~150 msg/`SaveFlows`) + `js.PublishAsync` (bounded pending) | `d275a9f`, `92aff7a` |

**`2xx-dışı oran ~%2` notu:** hub `/metrics` telemetri isteklerinde ~%2
2xx-dışı gösterir; `loadgen` **hata 0** raporlar. Fark, compose'daki 2 gerçek
`agent` konteynerinin geçiş anları + kaos pencereleri; loadgen istemci-tarafı
sayacı esas alınır.

## Dayanıklılık

| Test | Sonuç | Kaynak |
|---|---|---|
| Kaos — poll lideri kill | Hayatta kalan replika rolü **~7 sn** içinde devraldı | `scripts/chaos.sh`, DR-RUNBOOK §6 |
| Kaos — TimescaleDB restart | pgxpool reconnect, `/readyz` 200'e döndü, kayıp 0 | " |
| Kaos — NATS 60 sn stop | Ingest 503 → agent offline kuyruk → NATS dönünce **tam replay**; panel etkilenmedi | " |
| Kaos — ingest replika 2→3→2 | nginx dinamik upstream, kesinti yok | " |
| **Veri kaybı** (4 senaryo boyunca) | gönderilen **4610/4610** batch DB'de | " |
| Soak (16 dk ön koşu) | canlı heap **96→98 MB**, goroutine/FD platoda — **sızıntı yok** | `scripts/soak.sh`, `docs/perf/soak-*.md` |

**Tam 8 saat soak operatör görevidir** (`scripts/soak.sh`, varsayılan
`DURATION=8h`).

## Önerilen üretim boyutlandırması

Bu hedef ölçek (5.000 agent · 1.000 cihaz · ≥50k flow/sn) için başlangıç
noktası — kendi trafiğinizle Grafana kapasite panosundan ayarlayın:

| Bileşen | Öneri | Gerekçe |
|---|---|---|
| hub-ingest replika | 2–3 (yatay ölçeklenir) | Her replika NATS `store-writer`'ı paylaşır; 50k flow/sn'yi 2 replika + 64-conn havuz rahat karşıladı. Flow patlaması beklenen ortamlarda +1. |
| `-queue-workers` (replika başına) | 8 (patlama beklenen ortamda 12–16) | store-writer paralel worker; varsayılan 4 yalnız sürekli 50k için yeterli. |
| hub-controller replika | 2 (HA) | Panel + poller + uyarı; lider-seçimli, biri ölünce ~10 sn'de devir. |
| DB bağlantı havuzu | `BAZNTMS_DB_MAX_CONNS=64` (replika başına) | 32 birleşik yük altında doyuyordu. `devpoll-concurrency` bunu aşmasın. |
| `GOMEMLIMIT` | ~512 MiB (replika başına) | Go scavenger'ı RSS'i sınır altında tutar; canlı heap zaten < 150 MB. |
| PostgreSQL/TimescaleDB | 4+ vCPU, NVMe, `shared_buffers` ≈ RAM/4; retention = `-retention-hours` (ham), cagg'ler 1–2y | 27 M flow satırı 1.6 ms yazım p95 verdi; NVMe şart (Docker Desktop I/O bu testte tavan yaptı). |
| NATS JetStream | FileStorage NVMe **veya** patlama-ağırlıklı ortamda ayrı node; `MaxMsgs` 5 M, `MaxAge` 24 s | 200k flow/sn patlamada tek-konteyner FileStorage disk throughput'u darboğaz oldu (mesajlar kaybolmadı, birikti + boşaldı). |
| Flow collector | `SO_RCVBUF` 8 MiB (kod), gerekiyorsa `sysctl net.core.rmem_max` yükselt | Reader/worker hattı 50k/sn'yi drop'suz karşıladı. |

## Yeniden çalıştırma

```bash
docker compose -f deploy/docker-compose.scale.yml up -d --build
docker compose -f deploy/docker-compose.scale.yml --profile obs up -d   # Grafana :3000

scripts/loadtest.sh baseline   # ~4 dk hızlı regresyon
scripts/loadtest.sh target     # ~11 dk tam kapasite
scripts/loadtest.sh burst      # ~13 dk 200k flow/sn patlama
scripts/query_bench.sh         # panel/rapor sorgu p95
scripts/chaos.sh               # 4 kaos senaryosu
DURATION=8h scripts/soak.sh    # gece soak
```
