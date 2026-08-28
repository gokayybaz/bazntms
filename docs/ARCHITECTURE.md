# Mimari

bazNTMS, tek Go binary'si içinde çalışan bir monolittir: yakalama motoru,
SQLite collector, uyarı motoru, AI istemcisi, rapor üreticisi ve HTTP/WS
sunucusu aynı süreçte yaşar. Frontend derlenip binary'ye gömülür
(`go:embed all:frontend/dist`).

```
main.go
  ├─ store.Open()            SQLite aç + migrasyon
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

## Frontend (`frontend/`)

Vite + React + Tailwind v4. `useLive` hook'u WS'i birincil, 2 saniyelik REST
yoklamasını yedek kaynak yapar (WS koparsa otomatik dönüş). 401 görülürse App
login ekranına düşer; 60 sn'de bir de oturum denetimi yapılır. Grafikler
(ThroughputChart, CompareCard, DayBars) harici grafik kütüphanesi olmadan,
elle yazılmış SVG'dir.

## Veri akışı özeti

```
paket ─► process() ─► memory aggregates ─► WS tick (1 sn) ─► UI
                       └► Collector ─► SQLite (1 sn / 1 dk)
                                           │
                        AI analiz ◄────────┤
                        Rapor (HTML/PDF) ◄─┤
                        Uyarı kuralları ◄──┘ (canlı Snapshot üzerinden)
```
