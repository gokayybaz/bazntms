# Yük Testi

Kapasite hedefleri (`docs/enterprise-plan.html` → "Kapasite Hedefleri"):

| Hedef | Değer |
|---|---|
| Cihaz poll | 1.000 cihaz, 60 sn döngü |
| Agent telemetri | 5.000 agent, 30 sn batch → **≥170 ist/sn sürekli** |
| Flow işleme | ≥50.000 flow/sn sürekli; 200K flow/sn 5 dk kayıpsız (NATS buffer) |
| Depolama | ham 7g → 1dk 90g → 1sa 2y; panel sorgusu p95 < 1 sn |

## Sentetik Agent Filosu (birincil araç)

Gerçek agent protokolünü (hello/telemetry JSON) birebir oynatır:

```bash
# hub'ı kuyruk + postgres ile başlat (bkz deploy/docker-compose.yml)
docker compose -f deploy/docker-compose.yml up -d

# demo token ile 5.000 agent, 10 dakika
go run ./cmd/bazntms-loadgen -hub http://localhost:8080 \
  -token demo-enroll-token -agents 5000 -interval 30 -duration 10m
```

Çıktı: 5 saniyede bir `rps`, `p50/p95/p99` gecikme özeti.

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
