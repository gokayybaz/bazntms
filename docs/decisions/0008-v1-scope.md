# 0008 — v1.0 kapsamı: API / protokol kararlılık taahhüdü (Faz 21)

**Tarih:** 2026-09-08 · **Durum:** kabul edildi — kapasite hedefleri canlı
doğrulandı (`docs/CAPACITY.md`), Faz 21 kapanışında `v1.0.0` etiketlenir.

## Sorun

Yol haritasındaki kurumsal modüller (0–10) ve mimari değerlendirme fazları
(11–20) tamamlandı; Faz 21 kurumsal kapasite hedeflerini (5.000 agent /
1.000 cihaz / ≥ 50k flow/sn / panel p95 < 1 sn) sentetik yükle ölçüp
doğruladı. Sürüm `0.x` iken SemVer sözleşmesi gevşekti: minor sürümler
kırıcı değişiklik taşıyabiliyordu (CHANGELOG başlığındaki "v1.0.0'a kadar
minor = özellik" notu). Operatörlerin ve entegrasyonların (HTTP API
tüketicileri, agent filoları, Helm chart) güvenle yükseltebilmesi için
**neyin sabit kaldığını** açıkça ilan etmek gerekiyor.

## Karar

`v1.0.0` ile aşağıdaki yüzeyler **kararlı** ilan edilir. v1.x boyunca bunlarda
**kırıcı değişiklik yapılmaz**; kırıcı değişiklik `v2.0.0` gerektirir.

### 1. HTTP API (`api/openapi.yaml`)

- `/api/v1/*` uçları, istek/yanıt şemaları, alan adları, HTTP durum kodları
  ve kimlik doğrulama davranışı sabittir.
- **Geriye uyumlu ekleme serbesttir:** yeni uç, yanıta yeni alan, yeni
  opsiyonel istek parametresi, yeni enum değeri → minor sürüm.
- İstemciler bilinmeyen JSON alanlarını **yok saymalı** (tolerant reader).
- Kaldırma / yeniden adlandırma / zorunlu alan ekleme / anlam değişikliği →
  major. Kaldırılacak bir uç önce en az bir minor sürüm `Deprecation`
  başlığıyla işaretlenir.
- `info.version` OpenAPI belgesinde SemVer olarak izlenir (`1.0.0`).

### 2. Agent ↔ hub protokolü (`internal/version.ProtocolVersion`)

- `ProtocolVersion = 1`. Handshake (`/api/v1/agents/hello`) + telemetri
  batch şeması (`pkg/telemetry`) v1 boyunca `1` kalır.
- **Uyumluluk politikası:** hub, `hello.protocol_version` alanını
  desteklediği `maxProtocolVersion`'a **düşürür** (graceful degrade,
  Faz 21 / S21.15) ve yanıtta gerçek `protocol_version`'ı bildirir; agent
  bunu okuyup kendi çıktısını ona göre kısar. Yani:
  - **Yeni agent + eski hub** → agent hub'ın bildirdiği sürüme iner, çalışır.
  - **Eski agent + yeni hub** → hub eski şemayı kabul eder.
- Protokolde geriye uyumlu alan eklemesi minor `ProtocolVersion` artışı
  (`1` → sonraki minor sürümde gerekirse `2`); kırıcı değişiklik major.
  v1.x'te hedef: `ProtocolVersion` sabit `1`.
- Telemetride `ClampTS` (saat kayması) ve offline kuyruk + replay davranışı
  protokolün parçası sayılır — istemci saati bozuksa veri `now`'a çekilir,
  ağ kesintisinde batch'ler diske kuyruklanır.

### 3. Depolama / migrasyon

- Şema yalnız **ileri** migrasyonla değişir (`internal/store/migrations/`),
  geri alma yok — bu v1 öncesinde de böyleydi, taahhüt sürüyor.
- SQLite (tek-node) ve PostgreSQL/TimescaleDB (ölçek) yollarının **ikisi de**
  desteklenir. TimescaleDB continuous aggregate'leri olmayan düz PostgreSQL
  de çalışır (cagg bağımlı uçlar ham tabloya düşer).
- Yeni cagg / index eklemek kırıcı değil; mevcut kurulumda **bir kez
  geri-doldurma** gerekebilir (`docs/UPGRADE-RUNBOOK.md`).

### 4. Yapılandırma yüzeyi

- Agent config alanları (`agent.yml`), hub CLI bayrakları ve ortam
  değişkenleri (`BAZNTMS_*`) v1.x boyunca **anlamını korur**. Yeni bayrak /
  env eklenebilir; varsayılan davranış değişmez.
- Faz 21'de eklenenler kararlı sayılır: `BAZNTMS_DB_MAX_CONNS`,
  `GOMEMLIMIT` (Go runtime'ı), `-queue-workers`, `-devpoll-concurrency`,
  `-pprof-rates`, `-mock-devices` (sonuncusu yalnız yük testi).

### 5. Helm chart

- `deploy/helm/bazntms` chart `version` + `appVersion` sürüm etiketinden
  türetilir (`v1.0.0` → `1.0.0`). `values.yaml` anahtarları v1.x boyunca
  korunur.

## Kapsam dışı — v1'de **kasıtlı olarak yok**

Aşağıdakiler bilinçli olarak ertelendi. Yokluğu bir eksiklik değil, kapsam
kararıdır; gelecekte eklenirse geriye uyumlu (minor) olacak şekilde
tasarlanır.

| Özellik | Neden ertelendi | İz |
|---|---|---|
| **ETW L7 (WinINet / WinHTTP / Schannel)** | Windows'ta Npcap'siz kısmi SNI/Host mümkün ama OS HTTP yığını dışındaki app'leri (Chromium/Firefox/Electron) kapsamaz; ayrı sağlayıcı + ayrıştırma işi. L7 v1'de **pcap-gated** kalır. | `0007` §4, "Kalan" |
| **macOS Endpoint Security** | Süreç-atıflı akış için `NEFilterDataProvider` — notarization + `com.apple.developer.networking` entitlement + ayrı dağıtım kanalı maliyeti. macOS agent v1'de **pcap-only**. | `0007` §6 |
| **Gerçek KMS / zarf şifreleme (vault)** | `KeyProvider` arayüzü var (`-vault-key-source=env` — master anahtar diske yazılmaz), ama AWS KMS / GCP KMS / Vault Transit ile zarf şifreleme (DEK/KEK) uygulanmadı. | `0006` |
| **Çok-kiracılılık (`tenant_id`)** | Saha-kapsam RBAC (`-multi-site`, `site-admin`) MSP senaryosunu karşılıyor ama gerçek kiracı izolasyonu (satır düzeyi `tenant_id`, kiracı başına kota / şifreleme / yedek) yok. Şema + sorgu katmanında geniş değişiklik ister. | `0005` (HA), Faz 14 |

## Sonuç

`v1.0.0`, "kurumsal ölçekte çalıştığı ölçülmüş + API/protokol kararlı"
anlamına gelir — özellik-tam değil. Yukarıdaki 4 kapsam-dışı madde
gelecekteki fazların konusu; hiçbiri v1.x uyumluluğunu kırmadan eklenebilir.
