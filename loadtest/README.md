# Yük Testi

Kapasite hedefleri (`docs/enterprise-plan.html` → "Kapasite Hedefleri"):

| Hedef | Değer |
|---|---|
| Cihaz poll | 1.000 cihaz, 60 sn döngü |
| Agent telemetri | 5.000 agent, 30 sn batch → **≥170 ist/sn sürekli** |
| Flow işleme | ≥50.000 flow/sn sürekli; 200K flow/sn 5 dk kayıpsız (NATS buffer) |
| Depolama | ham 7g → 1dk 90g → 1sa 2y; panel sorgusu p95 < 1 sn |

## `bazntms-loadgen` (birincil araç)

`-mode` ile üç yük türü: `agent` (varsayılan), `flow`, `mixed` (ikisi birden).

### Agent telemetri filosu

Gerçek agent protokolünü (hello/telemetry JSON) birebir oynatır:

```bash
# hub'ı kuyruk + postgres ile başlat (bkz deploy/docker-compose.yml)
docker compose -f deploy/docker-compose.yml up -d

# demo token ile 5.000 agent, 10 dakika
go run ./cmd/bazntms-loadgen -hub http://localhost:8080 \
  -token demo-enroll-token -agents 5000 -interval 30 -duration 10m
```

Çıktı: 5 saniyede bir `rps`, `p50/p95/p99` gecikme özeti.

### Flow üreteci (S21.1)

Sentetik NetFlow v5/v9 + IPFIX + sFlow v5 datagramları — hub'ın `-flow-port`
dinleyicisine. Her sahte exporter tek protokole bağlı; v9/IPFIX şablonları
periyodik yenilenir.

```bash
# sürekli 50k flow/sn, 8 exporter, karışık protokol
go run ./cmd/bazntms-loadgen -mode flow \
  -flow-target 127.0.0.1:2055 -flow-rate 50000 -flow-proto mix -flow-exporters 8

# taban 20k, 4. dakikada 5 dk boyunca 200k patlama
go run ./cmd/bazntms-loadgen -mode flow -flow-target 127.0.0.1:2055 \
  -flow-rate 20000 -flow-burst 200000 -flow-burst-after 4m -flow-burst-for 5m -duration 15m
```

Doğrulama: hub `/metrics` → `bazntms_flows_received_total{version}` hızı
`-flow-rate`'e eşit, `bazntms_flows_dropped_total` == 0.

### Cihaz poll filosu (S21.2)

Hub'a `vendor=mock` cihaz ekler; hub'ın kendi `devpoll` zamanlayıcısı bu filoyu
yoklar (mock sürücü ağ I/O yapmaz, deterministik sayaç üretir). Hub
**`-mock-devices`** ile başlatılmalı.

```bash
# hub: mock cihaz + panel şifresi
bazntms-hub -db postgres://... -mock-devices -auth-password <pw>

# loadgen: 1000 mock cihaz, 60 sn poll
go run ./cmd/bazntms-loadgen -mode device -hub http://localhost:8080 \
  -password <pw> -devices 1000 -device-poll 60 -duration 30m
```

Doğrulama: hub `/metrics` → `bazntms_devpoll_cycle_duration_seconds` p95
bütçenin %80'i (48 sn) altında; `bazntms_devpoll_inflight` sınırlı (goroutine
sızıntısı yok); `bazntms_store_write_rows_total{table="device_iface_samples"}`
artıyor.

## Birleşik Koşucu — `scripts/loadtest.sh` (S21.3)

Bir profil (`loadtest/profiles/<ad>.env`) alır, çalışan bir hub yığınına karşı
agent + flow + cihaz yükünü **birlikte** sürer, `/metrics`'i ölçüm penceresinin
başında/sonunda örnekler, `scripts/perf_summary.py` ile profil eşiklerine karşı
PASS/FAIL raporu üretir (`docs/perf/runs/<utc>-<ad>.md`; çıkış kodu ihlalde 1).

```bash
docker compose -f deploy/docker-compose.scale.yml up -d --build

# canlı panolar (S21.7): Prometheus :9090 + Grafana :3000 (anon admin)
docker compose -f deploy/docker-compose.scale.yml --profile obs up -d
#   → http://localhost:3000 → "bazNTMS — Kapasite" dashboard

# hızlı regresyon (~3 dk) / tam kapasite (~10 dk) / patlama (~12 dk)
scripts/loadtest.sh baseline
scripts/loadtest.sh target
scripts/loadtest.sh burst

# yük altında profil topla (hub -pprof ile başlatılmışsa)
scripts/profile.sh 30
```

Ortam değişkenleriyle hedeflenir: `HUB_PANEL` (:8080), `HUB_AGENT` (:8081),
`FLOW_TARGET` (127.0.0.1:12055), `ENROLL_TOKEN`, `AUTH_PASSWORD`, `METRICS_URL`,
`STACK`. Profiller: `AGENTS`/`INTERVAL`/`FLOW_RATE`/`FLOW_BURST*`/`DEVICES`/
`DURATION`/`WARMUP` + `MAX_P95_MS`/`MIN_TELEMETRY_RPS`/`MAX_QUEUE_PENDING`/
`MAX_FLOW_DROP_RATE`/`MAX_DEVPOLL_CYCLE_S` eşikleri.

> Tek-makine SQLite'a karşı `mode=device` + eşzamanlı agent enroll'ü yazıcı
> kilidi (SQLITE_BUSY) yaratır — kapasite koşuları Postgres/Timescale yığınına
> karşı yapılır.

## k6 Senaryosu (alternatif)

Open-loop constant-arrival-rate: ritim VU sayısından bağımsız korunur.

```bash
k6 -e HUB=http://localhost:8080 -e ENROLL_TOKEN=demo-enroll-token \
   -e RATE=170 -e DURATION=10m loadtest/k6-ingest.js
```

## Doğrulama Kontrolleri

Hub `/metrics` (S21.5 ile eklenen ingest hattı serileri):

1. **Ingest tıknamıyor mu?** `bazntms_http_requests_total`
   (path `/api/v1/agent/telemetry`) hızı ≥170/sn, hata oranı ~0;
   `bazntms_queue_pending` düz (sürekli artmıyor = writer yetişiyor).
2. **Darboğaz nerede?** `bazntms_store_write_duration_seconds` histogramı
   (tablo bazında) — hangi tabloya yazım pahalı; `bazntms_telemetry_decode_duration_seconds`
   JSON çözme payı; `bazntms_db_pool_in_use` / `bazntms_db_pool_wait_count_total`
   havuz doygunluğu.
3. **Flow:** `bazntms_flows_received_total{version}` hızı hedefe eşit mi;
   `bazntms_flows_dropped_total{reason}` == 0 (çekirdek soket taşması bu
   katmanda görünmez — S21.9).
4. **Cihaz poll:** `bazntms_devpoll_cycle_duration_seconds` p95 bütçenin %80'i
   altında; `bazntms_devpoll_inflight` sınırlı (goroutine sızıntısı yok).
5. **Kuyruk sağlıklı mı?** `nats stream info BAZNTMS` — consumer
   `store-writer` ack-pending dengesi; yazım hatalarında Nak+retry akışı.
6. **Depolama alt kriteri:** TimescaleDB modunda `samples_1m`/`samples_1h`
   continuous aggregate'ları oluştu mu, retention politikaları aktif mi
   (`timescaledb_information.jobs`).
7. **Sorgu p95 < 1 sn:** 24 saatlik panel (history endpoint) süresi.
