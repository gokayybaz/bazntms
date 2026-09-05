# bazNTMS

[![CI](https://github.com/gokayybaz/bazntms/actions/workflows/ci.yml/badge.svg)](https://github.com/gokayybaz/bazntms/actions/workflows/ci.yml)
[![Release](https://github.com/gokayybaz/bazntms/actions/workflows/release.yml/badge.svg)](https://github.com/gokayybaz/bazntms/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**baz Network Traffic Monitoring System** — Go ile yazılmış, **çok platformlu** ağ trafiği monitörü. Bilgisayarınızın network
trafiğini gerçek zamanlı olarak **yakalar, analiz eder, kaydeder ve raporlar**.
Vite + React ile tasarlanmış web arayüzü Go binary'sinin içine embed edilir —
tek dosya, kurulum gerektirmez.

```
┌─────────────────────────────────────────────────────────────┐
│                      bazNTMS                         │
│                                                             │
│  gopacket/libpcap ──► Yakalama Motoru ──► Canlı WS Akışı    │
│        │                     │                              │
│        │               SQLite (collector)                   │
│        │              /       |        \                    │
│        │        Uyarı Motoru  AI Analizi   Rapor (PDF/HTML) │
│        │              │              │           │          │
│        └── PCAP Kayıt  Webhook      Ollama/LM     İndirme   │
│                        Masaüstü      Studio                  │
└─────────────────────────────────────────────────────────────┘
```

## Özellikler

### Trafik izleme
- **Canlı paket yakalama** (gopacket/libpcap): TCP, UDP, ICMP, ICMPv6 ayrımı
- **Yön tespiti**: indirme / gönderme / yerel trafiği ayrı ölçüm
- **Canlı verim grafiği**: saniyelik örnekleme, son 2 dakika (WebSocket)
- **En yoğun uç noktalar**: otomatik ters-DNS + "bu cihaz" rozeti
- **DNS görünürlüğü**: UDP/53 sorguları, domain bazlı sayaçlar, çözümlenen IP'ler
- **Aktif bağlantılar**: süreç adı + PID, filtre ve TCP/UDP seçimi
- **Protokol dağılımı ve en yoğun portlar**
- **GeoIP + ASN**: ülke bayrağı ve ağ sahibi (MaxMind offline veya ip-api.com)

### Kayıt ve veri
- **SQLite (dev modu)** veya **PostgreSQL/TimescaleDB (ölçek modu)**: saniyelik örnekler, dakikalık uç nokta/DNS/bağlantı kayıtları, otomatik temizlik; TimescaleDB'de hypertable + continuous aggregate + retention (ham 7g → 1dk 90g → 1sa 2y)
- **PCAP kayıt**: yakalama anında ham paketleri Wireshark uyumlu `.pcap` dosyasına yazma, boyut rotasyonu, tarayıcıdan indirme

### Ölçek altyapısı
- **Depo seçimi**: `-db` dosya yolu → SQLite, `postgres://` DSN → PostgreSQL/TimescaleDB (pgx)
- **NATS JetStream kuyruğu**: ingest → processor ayrışması, replay/kayıp toleransı, çoklu replika ingest
- **Kubernetes dağıtımı**: Helm chart (`deploy/helm/bazntms`), tek-node docker-compose demo (`deploy/docker-compose.yml`), k8s olmadan ölçek mimarisi (N × ingest + kontrolcü + LB: `deploy/docker-compose.scale.yml`)
- **Yük testi**: `bazntms-loadgen` sentetik agent filosu + k6 senaryosu (`loadtest/`)

### Dağıtım ve operasyon
- **Yönetim CLI**: `bazntmsctl setup` (kurulum sihirbazı), `update keygen/sign` (ed25519 imza kanalı)
- **Paketler**: deb/rpm (nfpm + systemd), Windows MSI (WiX), macOS pkg (launchd) — release CI otomatik üretir
- **Otomatik agent güncellemesi**: stable/beta kanalları, SHA-256 + ed25519 imza doğrulamalı, atomik binary değişimi
- **Dokümantasyon**: `docs/UPGRADE-RUNBOOK.md`, `docs/DR-RUNBOOK.md`, Docusaurus sitesi (`docs-site/`)

### Uyumluluk
- **5651 log imzalama**: hash-zincirli loglar + saatlik Merkle checkpoint + günlük RFC 3161 nitelikli zaman damgası + ed25519 manifest imzası; WORM depoda 2 yıl saklama
- **Delil paketi**: tarih aralıklı çıkarım, PII maskeleme, `bazntmsctl verify` ile offline doğrulama — adli süreçler için
- **ISO 27001**: Annex A kontrol haritası, imzalı log inceleme/erişim inceleme tutanakları, zaman sapması alarmı (A.8.17)

## Dokümantasyon Sitesi

Docusaurus sitesi her push'ta GitHub Pages'e otomatik yayınlanır
(`docs-site/**` veya `docs/**` değişince tetiklenir):

**https://gokayybaz.github.io/bazntms/**

- `docs/reference/` içeriği her build'de repo kökündeki `docs/*.md`
  dosyalarından otomatik senkronize edilir
- Yerel geliştirme: `cd docs-site && npm install && npm run dev`
- İlk kurulum: repo **Settings → Pages → Source = GitHub Actions** seçilmeli

### Analiz ve otomasyon
- **AI analizi**: OpenAI-uyumlu her servis (Ollama, LM Studio, llama.cpp, OpenAI...)
  - **Parça parça gönderme**: veri 4 parçaya bölünüp ayrı isteklerle gider — küçük modellerde context şişmez
  - Reasoning model desteği (`reasoning_content`, `<think>` temizliği, `/no_think`)
  - Kurulu modellerin otomatik listelenmesi ve analiz başına model seçimi
- **Uyarı sistemi**: bant genişliği zirvesi, şüpheli port, yeni süreç, yeni hedef; soğuma süresi; masaüstü + Telegram/Discord/Slack/generic webhook
- **Karşılaştırmalı grafikler**: son 7 gün çubuk grafiği, bugün vs dün saatlik overlay, % değişim
- **Rapor üretimi**: tek tıkla kapsamlı **HTML veya PDF** (gömülü fontla Türkçe çıktı)

### Güvenlik
- **Şifreli oturum**: `-auth-password` ile; cookie + Bearer token, IP bazlı deneme sınırı

## Hızlı Başlangıç

```bash
# 1) Her şeyi derle: frontend + hub + agent + ctl (tek komut)
make

# 2) Kurulum sihirbazı ile yapılandırma üret (isteğe bağlı)
./bazntmsctl setup            # interaktif: port, şifre, depo, NATS → bazntms-hub.yml

# 3) Yönetici yetkisiyle çalıştır
sudo ./bazntms -auth-password gizliSifre123 -llm-base-url http://localhost:11434/v1
# veya: sudo ./bazntms-hub -config bazntms-hub.yml
```

Tarayıcıdan `http://localhost:8080` → giriş yap → arayüz seç → **Yakalamayı Başlat**.

### Gereksinimler

| Platform | Çalıştırma | Derleme |
|----------|-----------|---------|
| macOS    | `sudo` (BPF erişimi) | Xcode CLT |
| Linux    | `sudo` veya `setcap cap_net_raw+ep` | `libpcap-dev` |
| Windows  | [Npcap](https://npcap.com) + yönetici | (gerek yok — `gopacket/pcap` Windows'ta cgo kullanmaz) |

- Go **1.22+** (yeni `http.ServeMux` kalıpları için)
- Node.js **18+** (yalnızca frontend derlemek için)

> Yetki olmadan da uygulama çalışır: bağlantı listesi, arayüzler, geçmiş ve uyarılar
> çalışır; yalnızca canlı paket yakalama/PCAP kayıt için yönetici yetkisi gerekir.

## Dokümantasyon

| Belge | İçerik |
|-------|--------|
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Tüm komut satırı bayrakları, ortam değişkenleri, GeoIP ve LLM kurulumları |
| [docs/API.md](docs/API.md) | REST + WebSocket uçlarının tam referansı ve örnekleri |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | İç tasarım: yakalama döngüsü, collector, uyarı motoru, veri şeması |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | İzin hataları, Npcap, AI sorunları, sık karşılaşılan durumlar |
| [docs/enterprise-plan.html](docs/enterprise-plan.html) | 🗺️ Enterprise yol haritası: hub + agent + cihaz entegrasyonları, ölçek hedefleri |

## Yapılandırma (özet)

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `-port` | `8080` | HTTP portu |
| `-db` | `bazntms.db` | SQLite dosyası |
| `-retention-hours` | `168` | Veri saklama süresi |
| `-auth-password` | — | Arayüz şifresi (boş = auth kapalı) |
| `-llm-base-url` | — | OpenAI-uyumlu AI adresi |
| `-record-dir` / `-record-max-mb` | `captures` / `100` | PCAP kayıt ayarları |
| `-geoip-dir` / `-ip-api-lookup` | `geoip` / `true` | GeoIP kaynakları |

Tam liste ve ortam değişkenleri: [docs/CONFIGURATION.md](docs/CONFIGURATION.md)

## Geliştirme

```bash
make dev-backend     # yalnızca API (-dev), frontend ayrı çalışır
cd frontend && npm run dev   # Vite dev server (proxy ayarlı)

make test            # go vet + gofmt + tsc
go test ./...        # Go testleri
```

## Güvenlik Notları

- **Agent ↔ hub karşılıklı TLS (mTLS)**: `-tls` ile hub kendi CA'sını üretir,
  ECDSA P-256 sunucu sertifikası imzalar ve enrollment sırasında her agent'a
  bir istemci sertifikası (`CN=bazntms-agent-<id>`, 90 gün) verir. Agent
  sonraki bağlantılarda bu sertifikayla kimliklenir (Bearer token'a eşdeğer);
  ömrünün yarısı geçince kendini yeniler. Tarayıcı aynı porttan sertifikasız
  bağlanabilir (`VerifyClientCertIfGiven`). Agent hub'ı `-hub-ca <dosya>` ile
  önceden pinler; verilmezse ilk bağlantıda TOFU + otomatik pin.
  ```
  hub:   bazntms-hub -tls -tls-hosts hub.example.com
  agent: bazntms-agent -hub-url https://hub.example.com:8080 -hub-ca ca.crt
  ```
- Panel/tarayıcı trafiği için (veya mTLS istemiyorsanız) reverse proxy de
  kullanılabilir:
  ```caddy
  ag.example.com { reverse_proxy 127.0.0.1:8080 }   # Caddy — otomatik TLS
  ```
  (Not: L7 reverse proxy agent mTLS'ini sonlandırır; mTLS için agent'lar hub'a
  doğrudan ya da L4/TCP-passthrough LB ile bağlanmalı.)
- **Ölçek + mTLS**: `docker compose -f deploy/docker-compose.scale.yml -f
  deploy/docker-compose.scale-mtls.yml up -d` — hub-ingest replikaları
  paylaşılan bir CA volume'ü kullanır (bir replikanın verdiği sertifika
  diğerlerince kabul edilir), nginx `:8443`'te L4 passthrough yapar. Agent
  API: `https://localhost:8443`.
- Oturumlar bellekte tutulur; sunucu yeniden başlayınca yeniden giriş gerekir
- ip-api.com modu uzak IP'leri üçüncü taraf servise gönderir (`-ip-api-lookup=false` ile kapatın)

## Lisans

MIT
