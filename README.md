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

### Ölçek altyapısı (Faz 4)
- **Depo seçimi**: `-db` dosya yolu → SQLite, `postgres://` DSN → PostgreSQL/TimescaleDB (pgx)
- **NATS JetStream kuyruğu**: ingest → processor ayrışması, replay/kayıp toleransı, çoklu replika ingest
- **Kubernetes dağıtımı**: Helm chart (`deploy/helm/bazntms`), tek-node docker-compose demo (`deploy/`)
- **Yük testi**: `bazntms-loadgen` sentetik agent filosu + k6 senaryosu (`loadtest/`)

### Dağıtım ve operasyon (Faz 7)
- **Yönetim CLI**: `bazntmsctl setup` (kurulum sihirbazı), `update keygen/sign` (ed25519 imza kanalı)
- **Paketler**: deb/rpm (nfpm + systemd), Windows MSI (WiX), macOS pkg (launchd) — release CI otomatik üretir
- **Otomatik agent güncellemesi**: stable/beta kanalları, SHA-256 + ed25519 imza doğrulamalı, atomik binary değişimi
- **Dokümantasyon**: `docs/UPGRADE-RUNBOOK.md`, `docs/DR-RUNBOOK.md`, Docusaurus iskeleti (`docs-site/`)

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
# 1) Frontend'i derle
cd frontend && npm install && npm run build && cd ..

# 2) Backend'i derle (frontend embed edilir)
go build -o bazntms ./cmd/bazntms-hub

# 3) Yönetici yetkisiyle çalıştır
sudo ./bazntms -auth-password gizliSifre123 -llm-base-url http://localhost:11434/v1
```

Tarayıcıdan `http://localhost:8080` → giriş yap → arayüz seç → **Yakalamayı Başlat**.

### Gereksinimler

| Platform | Çalıştırma | Derleme |
|----------|-----------|---------|
| macOS    | `sudo` (BPF erişimi) | Xcode CLT |
| Linux    | `sudo` veya `setcap cap_net_raw+ep` | `libpcap-dev` |
| Windows  | [Npcap](https://npcap.com) + yönetici | Npcap SDK + mingw-w64 |

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
| [docs/enterprise-plan.html](docs/enterprise-plan.html) | 🗺️ Enterprise yol haritası: hub + agent + cihaz entegrasyonları, faz planı |

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

- HTTPS sunulmaz; internete açacaksanız reverse proxy arkasına alın:
  ```caddy
  # Caddy örneği (otomatik TLS)
  ag.example.com {
      reverse_proxy 127.0.0.1:8080
  }
  ```
- Oturumlar bellekte tutulur; sunucu yeniden başlayınca yeniden giriş gerekir
- ip-api.com modu uzak IP'leri üçüncü taraf servise gönderir (`-ip-api-lookup=false` ile kapatın)

## Lisans

MIT
