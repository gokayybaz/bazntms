# bazNTMS

[![CI](https://github.com/gokayybaz/bazntms/actions/workflows/ci.yml/badge.svg)](https://github.com/gokayybaz/bazntms/actions/workflows/ci.yml)
[![Release](https://github.com/gokayybaz/bazntms/actions/workflows/release.yml/badge.svg)](https://github.com/gokayybaz/bazntms/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**baz Network Traffic Monitoring System** — merkezi **hub** + uç **agent** + ağ
cihazı entegrasyonları üçlüsüne kurulu, kendi altyapınızda (self-hosted) çalışan
bir ağ trafiği izleme platformu. Canlı paket ölçümü, akış toplama, süreç bazlı
L7/DNS görünürlüğü ve 5651 uyumlu imzalı loglar — tek Go binary'sine gömülü
(Vite + React arayüz embed edilir, kurulum sürtünmesi yok). Tek makinede başlayan
bir kurulum **5.000 agent** ölçeğine aynı binary ile taşınır.

```
        UÇLAR (agent filosu)                bazntms-hub                     AĞ CİHAZLARI
  ┌───────────────────────────┐      ┌───────────────────────┐      ┌──────────────────────────┐
  │ agent — Linux / Windows / │      │  ingest · RBAC · SSO  │      │ SNMPv3 (LLDP/CDP/ARP)    │
  │ macOS / k8s DaemonSet     │─────▶│  uyarı · audit        │◀─────│ NetFlow v5/v9 · IPFIX     │
  │ eBPF / ETW / pcap atfı    │ mTLS │  anomali · AI analiz  │ poll │ sFlow v5 · Syslog        │
  │ L7 (SNI/Host) · DNS       │      │  5651 imza · raporlar │      │ FortiGate REST API       │
  └───────────────────────────┘      └──────────┬────────────┘      └──────────────────────────┘
                                                │
                              ┌─────────────────┴──────────────────┐
                     PostgreSQL + TimescaleDB              NATS JetStream
                     hypertable · cagg · retention         ingest → processor
                     (tek düğümde: SQLite dosyası)         (tek düğümde: doğrudan yazım)
```

## Yetenekler

### Görünürlük
- **Trafik ölçümü**: paket bazlı yön / protokol / port dağılımı; en yoğun uç
  noktalar GeoIP + ASN ile dünya haritasında
- **L7 uygulama görünürlüğü**: süreç bazlı TLS ClientHello SNI + HTTP Host — imza
  tabanlı DPI yok
- **DNS görünürlüğü**: süreç bazlı sorgu/yanıt takibi; stub-resolver (systemd-resolved
  127.0.0.53 / Docker 127.0.0.11) dahil
- **Topoloji**: LLDP/CDP/ARP keşfi + agent subnet bildirimi → client → hub → cihaz
  → router → internet zinciri canlı akışıyla birlikte
- **Süreç / konuşma detayı**: bir sürecin veya bir IP çiftinin tüm ağ etkinliği
  tek yanıtta (hedefler, canlı bağlantılar, DNS/L7, zaman çizelgesi)

### Toplama & entegrasyon
- **Agent filosu**: enrollment, toplu telemetri, offline disk kuyruğu — doğrulanmış
  5.000 agent. Agent↔hub **karşılıklı TLS (mTLS)**, sertifikalar kendini yeniler
- **Süreç atfı**: Linux **eBPF** (fentry CO-RE, kernel ≥ 5.8 + BTF), Windows **ETW**,
  macOS **pcap** — `AttrSource` arayüzü + `collect.method: auto` + fallback
- **Akış toplama**: NetFlow v5/v9, IPFIX ve sFlow v5 tek toplayıcıda; şablon
  önbelleği + örnekleme oranına göre ölçekleme
- **Ağ cihazları**: SNMPv3 arayüz/durum takibi, Syslog alıcısı, FortiGate REST API
  (VPN tünel, SD-WAN sağlık, politika hit trendi, oturum izleme)

### Analiz & operasyon
- **Anomali tespiti**: materyalize baseline (mevsimsel + EWMA), filo/saha/agent ×
  bps/dns_qps/proc_bps
- **Uyarı yaşam döngüsü**: severity/state/dedup/gruplama, bakım pencereleri,
  otomatik + koşullu çözülme, korelasyon
- **Olay motoru + tehdit istihbaratı**: IOC kara listesiyle L7/DNS eşleştirme,
  incident üretimi, sağlık skoru
- **SIEM / ITSM connector**: olaylar CEF / LEEF / JSON → Splunk HEC, QRadar,
  ArcSight (syslog/HTTP); Jira & ServiceNow bilet açma
- **Raporlar**: zamanlı + PDF + SLA raporları (`internal/scheduler`, lider-kapılı);
  bildirim kanalları Teams, Slack, SMTP, imzalı webhook
- **AI analiz** (opt-in `-ai`): çoklu sağlayıcı (OpenAI-uyumlu + Anthropic),
  `/ai` sohbet sekmesi (SSE), gecelik analiz, olay-tetikli triyaj, egress kilidi

### Güvenlik & uyumluluk
- **Erişim**: rol tabanlı (admin / netops / analyst / viewer + site scope),
  OIDC SSO, entegrasyon token'ları, hash-zincirli append-only denetim kaydı
- **5651 log imzalama** (`-compliance`): hash-zincir + saatlik Merkle checkpoint +
  günlük RFC 3161 nitelikli zaman damgası + ed25519 manifest imzası; WORM depoda 2 yıl
- **Delil paketi**: tarih aralıklı çıkarım, PII maskeleme, `bazntmsctl verify` ile
  çevrimdışı doğrulama — teslim alan tarafın bazNTMS kurmasına gerek yok
- **ISO 27001 ISMS**: Annex A kontrol haritası, risk defteri, SoA, iç denetim
  kayıtları, zaman sapması alarmı (A.8.17)

### Ölçek & dağıtım
- **Depo seçimi tek bayrakla**: `-db` bir dosya yolu → SQLite, `postgres://` DSN →
  PostgreSQL/TimescaleDB (pgx). Uygulama kodu ve arayüz aynı kalır
- **NATS JetStream**: ingest → processor ayrışması, replay/kayıp toleransı, çoklu
  replika ingest; lider seçimi (`store.Leader` = pg advisory lock)
- **Kubernetes**: yayınlanmış Helm chart (`oci://ghcr.io/gokayybaz/charts/bazntms`)
  + çok-mimari imajlar (`ghcr.io/gokayybaz/bazntms-{hub,agent}`, amd64 + arm64,
  digest-sabit temel imajlar, SLSA provenance)
- **k8s olmadan ölçek**: `deploy/docker-compose.scale.yml` — N × ingest + kontrolcü
  + nginx LB + JetStream
- **Paketler**: deb / rpm (nfpm + systemd), Windows MSI (WiX, Npcap sessiz kurulum),
  macOS pkg (launchd) — release CI otomatik üretir
- **Otomatik agent güncellemesi** (varsayılan açık): stable/beta kanalları,
  SHA-256 (+ opsiyonel ed25519) doğrulamalı, atomik binary değişimi
- **Doğrulanmış kapasite**: 5.000 agent @ 30 sn (p95 5 ms) · ≥50.000 flow/sn
  (kayıpsız) · 1.000 cihaz / 60 sn poll — bkz. [`docs/CAPACITY.md`](docs/CAPACITY.md)

## Hızlı Başlangıç

Üç kurulum yolu — üçü birbirinin alternatifi, hepsi aynı binary'yi kullanır.

```bash
# A) Tek-node demo
docker compose -f deploy/docker-compose.yml up --build
# → http://localhost:8080 · şifre: demo123

# B) Elle derle ve agent bağla
make                       # frontend + hub + agent + ctl
./bazntmsctl setup         # interaktif sihirbaz → bazntms-hub.yml
./bazntms-hub -config bazntms-hub.yml
./bazntms-agent -hub-url https://hub.example.com -enroll-token <hub-loglarındaki-token>

# C) Ölçek mimarisi (k8s olmadan)
docker compose -f deploy/docker-compose.scale.yml up --build
# 2 × ingest + kontrolcü + nginx LB + JetStream · dashboard :8080 · agent API :8081
```

Kubernetes:

```bash
helm install bazntms oci://ghcr.io/gokayybaz/charts/bazntms --version 1.3.0 \
  --set config.database.path="postgres://..." \
  --set config.nats.url="nats://..." \
  --set auth.existingSecret=bazntms-auth
```

### Gereksinimler

| Platform | Süreç atfı + L7 (agent) | Çalıştırma | Derleme |
|----------|------------------------|-----------|---------|
| Linux    | **eBPF** (kernel ≥ 5.8 + BTF) + L7 için dar kapsamlı yardımcı pcap | `CAP_NET_RAW` + `CAP_NET_ADMIN` (paket kurulumları verir) | `libpcap-dev` |
| Windows  | **pcap** (MSI Npcap'i otomatik kurar) — L7 dahil | yükseltilmiş süreç (SYSTEM servis) | (gerek yok) |
| macOS    | pcap — L7 dahil | `sudo` (BPF erişimi) | Xcode CLT |

> **v1.3.0'dan beri derin toplama (süreç trafiği + DNS + L7/SNI) tüm kurulum
> yollarında varsayılan AÇIK.** Linux'ta eBPF çekirdek düzeyinde sayım yapar,
> L7 (SNI/Host) için dar filtreli bir yardımcı pcap handle açılır. Windows'ta
> MSI kurulumu [Npcap](https://npcap.com)'i sessizce kurar ve agent `pcap`
> yöntemiyle gelir (Npcap yoksa ETW'ye düşer — L7 hariç her şey akar).
> Kapatmak: `collect.method: off`. Hub politikası `-agent-pcap` de varsayılan açık.

- Go **1.26+**
- Node.js **18+** (yalnızca frontend derlemek için)

> Agent yetkisiz de temel telemetri (bağlantı listesi, arayüzler, geçmiş, uyarılar)
> gönderir; yalnızca canlı paket yakalama / PCAP kayıt için yükseltilmiş yetki gerekir.

## Dokümantasyon

Docusaurus sitesi her push'ta GitHub Pages'e otomatik yayınlanır
(`docs-site/**` veya `docs/**` değişince):
**https://gokayybaz.github.io/bazntms/**

| Belge | İçerik |
|-------|--------|
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Tüm komut satırı bayrakları, ortam değişkenleri, GeoIP ve AI kurulumları |
| [docs/API.md](docs/API.md) | REST + WebSocket uçlarının tam referansı ve örnekleri |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | İç tasarım: yakalama motoru, collector, uyarı/anomali motoru, veri şeması, süreç atfı sağlayıcıları |
| [docs/ALERTING.md](docs/ALERTING.md) | Uyarı yaşam döngüsü, severity/dedup/korelasyon, bakım pencereleri, notify rotaları |
| [docs/ANALYTICS.md](docs/ANALYTICS.md) | Anomali baseline, sağlık skoru, konuşma/iface analitiği |
| [docs/THREAT-MODEL.md](docs/THREAT-MODEL.md) | Varlıklar, güven sınırları, sınır başına tehditler ve karşı önlemler |
| [docs/DEPLOYMENT-MODEL.md](docs/DEPLOYMENT-MODEL.md) | Dağıtım senaryosu kararı (tek-kurum / MSP çoklu-saha) ve izolasyon modeli |
| [docs/CAPACITY.md](docs/CAPACITY.md) | Kapasite doğrulaması: yük profilleri, ölçüm sonuçları, tuning bayrakları |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | İzin hataları, atıf motoru (eBPF/ETW/pcap), Npcap, AI sorunları |
| [docs/UPGRADE-RUNBOOK.md](docs/UPGRADE-RUNBOOK.md) | Sürüm atlama: hub / agent / DB / K8s güncelleme adımları |
| [docs/DR-RUNBOOK.md](docs/DR-RUNBOOK.md) | Felaket kurtarma: yedek, geri yükleme, RTO/RPO |
| [docs/RELEASE-RUNBOOK.md](docs/RELEASE-RUNBOOK.md) | Bakımcı: sürüm etiketi kesme + pipeline + doğrulama |
| [CHANGELOG.md](CHANGELOG.md) | Sürüm başına başlıklar + kırıcı / yükseltme notları |

## Yapılandırma (özet)

Hub bayrakları `bazntms-hub.yml` config dosyasından da verilebilir (bayraklar
üstünlükte). Tam liste: [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

| Bayrak | Varsayılan | Açıklama |
|--------|-----------|----------|
| `-config` | — | YAML config dosyası |
| `-port` | `8080` | HTTP portu |
| `-db` | `bazntms.db` | SQLite dosyası **veya** `postgres://` DSN |
| `-nats` | — | NATS JetStream adresi (boş = doğrudan yazım) |
| `-auth-password` | — | Panel şifresi (boş = auth kapalı; RBAC ayrı) |
| `-tls` | `false` | HTTPS + agent mTLS (hub kendi CA'sını üretir) |
| `-multi-site` | `false` | MSP çoklu-saha modu (sert yetki sınırı) |
| `-agent-pcap` | `true` | Filo genelinde derin toplama politikası |
| `-ai` | `false` | AI analiz sekmesi + uçları |
| `-compliance` | `false` | 5651 log imzalama motoru |
| `-flow-port` / `-syslog-port` | — | NetFlow/IPFIX/sFlow · Syslog dinleme portları |
| `-ioc-file` | — | Tehdit istihbaratı domain kara listesi |
| `-public-url` | — | Panelin dış adresi (WS origin + OIDC redirect) |

## Geliştirme

```bash
make dev-backend             # yalnızca API (-dev), frontend ayrı çalışır
cd frontend && npm run dev    # Vite dev server (proxy ayarlı)

make test                    # go vet + gofmt -l + tsc -b
go test ./...                # Go testleri (make test bunu ÇALIŞTIRMAZ)
```

CI: ayrı `frontend` job (lint + test + build) + Linux/macOS/Windows'ta
`go vet && go test` + `govulncheck`. Türkçe kod yorumları ve commit mesajları
kullanılır — bkz. [CLAUDE.md](CLAUDE.md).

## Güvenlik Notları

- **Agent ↔ hub mTLS**: `-tls` ile hub kendi CA'sını üretir (ECDSA P-256),
  enrollment sırasında her agent'a bir istemci sertifikası (`CN=bazntms-agent-<id>`,
  90 gün) verir; ömrünün yarısı geçince kendini yeniler. Tarayıcı aynı porttan
  sertifikasız bağlanabilir (`VerifyClientCertIfGiven`). Agent hub'ı `-hub-ca` ile
  önceden pinler; verilmezse ilk bağlantıda TOFU + otomatik pin.
  ```
  hub:   bazntms-hub -tls -tls-hosts hub.example.com
  agent: bazntms-agent -hub-url https://hub.example.com:8080 -hub-ca ca.crt
  ```
- Panel/tarayıcı trafiği için reverse proxy de kullanılabilir (`Caddy { reverse_proxy
  127.0.0.1:8080 }`). L7 reverse proxy agent mTLS'ini sonlandırır; mTLS için
  agent'lar hub'a doğrudan ya da L4/TCP-passthrough LB ile bağlanmalı.
- **Ölçek + mTLS**: `docker compose -f deploy/docker-compose.scale.yml -f
  deploy/docker-compose.scale-mtls.yml up -d` — hub-ingest replikaları paylaşılan
  CA volume'ü kullanır, nginx `:8443`'te L4 passthrough yapar.
- Kimlik kasası master anahtarı diske yazılmaz seçeneği: `-vault-key-source=env`
  (`BAZNTMS_VAULT_MASTER_KEY` — KMS / secret manager enjeksiyonu).
- Panel oturumları bellekte (tek replika) veya `sessions` tablosunda
  (`-session-store=db`, çoklu controller). `-ip-api-lookup=false` üçüncü taraf
  IP çözümlemeyi kapatır.

## Lisans

MIT
