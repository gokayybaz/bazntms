# Mimari

bazNTMS, tek Go binary'si içinde çalışan bir monolittir: yakalama motoru,
SQLite/PostgreSQL collector, uyarı motoru, AI istemcisi, rapor üreticisi ve
HTTP/WS sunucusu aynı süreçte yaşar. Frontend derlenip binary'ye gömülür
(`go:embed all:frontend/dist`). Depo katmanı `store.Store`
arayüzüne ayrıldı: SQLite dev modunda kalır, ölçek modunda
PostgreSQL/TimescaleDB + opsiyonel NATS JetStream kuyruğu devreye girer.

```
main.go
  ├─ store.Open()            SQLite (dosya) veya PostgreSQL/TimescaleDB (postgres:// DSN) aç + migrasyon
  ├─ queue.Connect()         opsiyonel NATS JetStream (ingest → processor ayrışması)
  ├─ capture.NewEngine()     yakalama motoru
  ├─ store.NewCollector()    örnekleyici (saniye/dakika yazımları)
  ├─ alert.NewManager()      uyarı kural motoru
  ├─ geoip.New()             MMDB / ip-api çözümleyici
  ├─ ai.NewClient()          OpenAI-uyumlu istemci
  └─ server.New()            REST + WS + SPA
```

## Yakalama motoru (`internal/capture`)

`Engine.Start(device)` bir libpcap handle'ı açar (snaplen 65535, promisc kapalı,
1 sn okuma zaman aşımı) ve `loop()` goroutine'ini başlatır.

**Kritik tasarım kararı:** `gopacket.NewPacketSource()` **bilinçli olarak
kullanılmaz**. PacketSource kendi içinde gizli bir okuma goroutine'i
(`packetsToChannel`) başlatır; `handle.Close()` o goroutine `ReadPacketData`
içindeyken çağrılırsa serbest bırakılan `pcap_t` üzerinde okuma devam eder ve
use-after-free SIGSEGV oluşur. Bunun yerine paketler ana döngüde elle okunur:

```
loop():  select { stopCh | tick'ler | default } → handle.ReadPacketData()
         → recordPacket(data, ci) → gopacket.NewPacket(Lazy, NoCopy) → process()
```

`Stop()` sırası: `close(stopCh)` → `<-doneCh` (döngü çıkışı) →
`handle.Stats()` (handle canlıyken) → `handle.Close()` → `e.handle = nil`.
`gopacket`'ın `Stats()`'ı kapalı-handle kontrolü yapmaz; sıra bu yüzden önemlidir.
`stopMu`, Start/Stop'u serileştirir (çift close + kapalı handle yarışları).

### İstatistik toplama
- Yön tespiti: kaynak/hedef IP yerel arayüz adresleriyle karşılaştırılır
  (`refreshLocalNets`, 15 sn'de bir tazelenir); her iki taraf ayrı sayılır
- `endpoints` (IP bazlı in/out/paket), `ports`, `protocols`, `dnsCounts`
- Saniyelik `history` kovaları (120 kayıt; grafik doğrudan bundan beslenir)
- DNS: UDP/53 payload'ı `layers.DNS` ile ayrıştırılır; `in-addr.arpa`/`ip6.arpa`
  filtrelenir; yanıtlardaki A/AAAA kayıtları sorulan domain'e eşlenir
  (domain başına en fazla 4 IP, toplam 4096 domain sınırı)

### PCAP kayıt (`record.go`)
`recMu` altında `pcapgo.Writer` ile klasik `.pcap` yazımı. Dosya limiti aşılırsa
rotasyon. Yalnızca ana döngü yazar; `Stop()` döngü çıkışından sonra kaydı kapatır.

## Collector (`internal/store/collector.go`)

| Zamanlayıcı | İş |
|-------------|-----|
| 1 sn | `engine.Snapshot()` → `samples` tablosuna satır (yalnızca `running` iken) |
| 1 dk | Uç nokta/DNS **delta** hesabı: önceki kümülatif − şimdiki; `connection_events` yazımı |
| 10 dk | `Prune(retention)`: tabloların süre aşımı temizliği |

Motor her `Start()`'ta sayaçları sıfırladığı için delta fonksiyonu
`a < b` durumunda "sıfırlandı" kabul eder ve yeni toplamı fark olarak yazar.

## Uyarı motoru (`internal/alert`)

`Manager` saniyede bir `Snapshot` değerlendirir; süreç/port kuralları 5 saniyede
bir `sysmon.ListConnections()` ile:

| Kural | Mekanizma |
|-------|-----------|
| `bw` | bps eşikini üst üste N saniye aşma sayacı |
| `port` | kurulan bağlantının uzak portu şüpheli listede mi |
| `proc` | `alert_seen` tablosuna karşı yeni süreç; ilk çalıştırmada taban çizgisi sessizce atılır |
| `target` | ilk kez ≥ X MB trafik gören uzak IP; kalıcı görüldü işareti |
| `ioc` | agent'ın gördüğü L7 (SNI/Host) + DNS alan adları tehdit istihbaratına (`internal/threatintel`, Faz 24-E — sağlayıcı-bağımsız; `-ioc-file` → `localfile` sağlayıcı, domain + **IP** kara listesi, mtime hot-reload) sorulur; suspicious/malicious → uyarı (itibar + kaynak taşır). İmza tabanlı DPI değil; **oto-blok yok** |
| `iface_util` | SNMP arayüz verimi güvenilir hızın (ifSpeed/ifHighSpeed) `warn_pct`/`crit_pct` eşiğini `sustain_sec` boyunca aşma (Faz 23-C, `internal/alert/iface.go`). loopback/tünel atlanır |

Her olay `kind|key` başına cooldown (varsayılan 10 dk) tabi tutulur; geçenler
`alert_events`'e yazılır ve `Notifier` ile (masaüstü, Telegram, Discord, Slack,
generic webhook, Teams, e-posta, imzalı webhook v2 — hepsi asenkron, hatalar
yutulur) dağıtılır. **SIEM/ITSM bağlayıcı** (`internal/alert/siem.go`): olay
CEF (ArcSight) / LEEF (QRadar) / JSON'a formatlanıp RFC3164 syslog (UDP/TCP)
veya HTTP POST (Splunk HEC, ServiceNow, jenerik toplayıcı) ile iletilir;
`notifiers.siem` altından yapılandırılır.

**Olay (incident) korelasyon motoru (`internal/incident`, Faz 24-B):**
lider-kapılı (uyarı motoruyla aynı lider), ~30 sn'de bir `AlertEventsSince`'i
agent bazında (`alert_events.agent_id` — 0018) gruplayıp 5 deterministik kurala
uygular (yeni-süreç+yeni-hedef/ioc, +şüpheli-port, hedef+bant, anomali+bant,
≥N-şüpheli). **AI/LLM YOK.** Dedup = `correlation_key` ("r<kural>|agent<id>") —
anahtar başına tek açık incident; tekrar → severity **yalnız yükselir**, risk
en yükseği tutar. Risk skoru 0-100 açıklanabilir (kural tabanı + önem + kanıt).
Bildirim: `alerts.NotifyIncident` sentetik AlertEvent'e çevirip mevcut
kanallardan. Bkz. `docs/decisions/0011-incident-engine.md`.

**Normalleştirilmiş olay akışı (`internal/store/events.go`, Faz 24-A):**
uyarılar (`alert_events`) ile ham gözlemler ayrılır (ADR 0010). `QueryEvents`
mevcut kaynak tabloları (`agent_dns`, `l7_endpoints`, `flows`, `syslog_events`,
`connection_events`) `UNION ALL` ile tek normalize şemaya sunar — **yeni yazma
hattı yok**. `GET /api/v1/events?type=&agent_id=&device=&since_min=&before=&limit=`
(ts-imleçli). `EventStore` alt-arayüzü. `process.started` / `connection.opened`
kapsam dışı (bkz. ADR); korelasyon motoru (24-B) bu akışı kanıt kaynağı olarak
kullanır.

## AI istemcisi (`internal/ai`)

OpenAI-uyumlu `/chat/completions` çağrıları; iki mod:

- **Tek seferde**: tüm veri tek JSON olarak gider
- **Chunked**: 4 veri bölümü ayrı isteklerle → her birinden kısa not → final
  istekte yalnızca notlar birleştirilir. Ham veri ikinci kez gitmez.

Reasoning modelleri için: `message.reasoning_content` / `reasoning` fallback,
`<think>` bloklarının temizlenmesi, `finish_reason=length` için açıklayıcı hata,
`/no_think` (Qwen3) ve `-llm-max-tokens` override.

## Akış toplama (`internal/flows`)

Tek bir `Collector` UDP dinleyicisi, gelen datagramı versiyonuna göre ayırır:

| Protokol | Ayırt edici | Çözücü |
|----------|-------------|--------|
| NetFlow v5 | ilk 2 bayt = 5 | `ParseV5` (sabit 48 baytlık kayıt) |
| NetFlow v9 | ilk 2 bayt = 9 | `ParseV9` + `TemplateCache` (exporter×domain×templateID) |
| IPFIX / v10 | ilk 2 bayt = 10 | `ParseIPFIX` (aynı cache) |
| sFlow v5 | ilk **4** bayt = 5 | `ParseSFlow` |

Çakışma yok: NetFlow'un ilk 4 baytı uint32 olarak daima ≥ `0x50000`
(`version<<16 + count`), sFlow'unki tam olarak `5`. Üçü de aynı `-flow-port`'ta karışık
gelebilir; `-sflow-port` yalnızca ayrı bir bind ister.

sFlow **örnekleme tabanlıdır**: cihaz her N. paketin başlığını kopyalar.
`ParseSFlow` "flow sample" (format 1/3) içindeki ham paket başlığını elle çözer
(Ethernet → 802.1Q atla → IPv4 → TCP/UDP; stdlib-only, gopacket yok) ve
örnekleme oranıyla ölçekler: `Packets = rate`, `Octets = frame_length × rate`.
Counter sample'lar (arayüz sayaçları) şimdilik atlanır. Çıktı NetFlow ile aynı
`flows` tablosuna, aynı `FlowRow` şemasıyla yazılır.

**Konuşma toplama (Faz 23-B, `internal/store/flow_conversations.go`):**
`GET /api/v1/flows/conversations` ham `flows`'u bir zaman penceresinde
(`15m|1h|6h|24h`) **sunucu-tarafı** toplar — `by=5tuple`
(src,dst,src_port,dst_port,proto) ya da `by=pair` (uç-çifti, A→B ve B→A Go
tarafında birleştirilir, kanonik `Src` = sözlüksel küçük uç). `sort` ∈
octets|packets|flows|last_seen. **Yeni cagg yok** (src/dst yüksek kardinalite —
`pg.go` `flows_1h` notu): görünüm ham `flows` retention penceresiyle (vars. 7g)
sınırlı; `pair` modu birleştirme öncesi `flowConvoPairCap=2000` grup çeker.
`0015` migrasyonu `idx_flows_convo (ts,src,dst,proto)` + `idx_flows_pair
(src,dst,ts)` ekler. Drill-down `GET /api/v1/flows/conversation?src=&dst=` ham
akışları + uç GeoIP/ASN (`s.geo`) + `process_traffic.remote_ip` eşleşmesiyle
ilişkili agent/süreç döndürür. Frontend `TopConversationsCard` → `/cihazlar`.
Ham NetFlow görünümü (`/api/v1/flows`, `FlowsCard`) değişmedi.

## Sunucu (`internal/server`)

- `ServeMux` (Go 1.22+ metot kalıpları) + `logRequest` middleware
- `auth.middleware`: `/api/*` ve `/ws` oturum denetimi; `/api/login`,
  `/api/auth/status`, OIDC ve `/api/openapi.{yaml,json}` + `/api/docs` muaf;
  statik dosyalar açık (SPA kabuğu). Sabit zamanlı şifre karşılaştırma, IP
  bazlı deneme sınırı, HttpOnly cookie + Bearer token
- **API sözleşmesi** (`api/openapi.yaml`, `internal/server/openapi.go`): elle
  bakımlı OpenAPI 3.1 şeması binary'ye gömülür; `/api/openapi.yaml` (ham),
  `/api/openapi.json` (yaml→json) ve `/api/docs` (tek dosya, CDN'siz gezgin)
  uçlarında sunulur. `TestOpenAPICoversRouter` şemayı `server.go`'daki gerçek
  rota kayıtlarına karşı iki yönlü doğrular (drift = kırık test)
- `Hub`: saniyede bir `tick` yayınlar — **alarm olayları** + **filo özeti**
  (`store.FleetSummary`: agent online/toplam, son ~90 sn ortalama rx/tx/pps,
  son 1 dk NetFlow hızı). Filo özeti Hub'da ~3 sn önbelleklenir (N istemci ×
  1 sn tick DB'yi dövmesin). Boş odaya tick üretilmez; hub yerel yakalama
  Snapshot'ı tick'ten çıkarıldı (dağıtık modda boş — bkz. "Not: hub'ın kendi
  yerel yakalaması")
- Statik servis: `http.FileServerFS` + SPA history fallback

### Agent ↔ hub mTLS (`internal/pki`)

`-tls` ile hub `<tls-dir>/ca.{crt,key}` (yoksa üretir — ECDSA P-256, 10 yıl)
ve buradan bir sunucu sertifikası (`server.{crt,key}`, SAN'lar `-tls-hosts` +
`localhost`/IP'ler + hostname, ~13 ay, süre bitince yenilenir) tutar.
`tls.Config.ClientAuth = VerifyClientCertIfGiven`: tarayıcı sertifikasız
bağlanır, agent sertifikası **sunulursa** CA'ya karşı doğrulanır.

Enrollment: agent bir ECDSA anahtar + CSR üretir, `hello.csr_pem` ile
gönderir; hub `CN=bazntms-agent-<id>` (agent'ın iddia edemeyeceği), ClientAuth
EKU'lu, 90 günlük bir istemci sertifikası imzalayıp CA ile birlikte döner.
Agent bunları `<state>.{crt,key,ca}` olarak yazar ve sonraki tüm bağlantılarda
istemci sertifikası + pinlenmiş CA kullanır. `agentAuth` middleware'i
`r.TLS.VerifiedChains` doluysa CN'den `agent_id` çözer (Bearer'a eşdeğer);
silinmiş agent `AgentByID` hatasıyla reddedilir (CRL yok). Ömrün yarısı
geçince agent `POST /api/v1/agent/cert` ile yeniler.

CA pinleme: agent `-hub-ca <dosya>` ile önceden sağlar; yoksa ilk hello
`InsecureSkipVerify` (TOFU) ile yapılır ve dönen CA pinlenir — enrollment
token o ilk el sıkışmada kimliği sağlar. L7 reverse proxy mTLS'i sonlandırır;
mTLS'te agent'lar hub'a doğrudan ya da L4 passthrough LB ile bağlanmalı.

## Frontend (`frontend/`)

Vite + React + Tailwind v4 + `react-router-dom`. **Tasarım dili htop/ncurses
TUI** (Faz 17) — tam ayrıntı [`frontend/DESIGN.md`](../frontend/DESIGN.md):
düz siyah zemin, tek monospace aile, kare köşe (`*{border-radius:0}`), gölge
yok, klavye-öncelikli. İmza bileşenler: `Meter` (htop eşik çubuğu), `TuiTable`
(sort/`/` filtre/↑↓-jk klavye-nav kolonlu tablo), `Panel` (tek konteyner —
eski `Card.tsx` bunun alias'ı), `Sparkline` (blok rampası), `TabBar`/`FnKeyBar`.
Renk tokenleri `index.css` `@theme` (`--color-ground/panel/rule/ink/tui-dim/rx/tx`).

`useLive` hook'u WS'i birincil, REST yoklamasını yedek kaynak yapar (WS koparsa
otomatik dönüş). WS tick'i alarm olayları + `fleet` özetini taşır → üst
`TuiHeader` şeridinde **filo rx/tx/pps Meter bandı + olay/uyarı sayaç + canlı
saat** 1 sn'de güncellenir (WS yoksa 5 sn'lik REST'ten). **Agent sayısı
(aktif/toplam) ise her zaman `/api/v1/agents` REST listesinden** gelir — canlı
trafik şeması, topoloji ve alttaki filo tabloları da aynı listeyi kullandığı
için "aktif agent" sayısı bu görünümlerle her zaman tutarlıdır
(`fleet.agents_online` yalnızca ilk poll gelene kadar geçici kaynak).
401 görülürse App TTY login ekranına düşer; 60 sn'de bir de oturum denetimi
yapılır. Grafikler (ThroughputChart — basamaklı çizgi, gradyan yok) harici
grafik kütüphanesi olmadan, elle yazılmış SVG'dir.

**Klavye modeli** (`lib/useHotkeys.ts` + `lib/KeymapContext.tsx`): tek global
`keydown`; `1-9` sekme değiştir, `↑↓/jk` + `Enter` liste gezinme, `/` filtre,
`s`/`F6` sırala, `F1`/`?` yardım overlay, `F5` yenile, `F10` çıkış. Metin alanı
odaktayken tek-harf kısayolları bastırılır. `FnKeyBar` alt şeridi aktif ekranın
`useRegisterKeys` ile kaydettiği eylemleri çizer.

Dashboard'daki **`TrafficFlowDiagram`** de aynı yaklaşımla elle yazılmış SVG
bir sahnedir: sol sütunda **yalnızca ÇEVRİMİÇİ agent'lar** ayrı bir istemci
düğümü (monitör ikonu + en yoğun arayüz hızı), ortada Router/Güvenlik Duvarı,
sağda İnternet. Kapalı agent'lar hiç çizilmez ve paket üretmez — bileşen tam
filoyu alır, `online` alanına göre süzer, alt etikette "N çevrimdışı gizli"
ipucu verir; bayat/bilinmeyen agent olayları (`resolveIdx` → DROP) ve agent
yokken üretilen ambient paketler de bastırılır. Her canlı akış/agent/syslog
olayı, olayı üreten agent'ın düğümünden yön (giden/gelen/yerel/olay) bazlı
animasyonlu bir "paket" geçirir; NetFlow olayları (agent'sız) ve cihaz syslog'u
doğrudan güvenlik duvarı ↔ internet ekseninde akar. viewBox yüksekliği çevrimiçi
agent sayısıyla büyür, düğüm detayı (tam/kompakt/mini) filo kalabalıklaştıkça
düşer. Paketler
`packetsRef`'te tutulur, tek bir `requestAnimationFrame` döngüsü boştayken
sessiz kalıp yalnızca hareket varken yeniden çizdirir; yön sınıflandırması
(`from`/`to` özel-genel IP ekseni) `lib/traffic.ts`'te, `TrafficFlowDiagram.test.tsx`
+ `lib/traffic.test.ts` ile kaplı. `prefers-reduced-motion` altında animasyon durur.

### Sayfa yapısı (routing)

`App.tsx` bir kabuk (`grid-rows-[auto_auto_1fr_auto]`): üst `TuiHeader` (filo
Meter bandı + WS durumu + kimlik + saat) / `TabBar` (numaralı yatay nav, `1-9`
tuşları) / `<main>` (kayan sayfa gövdesi, `<Routes>`) / alt `FnKeyBar`
(bağlam F-tuşları). `KeymapProvider` + `DialogProvider` tüm kabuğu sarar.
Sol sidebar ve ayrı header kaldırıldı (Faz 17). Backend zaten SPA
history-fallback sağladığı için (`server.go`: bilinmeyen path → `index.html`)
istemci tarafı routing ek backend desteği gerektirmeden çalışır.

| Rota | Sayfa | Veri kaynağı |
|------|-------|--------------|
| `/` | Dashboard (`Overview` bileşeni) — meter bandı + log-tail + filo/topoloji/cihazlar | agent/cihaz/flow/syslog özet — kendi polling'i + WS filo özeti (`useLive`) |
| `/agentlar`, `/agentlar/:id` | Agent listesi + derin detay | `GET /api/v1/agents[/…][/history]` |
| `/agentlar/:id/surec/:ad` | Süreç detayı (Faz 23-A) — Süreç Trafiği tablosunda `Enter`; özet/uzak hedefler/canlı bağlantılar/uygulama görünürlüğü (DNS+SNI+Host)/zaman çizelgesi. `Esc` → agent | `GET /api/v1/agents/:id/processes/:ad` (tek uç, sunucu-tarafı toplama) |
| `/cihazlar`, `/cihazlar/:id` | Cihaz listesi + derin detay + **Top Konuşmalar** (Faz 23-B — NetFlow 5'li/uç-çifti toplama, `Enter` → drill-down) | `GET /api/v1/devices[/…]`, `GET /api/v1/flows[/conversations][/conversation]`, FortiGate için `FortiPanel` |
| `/akis` | Canlı Trafik Şeması (`TrafficFlowCard` → animasyonlu SVG) — panodan ayrı sekme, rAF yalnız burada. `F4` tam ekran; admin'e `F3` "Düzenle" → agent'ları switch/AP cihazlarına gruplar (`agents.uplink_device_id`), okları ara katman üzerinden çizer | `GET /api/v1/agents`, `/flows`, `/syslog`, `/agents/:id`, `/devices`; `PUT /api/v1/agents/:id/uplink`, `POST /api/v1/devices` |
| `/cografi` | Coğrafi Trafik (`GeoMapCard` → dünya haritası balonları) — panodan ayrı sekme | `GET /api/v1/geo` |
| `/topoloji` | Ağ topolojisi (SVG, yatay: client ▸ hub ▸ cihaz ▸ router ▸ internet; Router `kind` router/firewall cihazından türer). Faz 23-D: SNMP-destekli kenarlar canlı telemetri (util → renk/durum: normal/uyarı≥%70/kritik≥%90/down), `Enter`/tık → link inspector (hız/RX/TX/kullanım/hata/iskarta), güven rozeti | `GET /api/v1/topology` (kenar `telemetry` = `local_port` ↔ `LatestDeviceIfaces` eşleşmesi; `confidence` = discovered\|inferred\|manual, `0017`) |
| `/uyarilar` | Alt sekmeler: **Alarmlar** (yaşam döngüsü tablosu + eşik/bildirim ayarları) · **Olaylar** (Faz 24-B/C — korele incident listesi + `/uyarilar/olay/:id` detay: özet/korelasyon/kanıt zaman çizelgesi/RELATED/ACTIONS; sekmede açık-sayı rozeti) · **Olay Akışı** (Faz 24-A — normalleştirilmiş ham gözlem, ADR 0010) | `GET /api/v1/alerts/{events,silences}` + `POST .../events/:id/{ack,resolve,note}` + `GET/PUT /api/alerts` · `GET /api/v1/events` · `GET /api/v1/incidents[/:id]` + `POST .../{ack,investigate,resolve,close}` |
| `/anomali` | Anomali paneli — mevsimsel "beklenen ±2σ bant vs gerçek" grafiği (elle SVG) + aktif sapmalar (filo/saha/agent × bps/dns/proc) | `GET /api/v1/anomaly/{baseline,active}` |
| `/raporlar` | Ağ trafiği + kurumsal (SLA/kapasite/saha kırılımı, PDF) + uyumluluk raporları · zamanlanmış teslim + arşiv · SLA hedefleri | `GET /api/report?type=…` · `GET/POST/DELETE /api/v1/reports/*` · `/api/v1/sla/targets` |
| `/uyumluluk`, `/uyumluluk/{risk,soa,politikalar,denetimler,yonetisim}` | 5651 + ISO 27001 ISMS | `GET/POST/PUT /api/v1/isms/*`, paylaşılan tip/yardımcılar `lib/isms.tsx`'te |
| `*` | 404 | — |

Uyumluluk/Yönetim CRUD akışları çok-alanlı `useDialog().form()` TUI dialog'unu
kullanır (art arda native `prompt()` zincirleri kaldırıldı); `ComplianceSubNav`
ve `AdminPageShell` ikincil şeritleri `TabBar` diliyle çizilir.

Her sayfa **yalnızca kendi ihtiyacı olan uçları** kendi `useEffect`'inde
çeker (genelde 5–20 sn aralıklı `setInterval` ile); ortak bir global store
yok. Kendi polling'i olmayan birkaç bileşen (`DevicesCard`, `TopologyCard`,
`ComplianceCard` vb.) `refreshKey` prop'una bağlıdır — bu değer
`App.tsx`'te 20 sn'de bir otomatik artan `historyRefresh` state'inden gelir
(önceden kaldırılan Trafik sayfasındaki elle "geçmişi yenile" düğmesinin
yerini almıştır).

**Not — hub'ın kendi yerel yakalaması:** Hub, merkezi bir toplama noktası
olarak konumlandığı için navbar'da kendi arayüz seçici/yakalama başlat-
durdur kontrolleri **yok** — yalnızca agent/cihaz filosundan gelen veriler
görünür. Yerel yakalamayla ilgili bileşenler (StatCards, ThroughputChart,
EndpointsTable, ConnectionsTable, DnsCard, AICard, CompareCard, PcapCard
vb.) bu nedenle kaldırıldı; ilgili REST uçları (`/api/capture/*`,
`/api/history`) backend'de hâlâ durur ve CLI/otomasyon veya tek-makine
(standalone) kurulumlar için kullanılabilir.

**Rapor motoru (`internal/report`):** trafik + kurumsal raporlar tümüyle
**filo verisinden** üretilir — `store.Fleet*` sorguları agent arayüz
telemetrisi (`agent_iface_samples`, kümülatif sayaç → `LAG()` ile delta),
NetFlow (`flows`), agent süreç trafiği (`process_traffic`) ve SNMP cihaz
sayaçlarından (`device_iface_samples`) beslenir. Hub yerel yakalamasının
`samples`/`endpoint_stats`/`dns_queries` tablolarına **bağlı değildir**
(çoklu-hub kurulumunda tüm hub'lar `-capture=false` çalıştığı için o tablolar
boştur). Protokol/hacim dağılımı (`FleetProtocolTotals`) TimescaleDB modunda
`flows_1h` continuous aggregate'inden okunur → ham `flows` 7 günde düşse de
protokol trendi 1 yıl tutulur (30/90 günlük rapor doğru çıkar). Diğer filo
ham tabloları (`agent_iface_samples`, `process_traffic`) için cagg yok — o
metriklerde pratik rapor penceresi hâlâ retention süresiyle sınırlı.

Kurumsal rapor (Faz 25-B) ayrıca **yönetici özeti** (KPI ızgarası: ağ sağlık
skoru + erişilebilirlik + açık olay + kapasite riski), **ağ sağlık skoru**
bölümü (`internal/health`, 25-A), **top konuşmalar** (NetFlow, 23-B),
**DNS / uygulama görünürlüğü** (süreç-atıflı DNS + TLS SNI/HTTP Host), **açık
olaylar** (incident korelasyonu, 24-B) ve **öneriler** taşır. Öneriler
`recommend()` — eşik-tabanlı **deterministik şablonlar**, LLM yok; her madde
bir metriğe ve eşiğe bağlıdır. Ek bölümler best-effort doldurulur (kaynak
eksikse bölüm zarifçe "veri yok" der, rapor 500 vermez).

**Hedef zenginleştirme (`internal/enrich`, Faz 23-E):** paylaşılan `Service`
uzak IP → `{private, country, asn, org}` (geoip.Resolver'a devreder — o zaten
100k LRU önbellekli; RFC1918/ULA/loopback → `private=true`, lookup yok) ve alan
adı → `{normalized, registrable, category}` (`x/net/publicsuffix` eTLD+1 +
opsiyonel `-domain-category-file`, bounded önbellek) döndürür. Süreç detayı
hedefleri (`/api/v1/agents/:id/processes/:ad`), NetFlow konuşma drill-down'ı
(`/api/v1/flows/conversation`), coğrafi harita ve anlık `GET /api/v1/enrich?ip=&domain=`
hepsi bu servisi kullanır. Frontend ortak render: `lib/enrich.tsx`
(`IpBadge`/`DomainBadge` — RFC1918 → `YEREL` pill). Zenginleştirme salt
okuma-yolu; hata/eksik veri ingest'i bloklamaz.

**Coğrafi trafik haritası (`/api/v1/geo` → `Overview` `GeoMapCard`):**
`store.FleetTopEndpoints` ile çıkarılan uzak uç noktalar `geoip.Resolver`
üzerinden ülkeye (ISO2) çözümlenir, `internal/geoip/centroids.go`'daki ~120
ülkelik merkez koordinat tablosuyla eşleştirilir ve equirectangular bir SVG
dünya haritasında hacme göre boyutlanmış balonlarla gösterilir (harici
grafik/harita kütüphanesi yok — kıtalar kaba poligon, konvansiyona uygun).
Sunucu tarafı toplama saf fonksiyon (`aggregateGeo`) olarak test edilir.
GeoIP kaynağı yoksa uç boş liste döner, kart "veri yok" durumuna düşer.

## Süreç Bazlı Trafik Atfı

Agent, "hangi süreç nereye ne kadar veri gönderdi + hangi domain'e baktı"
sorusunu üç değiştirilebilir arka uçtan biriyle yanıtlar. Ortak arayüz
`internal/agent/attrsource.go` `AttrSource` (`Deltas` / `L7Deltas` / `DNSDeltas`
/ `Stop` / `Method`); seçici `newAttrSource(cfg, caps)` platform + `collect.method`
+ çalışma-zamanı yeteneğine göre birini kurar ve eskisine **zarif düşer**.
Aktif yöntem her telemetri batch'inde `attr_method` ile hub'a bildirilir.

| Arka uç | Platform | Yöntem | Gereksinim |
|---------|----------|--------|-----------|
| **eBPF** | Linux | `internal/agent/bpf/` — CO-RE **fentry** (`tcp/udp_sendmsg`, `tcp_cleanup_rbuf`, `skb_consume_udp`) → `LRU_HASH` map; DNS = udp/53 payload → `RINGBUF` | kernel ≥ 5.8 + BTF + `CAP_BPF`/`CAP_SYS_ADMIN` |
| **ETW** | Windows | `internal/agent/etw_windows.go` — elle sarılmış advapi32 tüketicisi; `Microsoft-Windows-Kernel-Network` (bayt) + `Microsoft-Windows-DNS-Client` 3006/3008 (DNS) | yükseltilmiş süreç (SYSTEM / yönetici) |
| **pcap** | Linux / macOS / Windows | `internal/agent/attr.go` — **nethogs yöntemi**: pcap ile başlık yakala (snaplen 600), 4'lü çifti dönemlik soket-tablosu → PID eşlemesiyle sürece çevir | `CAP_NET_RAW` / Npcap + libpcap |

`auto` tercih sırası: Linux `eBPF → pcap`, Windows `ETW → pcap`, macOS `pcap`.
eBPF/ETW **paket yakalamaz** — çekirdeğin soket katmanına bağlanır; her *paket*
yerine her *send/recv işlemi* başına çalışır → belirgin şekilde ucuz, byte
sayımı offload'dan (GRO/LRO) etkilenmez, `CAP_NET_RAW` gerekmez.

**L7 uygulama görünürlüğü** (`internal/agent/l7.go`, `l7Tracker`): giden TCP
payload'ında TLS ClientHello **SNI**'si ve HTTP **Host** başlığı çıkarılıp
sürece atfedilir → `l7_endpoints` tablosu → `GET /api/v1/l7`. Payload
gerektirdiği için **yalnız pcap tabanlıdır**: pcap arka ucunda ana handle'dan,
eBPF modunda ise dar filtreli bir yardımcı pcap handle'dan (`l7helper.go`,
`tcp and dst port 443/80/8443/8080 and tcp-push`); ETW modunda **yoktur**
(`-collect-method=pcap` + Npcap gerekir). İmza tabanlı DPI değil — yalnızca
açıkça görünen alan adı.

**DNS görünürlüğü** (`internal/agent/dns.go` — ortak `parseDNSNames` +
`keepDomain` filtresi, ters arama / `.local` / noktasız hariç → `agent_dns`
tablosu):
- **eBPF:** `skb_consume_udp` içinde soket DNS taşıyorsa (uzak port 53 **veya**
  v4 loopback hedef) skb payload'ı ringbuf'a; **yanıtlar** yakalanır (soru
  bölümü domain'i taşır). systemd-resolved (127.0.0.53) / Docker gömülü DNS
  (127.0.0.11) **native** görünür — pcap'in eski loopback-handle hack'i
  (`loopback.go`) eBPF modunda hiç açılmaz.
- **ETW:** `DNS-Client` 3006 (sorgu) + 3008 (yanıt) → q/r ayrımı tam.
- **pcap:** ana handle (loopback-dışı arayüz) + ayrı bir `udp` loopback handle
  (`loopback.go`, best-effort; Windows'ta Npcap loopback adaptörü gerekir).
  Süreç atfı best-effort — DNS UDP soketleri milisaniyelik olduğundan
  `/proc/net/udp` anlığına çoğu zaman yakalanmaz (domain kaydı süreç atfından
  bağımsız).

pcap arka ucunda soket→PID kaynağı (`pkg/proctraffic`): Linux `/proc/net/*`
inode ↔ `/proc/[pid]/fd`, macOS `lsof -F`, Windows `netstat -ano` + gopsutil.
eBPF/ETW'de PID doğrudan çekirdek olayından gelir (`bpf_get_current_pid_tgid` /
`EventHeader.ProcessId`), süreç adı `comm` / gopsutil ile.

Hub politikası (`-agent-pcap`) + agent isteği (`-pcap` / `collect.pcap`) ikisi
de açıkken çalışır; hiçbir arka uç kurulamazsa atıf devre dışı kalır, temel
telemetri aksamaz. Ham PCAP kaydı (`-record`) her zaman pcap ister.

**Süreç detayı (Faz 23-A):** `GET /api/v1/agents/:id/processes/:ad` tek bir
sürecin (ad bazlı — PID zamanla değişir) tüm ağ etkinliğini **sunucu-tarafı
toplayarak** tek yanıtta döndürür: kimlik/özet (ilk-son görülme, PID'ler, bayt,
son-kova hız), uzak hedefler (`process_traffic` → `GROUP BY remote_ip,port,proto`
+ opportunistik GeoIP/ASN), canlı bağlantılar (`agent_conn_latest` süreç
filtreli — **yaş türetilemez**, o tabloda ts yok), uygulama görünürlüğü
(`agent_dns` ∪ `l7_endpoints`), zaman çizelgesi (ilk görülme · DNS/L7 ilk
temas · trafik sıçraması = kova toplamı > ort.+3σ · key'inde süreç adı geçen
`alert_events`). Yeni tablo/pipeline yok; RBAC site scope (`agentInScope`).
Store metotları `internal/store/process_detail.go`, frontend
`frontend/src/pages/ProcessDetailPage.tsx`.

## Veri akışı özeti

```
paket ─► process() ─► memory aggregates ─► WS tick (1 sn) ─► UI
                       └► Collector ─► Store: SQLite | PostgreSQL/TimescaleDB (1 sn / 1 dk)
                                           │
                        AI analiz ◄────────┤
                        Rapor (HTML/PDF) ◄─┤
                        Uyarı kuralları ◄──┘ (canlı Snapshot üzerinden)
```
