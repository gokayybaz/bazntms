# Yapılandırma Referansı

## Komut Satırı Bayrakları

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `-port` | `8080` | HTTP sunucu portu (0.0.0.0'a bağlanır) |
| `-dev` | `false` | Frontend embed'ini atla; Vite dev server ile geliştirme |
| `-db` | `bazntms.db` | **SQLite** dosya yolu **veya** `postgres://` DSN (Faz 4.1: DSN ile PostgreSQL/TimescaleDB modu) |
| `-retention-hours` | `168` (7 gün) | SQLite/Prune modunda saklama süresi; eski kayıtlar 10 dakikada bir silinir. TimescaleDB modunda retention politikaları devrededir |
| `-nats` | — | NATS JetStream adresi (Faz 4.2). Boşsa kuyruk kapalı: ingest doğrudan store'a yazar. Örn: `nats://localhost:4222` |
| `-capture` | `true` | Hub'ın kendi paket yakalaması/collector'u. Çoklu replika ingest'te kapatılır |
| `-alerts` | `true` | Uyarı kural motoru. Çoklu replikada yalnızca bir replikada açık olmalı |
| `-poller` | `true` | SNMP cihaz poller'ı. Çoklu replikada yalnızca bir replikada açık olmalı |
| `-auth-password` | — | Arayüz şifresi. Boşsa kimlik doğrulama kapalı. `AUTH_PASSWORD` ortam değişkeni de geçerli |
| `-llm-base-url` | — | OpenAI-uyumlu AI servisi adresi. Örn: `http://localhost:11434/v1` (Ollama), `http://localhost:1234/v1` (LM Studio) |
| `-llm-api-key` | — | AI API anahtarı. Yerel modeller için gerekmez |
| `-llm-model` | — | Varsayılan model. UI'dan da seçilebilir |
| `-llm-max-tokens` | `0` | İstek başına token limiti (0 = dahili varsayılanlar: parça 1500, final 2500, tek seferde 3000) |
| `-llm-no-think` | `false` | Qwen3 serisi modellerde düşünme modunu kapatır (sistem mesajına `/no_think` ekler) |
| `-record-dir` | `captures` | PCAP kayıt dosyalarının yazılacağı dizin |
| `-record-max-mb` | `100` | PCAP dosya başına üst boyut; aşıldığında otomatik yeni dosyaya geçer (rotasyon) |
| `-geoip-dir` | `geoip` | MaxMind GeoLite2 `.mmdb` dosyalarının aranacağı dizin |
| `-ip-api-lookup` | `true` | MMDB yoksa ip-api.com batch servisiyle IP çözümleme (internet kullanır) |

## Ortam Değişkenleri

| Değişken | Karşılığı | Not |
|----------|----------|-----|
| `AUTH_PASSWORD` | `-auth-password` | |
| `LLM_BASE_URL` / `OPENAI_BASE_URL` | `-llm-base-url` | |
| `LLM_API_KEY` / `OPENAI_API_KEY` | `-llm-api-key` | |
| `LLM_MODEL` | `-llm-model` | Varsayılan: `gpt-4o-mini` |
| `LLM_MAX_TOKENS` | `-llm-max-tokens` | |
| `LLM_NO_THINK` | `-llm-no-think` | `1` veya `true` |

Bayraklar ortam değişkenlerinden önceliklidir.

## AI Kurulumu

### Ollama (yerel)

```bash
ollama pull qwen2.5:7b
sudo ./bazntms -llm-base-url http://localhost:11434/v1
```

Yerel adres görüldüğünde API anahtarı zorunluluğu otomatik kalkar. Kurulu
modeller `/api/ai/models` üzerinden arayüze listelenir.

### LM Studio

Uygulamada "Local Server" sekmesinden sunucuyu başlatın:

```bash
sudo ./bazntms -llm-base-url http://localhost:1234/v1
```

### llama.cpp server / vLLM / OpenRouter / OpenAI

```bash
# llama.cpp
sudo ./bazntms -llm-base-url http://localhost:8080/v1 -llm-model model-adi

# Bulut servisler
LLM_API_KEY=sk-... ./bazntms
```

### Reasoning modelleri (Qwen3, DeepSeek-R1 vb.)

Bu modeller final cevaptan önce uzun düşünme metni üretir; token limiti
düşünmede biterse boş yanıt döner. Sunucu `reasoning_content` alanını ve
`<think>...</think>` bloklarını otomatik destekler. Ek ayarlar:

```bash
-llm-no-think            # Qwen3: düşünmeyi kapat (çok daha hızlı)
-llm-max-tokens 4000     # düşünmeye alan bırak
```

### Parça parça gönderme (chunked)

Analiz verisi 4 bölüme ayrılır: (1) trafik özeti + protokoller, (2) en yoğun
hedefler, (3) en aktif süreçler, (4) DNS sorguları. `chunked: true` iken her
bölüm ayrı istekle gider ve modelden yalnızca kısa not alınır; son istekte
yalnızca notlar birleştirilerek final analiz üretilir. Ham veri modele hiçbir
zaman ikinci kez gönderilmez — küçük modellerde (3B–7B) context şişmez.

## GeoIP Kurulumu

### 1) MaxMind GeoLite2 (offline, önerilen)

1. [maxmind.com](https://www.maxmind.com/en/geolite2/signup) üzerinden ücretsiz hesap açın
2. GeoLite2 **Country** ve GeoLite2 **ASN** veri tabanlarını indirin
3. `.mmdb` dosyalarını `geoip/` dizinine koyun:

```
geoip/
├── GeoLite2-Country.mmdb
└── GeoLite2-ASN.mmdb
```

Sunucu başlarken otomatik algılanır (`-geoip-dir` ile değiştirilebilir).
Kullanım tamamen offline'dur.

### 2) ip-api.com (anahtarsız fallback)

MMDB dosyaları yoksa devreye girer:

- Bilinmeyen genel IP'ler 3 saniyede bir ≤100'lük gruplar hâlinde sorgulanır
- Sonuçlar bellek önbelleğine yazılır (100.000 girdiye kadar)
- `429` yanıtında 1 dakika geri çekilme
- **Gizlilik**: uzak IP'leri üçüncü taraf servise gönderir; `-ip-api-lookup=false` ile kapatın

Özel, loopback ve link-local adresler (192.168.x.x, 127.x, fe80:: vb.) hiçbir
zaman çözümlenmez.

## Veri Saklama

| Tablo | Yazım periyodu | İçerik |
|-------|---------------|--------|
| `samples` | saniye | bps in/out/local, pps, düşen paket, protokol dağılımı (JSON) |
| `endpoint_stats` | dakika | uç nokta bazlı transfer farkları (delta) |
| `connection_events` | dakika | aktif bağlantılar (süreç adı + PID) |
| `dns_queries` | dakika | domain bazlı sorgu/yanıt sayaçları |
| `alert_events` | olay anında | uyarı geçmişi |
| `alert_seen` | kalıcı | yeni süreç/hedef kurallarının "görüldü" işaretleri |
| `alert_config` | PUT ile | uyarı ayarları (JSON, tek satır) |
| `insights` | analizle | AI analiz sonuçları |

Saklama süresi: `-retention-hours` (varsayılan 168 saat = 7 gün). DB dosyası
`-db` ile taşınabilir; boyut kontrolü için `ls -la <db>*` (WAL dahil).

## RBAC, SSO ve Denetim (Faz 5)

### Roller

Tek-şifre modu (`-auth-password`) **admin** kimliği olarak çalışmaya devam
eder. Kalıcı kullanıcılar `-auth-password` ile ilk girişten sonra
`/api/v1/users` üzerinden açılır (bcrypt saklanır):

| Rol | Görüntüleme | Yakalama/kayıt | AI/rapor | Cihaz yönetimi | Agent silme | Kullanıcı/token/audit |
|---|---|---|---|---|---|---|
| `admin` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `netops` | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ |
| `analyst` | ✓ | ✗ | ✓ | ✗ | ✗ | ✗ |
| `viewer` | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |

`site` alanı dolu kullanıcılar yalnızca kendi sitelerinin agent'larını görür.

### SSO (OIDC)

```yaml
oidc:
  issuer: https://keycloak.kurum.local/realms/bazntms
  client_id: bazntms
  client_secret: gizli
  group_roles:
    bazntms-admin: admin
    bazntms-netops: netops
    bazntms-analyst: analyst
  default_role: viewer
```

Giriş: `GET /api/auth/oidc/login` (arayüz oturum açma sayfasından da
bağlanır). Grup/rol claim'i `groups` veya `roles` okunur.

### Entegrasyon API token'ları

`POST /api/v1/tokens {"name":"grafana","role":"analyst"}` → düz token
(`bnt_...`) **bir kez** döner; `Authorization: Bearer bnt_...` ile kullanılır.
`DELETE /api/v1/tokens/{id}` ile iptal edilir.

### Denetim kaydı (audit log)

Tüm kritik işlemler (giriş/başarısız giriş, kullanıcı/token/cihaz/agent
değişiklikleri, yakalama başlat/durdur, AI analizi, yetki reddi) append-only
`audit_events` tablosuna yazılır. Her kayıt önceki kaydın hash'ini taşır
(SHA-256 zinciri); `GET /api/v1/audit/verify` zinciri yeniden hesaplayarak
bütünlüğü kanıtlar.

## İleri Analiz (Faz 6)

### FortiGate REST API entegrasyonu (Faz 8)

Arayüzde cihaz eklerken **vendor: FortiGate** seçilir; alanlar:

| Alan | Açıklama |
|------|----------|
| `api_url` | `https://<cihaz>[:port]` — REST API adresi |
| `api_token` | System > Admin > REST API Admin token'ı — vault'ta şifreli saklanır, **read-only profil önerilir** |
| `api_verify_tls` | Self-signed sertifika kullanıyorsanız kapatın |
| `vdom` | boş/`root` → tek VDOM; `all` → tüm VDOM'lar taranır |

Toplanan veriler: sistem durumu (serial/firmware/uptime), CPU/RAM/disk,
oturum sayısı, arayüz sayaçları, IPsec/SSL VPN, SD-WAN health-check,
politika hit sayaçları. Uyarı eşikleri uyarı ayarlarında `forti` bölümünden
yönetilir (`vpn_down`, `sdwan_latency_ms`, `sdwan_loss_pct`, `max_sessions`).

İstekler arasında hız koruması (varsayılan 150 ms) uygulanır; yönetim CPU
bütçesini korumak için poll aralığını 60 sn altına düşürmeyin.

### Anomali tespiti (AI'sız)

Uyarı ayarlarından (`PUT /api/alerts` veya arayüz) yönetilir:

```json
"anomaly": {
  "enabled": true,
  "sensitivity": 3.0,
  "min_samples": 120,
  "window_min": 5
}
```

Son 7 günün saatlik (hour-of-day) baseline'ı ile güncel 5 dakikalık pencere
karşılaştırılır; z-skoru eşiği aşarsa "Anomali" uyarısı üretilir ve tüm
bildirim kanallarına dağıtılır. Eski config'lerde alan yoksa varsayılanla
açılır (`sensitivity: 0` geçersiz kabul edilir).

### Bildirim kanalları

`notifiers` bölümüne eklenen kurumsal kanallar:

| Alan | Açıklama |
|------|----------|
| `teams_url` | Microsoft Teams incoming webhook (MessageCard) |
| `webhook_v2_url` + `webhook_v2_secret` | HMAC-SHA256 imzalı webhook (`X-BazNTMS-Signature` başlığı) |
| `email_host`, `email_port` (0→587), `email_from`, `email_to[]`, `email_user`, `email_pass` | SMTP e-posta (STARTTLS otomatik) |

### Topoloji keşfi

- Cihazlarda: SNMP poller her turda **LLDP-MIB**, **CISCO-CDP-MIB** ve
  **IP-MIB (ARP)** tablolarını yürütür; desteklenmeyen cihazlarda sessizce
  atlanır
- Agentlarda: yerel ağlar (CIDR) telemetriyle hub'a taşınır
- Kenarlar 24 saat görünmezse temizlenir (`PruneTopology`)
- Harita: arayüz "Ağ Topolojisi" kartı

### Kurumsal rapor

`/api/report?type=enterprise&days=30` — SLA (agent uptime, cihaz sağlığı,
paket düşme oranı), kapasite (dönem büyümesi) ve banding (p50/p95/p99).

## 5651 Uyumlu Log İmzalama (Faz 9)

```bash
./bazntms-hub -compliance \
  -tsa-url https://tsa.saglayici.com/tsa \
  -compliance-key compliance.key \
  -worm-dir /var/worm/bazntms \
  -mask-pii -compliance-retention-days 730
```

| Bayrak | Açıklama |
|--------|----------|
| `-compliance` | Motoru açar: syslog/uyum kayıtları hash-zincirli `compliance_logs` tablosuna yazılır |
| `-tsa-url` | RFC 3161 nitelikli zaman damgası servisi (KamuSM, e-Tugra, TurkTrust vb.). Boşsa mühürleme TSA'sız yapılır (`tsa_status: none`) |
| `-compliance-key` | ed25519 manifest imza anahtarı (PEM; yoksa üretilir, `.pub` dosyası doğrulama için ayrılır) |
| `-worm-dir` | Günlük imzalı paket dizini (`YIL/AY/gün` yapısı; `O_EXCL` ile üzerine yazılmaz) |
| `-mask-pii` | Delil paketinde IP/MAC/kullanıcı maskeleme (A.5.34/A.8.11) |
| `-compliance-retention-days` | Ham log saklama (varsayılan 730 gün = 5651 2 yıl) |

**İmza modeli:** her kayıt prev_hash zinciri taşır → saatlik Merkle checkpoint'leri → günün saatlik köklerinden günlük kök → RFC 3161 TSA + ed25519 manifest imzası → WORM paketi.

**Delil paketi:** arayüzde tarih aralığı seçip indirin (veya `GET /api/v1/compliance/evidence?from=&to=&mask=`), sonra offline doğrulayın:

```bash
bazntmsctl verify -bundle bazntms-delil-20260801-20260901.json \
  -pubkey compliance.key.pub -out rapor.txt
```

**Uyarı:** `time_drift` (A.8.17) — checkpoint saatleri sistem saatinden ileri saptığında üretilir; NTP senkronizasyonunu doğrulayın. Uyumluluk beyanı için hukuki danışmanlık gereklidir; bu özellik teknik kanıt üretir.

## Ölçek Altyapısı (Faz 4)

### PostgreSQL / TimescaleDB

`-db postgres://...` verildiğinde store otomatik PostgreSQL moduna geçer:

- Şema açılışta migrasyonla kurulur; `postgres://` DSN'de driver pgx'tir
- TimescaleDB eklentisi kuruluysa (`CREATE EXTENSION timescaledb`) açılışta
  hypertable'lar, **samples_1m / samples_1h** continuous aggregate'ları ve
  retention politikaları otomatik ayarlanır: **ham 7 gün → 1 dk 90 gün →
  1 saat 2 yıl**
- Eklenti yoksa düz PostgreSQL çalışır; temizlik Prune ile yapılır
- Sorgu p95 hedefi için 1 dakikalık panel sorguları continuous aggregate
  üzerinden okunur (gerçek-zamanlı agregasyon: yeni veri anında görünür)

```bash
./bazntms-hub -db "postgres://bazntms:bazntms@localhost:5432/bazntms?sslmode=disable"
```

### NATS JetStream kuyruğu

`-nats` verildiğinde ingest → processor ayrışması devreye girer:

- `POST /api/v1/agent/telemetry` batch'i `BAZNTMS` akışına
  (`ingest.telemetry`) yayınlar ve hemen yanıt döner; yazımı `store-writer`
  adlı durable consumer yapar
- NetFlow ve syslog olayları da kuyruğa alınır (`ingest.flows`,
  `ingest.syslog`)
- Yazım hatasında mesaj 2 sn gecikmeli Nak edilir (replay); 10. denemede
  atılır. Akış diskte tutulur (24 saat MaxAge, 5M mesaj sınırı)
- Çoklu hub replikası aynı kuyruğu paylaşabilir

### Çoklu replika rolleri

Stateless ingest ölçeklemesi için hub yeniden başlatılabilir (SIGTERM →
graceful shutdown). Node-yerel bileşenler bayraklarla ayrılır:

```bash
# ingest replikası (N adet, LB arkası)
./bazntms-hub -capture=false -alerts=false -poller=false -nats nats://...

# controller replikası (1 adet: yakalama + uyarı + poller)
./bazntms-hub -capture -alerts -poller
```

### Yük testi

Bkz. `loadtest/README.md` — `bazntms-loadgen` sentetik filosu ve k6
senaryosu (hedef: ≥170 ist/sn sürekli ingest).

### Dağıtım

- Tek-node demo: `docker compose -f deploy/docker-compose.yml up --build`
- Kubernetes: `deploy/helm/bazntms` (Helm chart, değerler `values.yaml`)
