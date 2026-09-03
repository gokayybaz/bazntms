# Mimari

bazNTMS, tek Go binary'si içinde çalışan bir monolittir: yakalama motoru,
SQLite/PostgreSQL collector, uyarı motoru, AI istemcisi, rapor üreticisi ve
HTTP/WS sunucusu aynı süreçte yaşar. Frontend derlenip binary'ye gömülür
(`go:embed all:frontend/dist`). Faz 4 ile depo katmanı `store.Store`
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

Her olay `kind|key` başına cooldown (varsayılan 10 dk) tabi tutulur; geçenler
`alert_events`'e yazılır ve `Notifier` ile (masaüstü, Telegram, Discord, Slack,
generic webhook — hepsi asenkron, hatalar yutulur) dağıtılır.

## AI istemcisi (`internal/ai`)

OpenAI-uyumlu `/chat/completions` çağrıları; iki mod:

- **Tek seferde**: tüm veri tek JSON olarak gider
- **Chunked**: 4 veri bölümü ayrı isteklerle → her birinden kısa not → final
  istekte yalnızca notlar birleştirilir. Ham veri ikinci kez gitmez.

Reasoning modelleri için: `message.reasoning_content` / `reasoning` fallback,
`<think>` bloklarının temizlenmesi, `finish_reason=length` için açıklayıcı hata,
`/no_think` (Qwen3) ve `-llm-max-tokens` override.

## Sunucu (`internal/server`)

- `ServeMux` (Go 1.22+ metot kalıpları) + `logRequest` middleware
- `auth.middleware`: `/api/*` ve `/ws` oturum denetimi; `/api/login` ve
  `/api/auth/status` muaf; statik dosyalar açık (SPA kabuğu). Sabit zamanlı
  şifre karşılaştırma, IP bazlı deneme sınırı, HttpOnly cookie + Bearer token
- `Hub`: saniyede bir `tick` yayınlar (Snapshot + GeoIP zenginleştirme +
  bağlantılar + uyarılar + kayıt durumu); boş odaya üretilmez
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

Vite + React + Tailwind v4 + `react-router-dom`. `useLive` hook'u WS'i
birincil, 2 saniyelik REST yoklamasını yedek kaynak yapar (WS koparsa
otomatik dönüş). 401 görülürse App login ekranına düşer; 60 sn'de bir de
oturum denetimi yapılır. Grafikler (ThroughputChart, CompareCard, DayBars)
harici grafik kütüphanesi olmadan, elle yazılmış SVG'dir.

Dashboard'daki **`TrafficFlowDiagram`** de aynı yaklaşımla elle yazılmış SVG
bir sahnedir: sol sütunda **agent filosunun HER üyesi ayrı bir istemci düğümü**
(monitör ikonu + online/offline + en yoğun arayüz hızı), ortada Router/Güvenlik
Duvarı, sağda İnternet. Her canlı akış/agent/syslog olayı, olayı üreten agent'ın
düğümünden yön (giden/gelen/yerel/olay) bazlı animasyonlu bir "paket" geçirir;
NetFlow olayları (agent'sız) doğrudan güvenlik duvarı ↔ internet ekseninde akar.
viewBox yüksekliği agent sayısıyla büyür, düğüm detayı (tam/kompakt/mini) filo
kalabalıklaştıkça düşer — tüm agent'lar her zaman görünür. Paketler
`packetsRef`'te tutulur, tek bir `requestAnimationFrame` döngüsü boştayken
sessiz kalıp yalnızca hareket varken yeniden çizdirir; yön sınıflandırması
(`from`/`to` özel-genel IP ekseni) `lib/traffic.ts`'te, `TrafficFlowDiagram.test.tsx`
+ `lib/traffic.test.ts` ile kaplı. `prefers-reduced-motion` altında animasyon durur.

### Sayfa yapısı (routing)

`App.tsx` bir kabuk: sol sabit `Sidebar` (rota listesi) + üst `Header` (WS
durumu, kullanıcı kimliği, çıkış — hub'ın kendi yerel yakalamasıyla ilgili
hiçbir kontrol barındırmaz, bkz. aşağıdaki not) + `<Routes>` içinde sayfa
gövdesi. Backend zaten SPA history-fallback sağladığı için (`server.go`:
bilinmeyen path → `index.html`) istemci tarafı routing ek backend desteği
gerektirmeden çalışır.

| Rota | Sayfa | Veri kaynağı |
|------|-------|--------------|
| `/` | Dashboard (`Overview` bileşeni) | agent/cihaz/flow/syslog özet — kendi polling'i |
| `/agentlar`, `/agentlar/:id` | Agent listesi + derin detay | `GET /api/v1/agents[/…][/history]` |
| `/cihazlar`, `/cihazlar/:id` | Cihaz listesi + derin detay | `GET /api/v1/devices[/…]`, FortiGate için `FortiPanel` |
| `/topoloji` | Ağ topolojisi (SVG, hub+spoke) | `GET /api/v1/topology` |
| `/uyarilar` | Olay akışı + eşik/bildirim ayarları | `alertEvents` (WS) + `GET/PUT /api/alerts` |
| `/raporlar` | Ağ trafiği + kurumsal (SLA/kapasite) + uyumluluk raporları | `GET /api/report?type=…` |
| `/uyumluluk`, `/uyumluluk/{risk,soa,politikalar,denetimler,yonetisim}` | 5651 + ISO 27001 ISMS | `GET/POST/PUT /api/v1/isms/*`, paylaşılan tip/yardımcılar `lib/isms.tsx`'te |
| `*` | 404 | — |

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
boştur). Filo ham tablolarında continuous aggregate yok → pratik rapor
penceresi retention süresiyle (varsayılan 7 gün) sınırlı; 30/90 gün seçilse
bile sorgu var olan veriyi tarar.

## Süreç Bazlı Trafik Atfı (Faz 2)

`pkg/proctraffic` + `internal/agent/attr.go`: **nethogs yöntemi** — agent pcap
ile paket yakalar (yalnızca başlıklar, snaplen 600), 4'lü çifti donemlik
soket-tablosu → PID eşlemesiyle süreçe çevirir ve delta üretir.

**L7 uygulama görünürlüğü** (`internal/agent/l7.go`): aynı yakalama akışında,
giden TCP payload'ında TLS ClientHello **SNI**'si (`server_name` uzantısı) ve
HTTP istek satırındaki **Host** başlığı çıkarılıp sürece atfedilir → `l7`
telemetri alanı → `l7_endpoints` tablosu → `GET /api/v1/l7`. snaplen 128 →
600'e çıkarıldı (ClientHello 128'i aşar). İmza tabanlı DPI değil — yalnızca
açıkça görünen alan adı.

| Platform | Soket→PID kaynağı |
|----------|-------------------|
| Linux    | `/proc/net/*` inode ↔ `/proc/[pid]/fd` (root: tüm süreçler) |
| macOS    | `lsof -F pcnt` (root: tüm süreçler) |
| Windows  | `netstat -ano` + gopsutil süreç adları |

Hub politikası (`-agent-pcap`) + agent isteği (`-pcap`) ikisi de açıkken çalışır;
izin yoksa atıf devre dışı kalır, temel telemetri aksamaz. Ham PCAP kaydı
(`-record`) da aynı politika kapısıyla agent'ta yerel olarak döner.
İleri faz: eBPF (Linux) ve ETW (Windows) sağlayıcılar aynı arayüzün arkasına
eklenerek daha hassas sayım sağlanır.

## Veri akışı özeti

```
paket ─► process() ─► memory aggregates ─► WS tick (1 sn) ─► UI
                       └► Collector ─► Store: SQLite | PostgreSQL/TimescaleDB (1 sn / 1 dk)
                                           │
                        AI analiz ◄────────┤
                        Rapor (HTML/PDF) ◄─┤
                        Uyarı kuralları ◄──┘ (canlı Snapshot üzerinden)
```
