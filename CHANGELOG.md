# Değişiklik Günlüğü

Bu projedeki dikkate değer değişiklikler burada tutulur. Biçim
[Keep a Changelog](https://keepachangelog.com/tr/1.1.0/) temellidir; sürümleme
[SemVer](https://semver.org/lang/tr/) (v1.0.0'a kadar minor = özellik, patch =
düzeltme). Her GitHub sürümü ayrıca `--generate-notes` ile üretilmiş tam commit
listesi taşır — bu dosya **operatörün önemsediği** başlıkları ve **kırıcı /
yükseltme** notlarını özetler.

Kanallar: `agents.uplink_device_id` gibi şema değişiklikleri hub açılışında
otomatik migrasyonla uygulanır (`internal/store/migrations/`), geri alma yoktur
— yükseltmeden önce yedek alın (bkz. [`docs/UPGRADE-RUNBOOK.md`](docs/UPGRADE-RUNBOOK.md)).

## [Yayımlanmamış]

## [0.4.0] — 2026-09-07

Faz 20 — süreç atfı çekirdek düzeyine taşındı. Linux'ta **eBPF**, Windows'ta
**ETW** ile süreç trafiği + DNS; **Windows'ta artık Npcap gerekmez**.

> Her iki arka uç da gerçek makinede canlı doğrulandı. Windows ETW ilk canlı
> testte üç hatayla çıkmadı — hepsi bu sürümde düzeltildi:
> `TRACE_LOGFILE_HEADER` struct'ında eksik 16 baytlık union alanı (ETW
> callback'i hiç çağrılmıyordu), Kernel-Network keyword maskesi (0x10/0x20),
> TCP recv uzak-uç ayrıştırması + multicast/loopback eleme.

### Eklendi
- **eBPF atıf motoru (Linux).** Kernel ≥ 5.8 + BTF olan makinelerde `collect.method`
  varsayılanı (`auto`) artık paket yakalamak yerine çekirdeğin soket katmanına
  CO-RE fentry ile bağlanır: her paket yerine her send/recv işleminde çalışır →
  belirgin şekilde ucuz, byte sayımı NIC offload'ından etkilenmez, **`CAP_NET_RAW`
  gerekmez** (`CAP_BPF`/`CAP_PERFMON` yeter). DNS `systemd-resolved` (127.0.0.53)
  ve Docker gömülü DNS (127.0.0.11) için native görünür.
- **ETW atıf motoru (Windows).** Yükseltilmiş (SYSTEM servis / yönetici) süreçte
  `Microsoft-Windows-Kernel-Network` + `Microsoft-Windows-DNS-Client` sağlayıcıları
  ile süreç trafiği + DNS — **Npcap kurulmadan**. Saf Go (yeni bağımlılık yok).
- **`collect.method`** agent config alanı + `-collect-method` bayrağı:
  `auto` | `ebpf` | `pcap` | `etw` | `off`. `auto` platforma göre en iyi arka
  ucu seçer ve kurulamayanı pcap'e düşürür; her düşüş loglanır.
- **`attr_method`** — agent aktif atıf arka ucunu her telemetri batch'inde
  bildirir; `/api/v1/agents` yanıtında ve agent detay sayfasındaki "Atıf"
  rozetinde görünür.

### Değiştirildi
- **Windows agent Npcap'siz çalışır.** `auto`/`etw` modunda süreç trafiği + DNS
  ETW ile toplanır. **Npcap yalnız şunlar için gerekir:** L7 (SNI/Host) paneli,
  ham `-record`, tek-makine hub yakalaması — bunlar `-collect-method=pcap` ister.
- L7 (SNI/Host): pcap arka ucunda ana handle'dan; eBPF modunda dar filtreli bir
  yardımcı pcap handle'dan (`CAP_NET_RAW` ister, yoksa L7 boş kalır — sayım + DNS
  aksamaz); **ETW modunda yoktur**.

### Şema
- `0007_attr_method` — `agents.attr_method TEXT` (agent'ın bildirdiği atıf arka ucu).

### Kırıcı / yükseltme
- **Windows'ta L7 (SNI/Host) paneli ve `-record`**, agent `auto`/`etw` modundaysa
  ve Npcap kurulu değilse **boş kalır**. Eskisi gibi çalışması için agent'a
  `collect.method: pcap` verin (Npcap gerekir) — ya da yalnız süreç trafiği +
  DNS yeterliyse hiçbir şey yapmayın (ETW ile gelir).

### Karar kaydı
- [`docs/decisions/0007-attr-backends.md`](docs/decisions/0007-attr-backends.md) —
  arka uç arayüzü, fentry/CO-RE, ETW'nin elle yazılması (GPL/cgo kütüphaneler
  reddedildi), L7'nin pcap-gated kalması.

## [0.3.3] — 2026-09-07

Dağıtım olgunluğu — Faz 19. Konteyner imajları, imzalı chart, provenance,
opt-in imzalı update kanalı. Ayrıca Canlı Akış uplink gruplama + `collect.pcap`
görünürlük düzeltmesi.

### Eklendi
- **Canlı Akış — uplink gruplama.** Yönetici `/akis` şemasında `F3` "Düzenle"
  ile agent'ları bir erişim katmanı cihazına (switch / AP) atar; şema
  agent'ları bu cihaza göre gruplar ve okları ara katman üzerinden çizer.
  Yeni uçlar: `PUT /api/v1/agents/{id}/uplink`, `PUT /api/v1/devices/{id}/uplink`.
  `POST /api/v1/devices` artık `host` olmadan "yönetilmeyen" topoloji düğümüne
  izin verir (SNMP/REST yok → poll edilmez).
- **Canlı görsellerde tam ekran.** `/akis` ve `/cografi` sayfalarında `F4` (veya
  sağ üst düğme) diyagramı kabuğun üstüne tam ekran açar (`Esc` kapatır);
  kalabalık filoda diyagram çok sütuna paketlenir.
- `lib/usePolledJson` — kartlar için ilk veri gelene kadar hızlanan yeniden
  denemeli JSON yoklama hook'u.
- Çok alanlı dialog'lara `password` alan tipi.

### Değiştirildi
- **Paketlenmiş agent kurulumları (deb / rpm / MSI / .pkg) artık
  `collect.pcap: true` yazar.** Süreç trafiği, DNS ve L7 (uygulama görünürlüğü)
  panelleri bu ayar (ve hub tarafında `-agent-pcap` politikası) açıkken dolar —
  önceden paketlenmiş kurulumlarda varsayılan kapalıydı ve üç panel sessizce
  boş kalıyordu.
- `deploy/config/bazntms-agent.yml.example`: `hub.url` / `hub.token` artık boş
  (placeholder `hub.example.com` yerine) — eksik yapılandırma sessizce yanlış
  bir adrese bağlanmak yerine açık hata versin.
- Agent kurulum sihirbazı enroll komutundaki hub adresi için `-public-url`
  değerini kullanır (panel bir tünel/ters-proxy arkasından `localhost`'ta
  açılmış olabilir); hub adresi `localhost` ise sihirbaz uyarı gösterir.

### Düzeltildi
- Süreç / L7 / DNS kartları geçici hub hatasında (yeniden başlarken 401/502)
  "Yükleniyor…" ekranında takılıp kalıyordu.
- Sayfa geçişinde içerik önceki kaydırma konumunu koruyup "ortadan başlıyordu".
- `Prune()` artık agent'ı silinmiş ama geride kalmış filo satırlarını
  (`agent_conn_latest` / `agent_iface_samples` / `process_traffic` /
  `l7_endpoints` / `agent_dns`) da süpürür.

### Dağıtım / tedarik zinciri
- **Konteyner imajları:** `ghcr.io/gokayybaz/bazntms-{hub,agent}` — çok-mimari
  (amd64 + arm64), digest-sabit temel imajlar, OCI etiketleri, hub
  `HEALTHCHECK`. Yayın öncesi Trivy imaj taraması (CRITICAL bloklar) + hub
  duman testi push'u kapıya alır.
- **Helm chart** artık `oci://ghcr.io/gokayybaz/charts/bazntms` olarak yayınlanır;
  CI'da `helm lint` + `helm template | kubeconform`. `hub` `/healthz` yanıtı
  `{version, protocol_version}` taşır (yükseltme doğrulaması).
- **SLSA build provenance** — her binary + konteyner imajı için attestation
  (`gh attestation verify`). Release notlarına doğrulama bölümü eklenir
  (`cosign verify-blob`, `gh attestation verify`, `helm pull`).
- **Opt-in imzalı agent auto-update kanalı:** `UPDATE_SIGNING_SEED` repo
  secret'ı tanımlıysa release manifest'i ed25519 ile imzalanır; hub
  `GitHubSyncer` imzaları geçirir ve indirdiği binary'leri imzalı manifest'e
  karşı doğrular. `bazntmsctl update verify`. Varsayılan davranış değişmez.
- `CHANGELOG.md` (bu dosya) + `docs/RELEASE-RUNBOOK.md`; `release.yml`
  preflight'ı `Chart.yaml`/CHANGELOG sürümünü etiketle eşleşmeye zorlar.

### Şema
- `0006_uplink` — `agents.uplink_device_id` + `devices.uplink_device_id`
  (nullable, FK yok; silinen cihazın referansları `DeleteDevice` içinde
  NULL'lanır).

### Kırıcı / yükseltme
- Yok. Ancak **v0.3.3 öncesi kurulmuş agent'lar** derin toplama açık değilse
  `agent.yml`'e `collect.pcap: true` elle eklenmeli (yeniden kurulum mevcut
  config'e dokunmaz) — bkz.
  [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md) "Süreçler / DNS / L7
  panelleri boş".

## [0.3.2] — 2026-09-07

Kendinden-barındırma sertleştirme + arayüzün htop/TUI'ye dönüşümü. Sürüm etiketi
bir MSI derleme hatası (WIX0104) için kesildi ama v0.3.1'den bu yana **83
commit** — güvenlik değerlendirmesinin tüm bulguları, şema migrasyon çerçevesi,
yüksek erişilebilirlik ve çoklu-saha (MSP) modu — bu sürümdedir.

### Güvenlik
- `GET /api/alerts` admin-korumalı; bildirim sağlayıcı sırları (Telegram token,
  e-posta parolası, webhook secret, SIEM) yanıtta maskeli (B1).
- Agent otomatik güncelleme istemcisi hub'a `Authorization: Bearer` + mTLS
  istemci sertifikasıyla kimlik doğrular — önceden gerçek hub'da hep 401 (B2).
- `L7` / `DNS` / `Süreçler` / `Geo` / `Rapor` uçlarına site-kapsam uygulaması —
  site-kısıtlı kimlik artık filo genelini göremez (B3).
- `agent.yml` `0600 root:root`; Windows MSI enroll-token registry anahtarına
  DACL (Users okuyamaz) (B4).
- Etkin RBAC admin'i varken legacy `-auth-password` girişi reddedilir (B6).
- WebSocket `CheckOrigin` origin izin listesi (`-public-url` + `-tls-hosts`) —
  CSWSH savunması (B5).
- Vault `KeyProvider` arayüzü: `-vault-key-source=env` ile master anahtar diske
  yazılmaz (`BAZNTMS_VAULT_MASTER_KEY`) (B8).
- `ETag` / `If-None-Match` → `304` (`/api/v1/agents`, `/api/alerts/events`,
  `/api/v1/devices`) (D4).
- `docs/THREAT-MODEL.md`, `docs/DEPLOYMENT-MODEL.md` eklendi;
  `-race` + `golangci-lint` + CodeQL CI'a girdi.

### Eklendi
- **Kendi şema migrasyon çerçevesi** — gömülü sıralı `.sql` dosyaları
  (`internal/store/migrations/{sqlite,postgres}/NNNN_*.sql`) + dialect-koşullu
  Go adımları; uygulanmış sürümler `schema_migrations`'ta izlenir; CI'da
  "önceki release şeması → HEAD binary" yükseltme testi. `0001_init` donduruldu.
- **Yüksek erişilebilirlik:** paylaşımlı oturum deposu (`-session-store=db`,
  `0005_sessions`), lider seçimi (`store.Leader` = Postgres advisory lock),
  JetStream ölü-mektup kuyruğu (DLQ), 2× `hub-controller` + nginx compose.
- **Çoklu-saha / MSP modu** (`-multi-site`): `site` sert yetki sınırı,
  `site-admin` rolü + `PermGlobalAdmin`, saha-kapsamlı CRUD denetimi,
  `0004_audit_site`, site-bağlı enroll token zorunlu.
- **Yönetim arayüzü:** Kullanıcılar / API Token'ları / Agent Ekle sihirbazı /
  Denetim Kaydı sayfaları (`/yonetim/*`), son-admin koruması.
- Agent `machine_id` (`0003_agent_machine_id`) — hub state kaybında agent
  yeniden kaydolmak yerine mevcut kimliğe bağlanır (`RegisterOrReuseAgent`).
- Tam cascade `DeleteAgent` + N-gün-offline arşiv (`-agent-archive-days`).
- **Otomatik agent güncellemesi varsayılan açık** + hub'ın GitHub release
  senkronu (`-update-github-repo`, 30 dk yoklama → `updates/stable` manifest).
- Loopback stub-resolver DNS görünürlüğü (systemd-resolved / Docker 127.0.0.11).

### Değiştirildi
- **Arayüz tümüyle htop/TUI diline dönüştürüldü** (Faz 17–18): Sidebar / Header /
  Card kaldırıldı, klavye-öncelikli gezinme (1–9 sekme, `j/k`, `/`, F-tuşları),
  `Panel` / `Meter` / `TuiTable` / `Sparkline` primitifleri, tek dim tonu,
  ince CRT; tüm native `prompt/confirm` → `useDialog()`; responsive/mobil
  (sayfa asla yatay kaymaz). CSS 50 KB → 37 KB.
- `Store` arayüzü 10 alan alt-arayüzüne bölündü (`store.go` 748 → 126 satır).
- Canlı Akış + Coğrafi Trafik panodan ayrı sekmelere taşındı (rAF animasyonu
  yalnız o sayfada).

### Düzeltildi
- Windows agent: friendly arayüz adı (`Ethernet`) otomatik `\Device\NPF_{GUID}`
  pcap adına çevrilir — önceden hata 123 ile atıf/L7/DNS motoru hiç başlamıyordu.
- Paket yakalamadan bağımsız DB bakımı (`Prune` + retention) ve
  anomali/baseline motoru çoklu-hub'da filo telemetrisinden beslenir.
- Tüm platformlarda "sıfırdan yeniden kurulum" (deb `dpkg -P`, rpm `rpm -e`,
  MSI uninstall-first, macOS `launchctl unload`, docker `rm -f`).

### Kırıcı / yükseltme
- **RBAC yapılandırılmışsa** (`users` tablosunda kayıt varsa) legacy
  `-auth-password` admin girişi artık reddedilir — bir admin kullanıcısı
  oluşturun (`bazntmsctl` / yönetim UI) veya `users` boşken devam edin.
- Ters-proxy / tünel arkasındaki kurulumlarda WebSocket bağlantısı için hub'ı
  `-public-url https://<dış-adres>` ile başlatın (yoksa origin reddi).
- Şema migrasyonları otomatik ama geri alınamaz — yükseltmeden önce yedek.

## [0.3.1] — 2026-09-05

### Düzeltildi
- **Windows agent paket yakalama arayüzü**: friendly ad (`Ethernet`) artık
  otomatik `\Device\NPF_{GUID}` pcap cihaz adına çevriliyor; `auto` seçimi
  yönlendirilebilir IPv4'lü arayüzü tercih ediyor. Önceden "Error opening
  adapter" (123) ile süreç atfı / L7 / DNS motoru Npcap doğru kurulu olsa bile
  başlamıyordu.
- Agent sürümü her telemetri batch'inde taşınıyor — MSI reinstall / self-update
  sonrası hub'ın gösterdiği sürüm donuk kalıyordu.
- macOS süreç trafiği atfı: `lsof` `P` (protokol) alanı kullanılıyor.

### Değiştirildi
- Landing page ve Docusaurus sitesi yeniden tasarlandı; dashboard bileşenlerinde
  kapsamlı erişilebilirlik + renk sözleşmesi geçişi.

## [0.3.0] — 2026-09-04

### Eklendi
- **Agent ↔ hub karşılıklı TLS (mTLS)**: hub CA'sı, enrollment'ta istemci
  sertifikası, ömrün yarısında otomatik yenileme; ölçekte L4 passthrough +
  paylaşılan CA.
- **Akış toplama**: NetFlow v9 + IPFIX (şablon tabanlı) + saatlik continuous
  aggregate (`flows_1h`, 1 yıl); sFlow v5 (stdlib-only parser).
- **Derin telemetri**: süreç bazlı L7 uygulama görünürlüğü (TLS SNI + HTTP Host)
  → `/api/v1/l7`; süreç bazlı DNS alan adları → `/api/v1/dns`.
- IOC / tehdit istihbaratı: L7 + DNS kara liste eşleşmesi.
- OpenAPI 3.1 şeması + CDN'siz tarayıcı (`/api/docs`), router ↔ şema drift testi.
- SIEM/ITSM push connector (CEF / LEEF / JSON → syslog veya HTTP POST).
- Dashboard coğrafi trafik haritası (GeoIP → ülke merkezleri, elle SVG).
- WebSocket tick'i filo geneli canlı özet taşıyor.

### Düzeltildi
- Site-kapsam RBAC cihaz / NetFlow / syslog / topolojiye yayıldı.
- `DeleteAgent` bağlı tüm filo tablolarını temizliyor; raporlar/anomali
  çoklu-hub'da filo veri modelinden üretiliyor.

## [0.2.8] — 2026-09-01

### Düzeltildi
- Linux (deb + rpm) kurulum sihirbazı RPM'de hiç çalışmıyordu (`/dev/tty` ile
  düzeltildi); otomasyon senaryosunu bozan regresyon giderildi.

## [0.2.7] — 2026-09-01

### Eklendi
- Windows MSI + macOS `.pkg` + Linux deb/rpm kurulumlarının üçü de sunucu
  bilgisi (Hub URL / Token / Site) soruyor.

### Düzeltildi
- Agent servisleri (macOS launchd + Linux systemd) state dosyasını köke yazmaya
  çalışıp çöküyordu; macOS `.pkg` postinstall yanlış dosya adı arıyordu.

## [0.2.6] — 2026-09-01

### Değiştirildi
- MSI kurulumu hub bilgisi girildiyse servisi otomatik başlatır; düzenleme
  alanlarındaki sahte varsayılan değerler kaldırıldı.

## [0.2.5] — 2026-09-01

### Eklendi
- Hub'dan agent yönetimi (yeniden adlandırma / silme).
- Uyarı motoru artık agent filosunu izliyor (offline, sürüm sapması,
  kaynak eşikleri).

## [0.2.4] — 2026-08-30

### Değiştirildi
- Windows MSI kurulumu gerçek sihirbaza dönüştü (WiX v4).

## [0.2.3] — 2026-08-30

### Eklendi
- Hub arayüzü "Tüm Kartlar"dan ayrı sayfalara bölündü: Genel Bakış, Agent'lar
  (liste + derin detay), Cihaz Detayı, Uyarılar (anomali + FortiGate), Raporlar
  (kurumsal + uyumluluk), Uyumluluk alt sayfaları.
- Frontend CI + test altyapısı; 404 rotası.

### Değiştirildi
- Hub container'ı root olmadan çalışıyor (agent kasıtlı olarak root kalıyor).
- Hub'ın kendi yakalama kontrolleri arayüzden kaldırıldı (artık salt izleme).

## [0.2.2] — 2026-08-30

### Eklendi
- Servis modunda log dosyası (teşhis).

## [0.2.1] — 2026-08-30

### Düzeltildi
- Windows MSI servis kurulumu (SCM desteği + kurulum sırasında sunucu bilgisi).

## [0.2.0] — 2026-08-30

Tek-makine monolitten **hub + uç agent + ölçek altyapısı**na geçiş
(enterprise yol haritası Faz 0–6). Mevcut yeteneklerin tamamı hub tarafına
taşındı — atılan iş yok.

### Eklendi
- Binary bölünmesi (`bazntms-hub` / `bazntms-agent` / `bazntmsctl`), koanf
  YAML+env config, `slog` JSON loglama, `/api/v1` + `/metrics` + `/healthz` +
  `/readyz`, testcontainers, `telemetry.proto`, sürüm paketi.
- Agent enrollment + telemetri hattı (offline disk kuyruğu), agent auth
  middleware, uzak politika kanalı.
- Süreç bazlı trafik atfı (`proctraffic` sağlayıcıları: proc/lsof/netstat),
  PCAP politikası.
- Ağ cihazı entegrasyonları: SNMPv3 poller (IF-MIB), NetFlow v5 collector,
  syslog alıcı, AES-GCM kimlik kasası.
- Ölçek: `store.Store` arayüzü + PostgreSQL/TimescaleDB (hypertable, continuous
  aggregate, retention), NATS JetStream kuyruğu, Helm chart + docker-compose,
  loadgen / k6.
- Güvenlik: roller (admin / netops / analyst / viewer) + site scope, OIDC SSO,
  API token'ları, hash-zincirli audit log, agent çoklu-hub failover, DR runbook
  + yedek scriptleri, `govulncheck` / `syft` / `trivy` / `cosign`.
- İleri analiz: topoloji keşfi (SNMP LLDP/CDP/ARP + agent subnet'leri) + SVG
  harita, z-score anomali motoru (saatlik baseline), Teams/SMTP/webhook v2
  bildirimleri, SLA/kapasite/banding raporu.

## [0.1.0] — 2026-08-28

İlk sürüm. Tek makinede çalışan ağ trafiği izleme aracı: paket yakalama,
SQLite kayıt, uyarı motoru, AI analizi, GeoIP, PCAP kaydı, rapor ve gömülü
dashboard — tek binary.

[Yayımlanmamış]: https://github.com/gokayybaz/bazntms/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/gokayybaz/bazntms/compare/v0.3.3...v0.4.0
[0.3.3]: https://github.com/gokayybaz/bazntms/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/gokayybaz/bazntms/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/gokayybaz/bazntms/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/gokayybaz/bazntms/compare/v0.2.8...v0.3.0
[0.2.8]: https://github.com/gokayybaz/bazntms/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/gokayybaz/bazntms/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/gokayybaz/bazntms/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/gokayybaz/bazntms/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/gokayybaz/bazntms/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/gokayybaz/bazntms/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/gokayybaz/bazntms/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/gokayybaz/bazntms/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/gokayybaz/bazntms/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/gokayybaz/bazntms/releases/tag/v0.1.0
