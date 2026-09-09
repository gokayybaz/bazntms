---
title: Değişiklik Günlüğü
sidebar_position: 17
custom_edit_url: https://github.com/gokayybaz/bazntms/edit/main/CHANGELOG.md
---

> Kaynak: [`CHANGELOG.md`](https://github.com/gokayybaz/bazntms/blob/main/CHANGELOG.md) — bu sayfa her build'de otomatik senkronize edilir.
# Değişiklik Günlüğü

Bu projedeki dikkate değer değişiklikler burada tutulur. Biçim
[Keep a Changelog](https://keepachangelog.com/tr/1.1.0/) temellidir; sürümleme
[SemVer](https://semver.org/lang/tr/) — **v1.0.0'dan itibaren** kırıcı
`/api/v1` / protokol değişikliği major, geriye uyumlu özellik minor, düzeltme
patch (bkz. [`docs/decisions/0008-v1-scope.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/decisions/0008-v1-scope.md)).
Her GitHub sürümü ayrıca `--generate-notes` ile üretilmiş tam commit listesi
taşır — bu dosya **operatörün önemsediği** başlıkları ve **kırıcı / yükseltme**
notlarını özetler.

Kanallar: `agents.uplink_device_id` gibi şema değişiklikleri hub açılışında
otomatik migrasyonla uygulanır (`internal/store/migrations/`), geri alma yoktur
— yükseltmeden önce yedek alın (bkz. [`docs/UPGRADE-RUNBOOK.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/UPGRADE-RUNBOOK.md)).

## [1.3.0] — 2026-09-08

Faz 26 — **AI analiz**. Monolit döneminde (`d92d0fb:internal/ai`) vardı,
`9d22e7a`'da silinmişti; geri getirilip çoklu-sağlayıcı + kalıcı sohbet +
otomatik analiz modeline yükseltildi. **Opt-in** (`-ai`) → geriye uyumlu,
**minor**. `ProtocolVersion` 1'de kalır. Migrasyon `0021` (`ai_providers`,
`ai_conversations`, `ai_messages`) hub açılışında otomatik uygulanır.

Ayrıca — **agent derin toplama & L7 görünürlüğü tüm kurulumlarda varsayılan
açık** hale getirildi (ayrı iş kolu). Davranış değişikliği ama yeni bayrak /
uç kaldırılmadı → **minor**; aşağıdaki yükseltme notuna bakın.

### ⚠ Yükseltme notu — derin toplama varsayılan açık
- **Agent**: süreç trafiği + DNS + L7/SNI atıf motoru artık **varsayılan
  çalışır** — `collect.pcap: true` veya `-pcap` gerekmez (yok sayılır ama
  kabul edilir). Kapatmanın tek yolu `collect.method: off`. `collect.pcap:
  false` yazan mevcut kurulumlar yükseltmeden sonra **derin toplamayı açar**;
  istemiyorsanız `collect.method: off` yapın.
- **Hub**: `-agent-pcap` politikası artık **varsayılan `true`**. Filo
  genelinde derin toplamayı kapatmak için hub'ı `-agent-pcap=false` ile
  başlatın (veya hub.yaml'de `agent_pcap: false`).
- **Windows**: MSI kurulumu artık **Npcap'i sessizce indirip kurar**
  (`npcap.com`, SHA-256 + Authenticode doğrulamalı) ve agent `collect.method:
  pcap` ile gelir → L7/SNI Windows'ta da çalışır. Npcap indirilemezse kurulum
  yine başarılı biter, agent ETW'ye düşer (süreç trafiği + DNS akar, L7 akmaz).
  İnternet erişimi olmayan Windows ana makineleri için Npcap'i önceden kurun.

### Eklendi — Faz 26
- **AI sohbet sekmesi** (`/ai`) — çok-turlu, SSE streaming, mini-markdown
  render, sunucu-tanımlı preset butonlar ("Filoyu özetle", "Güvenlik
  taraması", "Anomali yorumu" …). `yetki:analyze` (viewer göremez).
- **Çoklu sağlayıcı** — yerel (Ollama, LM Studio) + bulut (OpenAI, Anthropic
  native, OpenAI-uyumlu: vLLM/OpenRouter/DeepSeek/Groq). Panel: **Yönetim >
  AI Sağlayıcı** (`yetki:global-admin`) — ekle/düzenle + "Test Et" + canlı
  model listesi. API anahtarı vault-şifreli, panelde bir daha gösterilmez.
- **Sayfa-farkında "AI'ya Sor"** — Agent / Cihaz / Anomali / Olay detay
  sayfalarından ilgili bağlamla sohbet açar. IncidentDetailPage'de otomatik
  "AI Triyaj" notu paneli.
- **Otomatik analiz**: (1) preset butonlar, (2) gecelik filo analizi
  (`ai.nightly` — `internal/aijob` scheduler işi, lider-kapılı, opsiyonel
  e-posta), (3) olay-tetikli triyaj (`ai.triage` — yeni kritik incident →
  triyaj notu, saatlik hız-sınırlı).
- **AI danışmandır** — araç çağırmaz, durum değiştirmez; deterministik
  motorlar (anomali, incident, health, `recommend`) yetkili kalır (ADR 0014).

### Güvenlik — Faz 26
- **Egress kilidi** `-ai-allow-cloud=false` → yalnız loopback/RFC1918 model
  adresleri (kayıt + çalışma anı). Self-hosted / hava boşluklu kurulum: veri
  ağdan çıkmaz.
- Prompt injection sınırı: telemetri verisi "güvenilmez gözlem" olarak
  işaretli, çıktı otomatik aksiyona bağlanmaz.
- `ai.provider.*` + `ai.analyze` denetim zincirine (`api_key` maskeli).

### Geriye uyum — Faz 26
- `-llm-base-url` / `-llm-api-key` / `-llm-model` + `LLM_*` / `OPENAI_*`
  env korunur — `ai_providers` boşsa ilk açılışta bir `bootstrap` sağlayıcı
  seed eder. `-llm-max-tokens` / `-llm-no-think` bayrakları kaldırıldı
  (sağlayıcı `opts`'una taşındı).

### Değişti — agent derin toplama & L7 (tüm kurulum yolları)
- **`collect.method` varsayılanı `auto`** (config yoksa da). `pcapWant` artık
  yalnızca `method: off` iken kapanır; süreç/DNS/L7 panelleri kutudan çıktığı
  gibi dolar. `cmd/bazntms-agent` + `internal/config`.
- **Hub `-agent-pcap` varsayılanı `true`** + yeni `agent_pcap` hub.yaml /
  Helm config anahtarı (kapatmak için `false`).
- **Windows MSI**: `deploy/msi/install-npcap.ps1` — kurulumda Npcap sessiz
  kurulur (deferred/SYSTEM CustomAction, SHA-256 `npcap-1.88` + Authenticode
  "Nmap Software LLC" doğrulaması, `exit 0` garantili → MSI'ı asla düşürmez).
  Windows seed config'i ayrıldı: `deploy/config/bazntms-agent.windows.yml`
  (`method: pcap`).
- **Enroll sihirbazı** (`agentInstall.ts`) seed YAML: `collect.pcap: true` →
  `collect.method: auto` + açıklayıcı yorum.
- **Helm**: DaemonSet `agent.pcap` (varsayılan `true`) artık `NET_RAW` +
  `NET_ADMIN` capability ekliyor; `values.config.agent_pcap` → configmap.
- **deb/rpm/macOS** postinstall + `bazntms-agent.yml.example`: zaten
  `pcap: true` idi, açıklamalar güncellendi (yeni varsayılana atıf).

## [1.2.0] — 2026-09-08

Faz 23 — **Gözlemlenebilirlik derinliği** · Faz 24 — **Tespit & korelasyon** ·
Faz 25 — **Operasyonel olgunluk**. Tümü geriye uyumlu (yeni uç / tablo / alan)
→ **minor**. `ProtocolVersion` 1'de kalır. Migrasyonlar `0015`–`0020` hub
açılışında otomatik uygulanır.

### ⚠ Yükseltme notu — Faz 25-D
- `POST /api/v1/enroll-tokens` gövdesinde `expires_in_days` **atlanmış / 0**
  artık **1 gün** demek (eskiden: süresiz). Süresiz token için `-1` gönderin.
  Yeni token'lar ayrıca varsayılan **tek kullanımlık** (`max_uses` 1). Mevcut
  DB token'ları migrasyonla `max_uses=0` (sınırsız) alır — davranış değişmez.

### Eklendi — Faz 25 (operasyonel olgunluk)
- **Ağ sağlık skoru** (25-A). `internal/health` — 0-100 **deterministik
  ağırlıklı**, her kesinti açıklanabilir (agent offline / bayat telemetri /
  cihaz offline / kritik uyarı / açık olay riski / arayüz hata+iskarta).
  **Opak AI skoru değil.** `GET /api/v1/health` (~30 sn önbellek); panoda
  `Ağ Sağlığı` kartı + kurumsal raporda bölüm.
- **Kurumsal rapor v2** (25-B). `/api/report?type=enterprise` — yönetici özeti
  (KPI ızgarası), ağ sağlık skoru, top konuşmalar (NetFlow), DNS / uygulama
  görünürlüğü, açık olaylar (incident), **öneriler**. Öneriler `recommend()` —
  8 eşik-tabanlı **deterministik şablon**, LLM yok; her madde bir metriğe
  bağlı. Ek bölümler best-effort (kaynak eksik → "veri yok", 500 yok).
- **Denetim kaydı v2** (25-C). `audit_events` + `actor_type` / `request_id`
  (log korelasyonu) / `user_agent` / `result` (ok/error/denied) ve
  yapılandırma değişikliklerinde `before_json` / `after_json` durum farkı
  (sır alanları `•••` maskeli). Hash zinciri stabil — v2 segmenti yalnız bir
  v2 alanı doluyken katılır, eski kayıtlar aynen doğrulanır (ADR 0012).
  `GET /api/v1/audit` süzgeçli (actor/action/resource/ip/result/tarih);
  `AuditCard` süzgeç barı + öncesi/sonrası paneli. Migrasyon `0019`.
  Ayrıca: `InsertAuditEvent` **ve** `AppendComplianceLog` (5651 log zinciri)
  çoklu-replika (HA) yazımında pg advisory-lock + tek-transaction ile
  serileştirildi (2× hub-controller hash zincirini çatallıyordu).
- **Enrollment token sertleştirme** (25-D). `enroll_tokens` + `max_uses`
  (atomik `ConsumeEnrollToken` — eş zamanlı agent'lar son slotu paylaşamaz),
  `allowed_cidrs` (kaynak-IP kısıtı — soket peer'ine göre), `created_by`,
  `revoked_at`. `handleAgentHello` ayrık 4xx: 401 geçersiz/iptal/süre, 403
  CIDR, 409 max_uses. `enroll_token.used` denetlenir. Migrasyon `0020`.
  ADR 0013. **Bkz. yükseltme notu.**
- **UI tutarlılık** (25-E). `PanelState` bileşeni (yükleniyor/boş/hata bandı —
  ~70 elle varyant tekilleştirildi). `TuiTable` seçili satır klavye odağında
  htop tarzı belirgin imleç. `HelpOverlay` güncellendi.

### Eklendi — Faz 24 (tespit & korelasyon)
- **Normalleştirilmiş olay akışı** (24-A). `GET /api/v1/events` — ham gözlemler
  (dns.query / tls.sni_observed / http.host_observed / netflow.flow /
  syslog.received / connection.seen) uyarılardan ayrı bir okuma modelinde
  (mevcut kaynak tablolar UNION ALL — yeni yazma hattı yok, ADR 0010).
  `/uyarilar` → `Olay Akışı` sekmesi.
- **Olay (incident) korelasyon motoru** (24-B/C). `internal/incident` —
  lider-kapılı, 5 deterministik kural (yeni-süreç + yeni-hedef/ioc /
  şüpheli-port / hedef+bant / anomali+bant / ≥N-şüpheli), risk skoru 0-100
  açıklanabilir, dedup = correlation_key. **AI/LLM yok** (ADR 0011). Migrasyon
  `0018` (`alert_events.agent_id` + `incidents` + `incident_evidence`).
  `GET/POST /api/v1/incidents[/:id][/ack|investigate|resolve|close]` (denetimli).
  `/uyarilar` → `Olaylar` sekmesi + `/uyarilar/olay/:id` detay (kanıt zaman
  çizelgesi).
- **Anomali metriği `l7_qps`** (24-D). TLS SNI / HTTP Host gözlem hızı —
  endpoint-temas sıçraması ≈ alışılmadık / yeni hedef aktivitesi. `/anomali`
  metrik seçicisine eklendi.
- **Tehdit istihbaratı adaptörü** (24-E). Sağlayıcı-bağımsız
  `internal/threatintel` — `Provider` arayüzü, IP + domain, itibar enum'u
  (trusted…malicious), TTL önbellek. `-ioc-file` artık `localfile` sağlayıcısı
  (domain + **IP** kara listesi). `GET /api/v1/threatintel?ip=&domain=`.
  Süreç detayı hedefleri + akış drill-down'ında itibar pill'i. **Oto-blok yok**
  — observability-first. `ioc` uyarısı itibar/kaynak taşır.

### Eklendi — Faz 23 (gözlemlenebilirlik derinliği)
- **Süreç detayı & derin inceleme** (23-A). Agent → Süreç Trafiği → `Enter` →
  tek bir sürecin (ad bazlı) tüm ağ etkinliği tek ekranda: kimlik/özet, uzak
  hedefler (ip:port/proto toplama + GeoIP/ASN), canlı bağlantılar, uygulama
  görünürlüğü (DNS + TLS SNI + HTTP Host), zaman çizelgesi (ilk temas / DNS-L7
  ilk görülme / trafik sıçraması / ilişkili uyarılar). `GET /api/v1/agents/:id/
  processes/:ad` — yeni telemetri hattı yok, mevcut tablolar sunucu-tarafı
  toplanır. Rota `/agentlar/:id/surec/:ad`.
- **NetFlow konuşma toplama** (23-B). Ham `flows` → 5'li / uç-çifti konuşmalar
  (`GET /api/v1/flows/conversations`), pencere 15dk–24s, sıralama
  bytes/packets/flows/last-seen. Drill-down (`GET /api/v1/flows/conversation`):
  ham akışlar + uç GeoIP/ASN + `process_traffic` ile ilişkili agent/süreç.
  `/cihazlar` altında `Top Konuşmalar` (ham NetFlow görünümü değişmedi).
  Migrasyon `0015` (`idx_flows_convo`, `idx_flows_pair`).
- **Arayüz kapasitesi & kullanım** (23-C). SNMP `ifType` (IANAifType) +
  `ifHighSpeed` toplanır; `class` (ethernet/wifi/loopback/tunnel/vpn/bridge/
  vlan/ppp/unknown) + `rx/tx_util_pct` (yalnız güvenilir hız + oper=up).
  `iface_util` uyarısı: `warn_pct` (70) / `crit_pct` (90) eşiği `sustain_sec`
  (300) boyunca aşılırsa; loopback/tünel atlanır (`iface` config bölümü).
  `LatestDeviceIfaces` sayaç-geri-gitme koruması (SNMP restart/wrap → sahte
  sıçrama yok). Migrasyon `0016`. `DeviceDetailPage` TÜR / UTİL kolonları.
- **Topoloji canlı bağlantı telemetrisi** (23-D). SNMP-destekli kenarlara
  (`local_port` ↔ ifName) canlı arayüz telemetrisi bağlanır → görsel durum
  (normal/uyarı≥%70/kritik≥%90/down), `Enter`/tık → link inspector, güven
  düzeyi rozeti (`confidence`: discovered/inferred/manual). Migrasyon `0017`.
- **Hedef zenginleştirme** (23-E). Paylaşılan `internal/enrich` servisi: uzak IP
  → ülke/ASN/org + özel/genel (RFC1918→`YEREL`); alan → normalize + kayıtlı-alan
  (eTLD+1) + kategori (`-domain-category-file`). `GET /api/v1/enrich?ip=&domain=`.
  Süreç detayı hedefleri + konuşma drill-down + harita aynı servisi kullanır;
  frontend ortak `lib/enrich.tsx`.

## [1.1.0] — 2026-09-08

Faz 22 — **İleri Analiz & operasyonel derinlik**. `docs/enterprise-plan.html` →
"İleri Analiz ve Kurumsal Özellikler" başlığı: anomali motoru site/cihaz bazına
çıktı, uyarılara yaşam döngüsü geldi, bilet sistemlerine bağlandı, kurumsal
raporlar zamanlanabilir oldu. Tüm değişiklikler geriye uyumlu (yeni tablo / uç /
alan) → **minor** (ADR 0008). `ProtocolVersion` 1'de kalır.

### Eklendi
- **Anomali motoru v2.** Saat-of-day z-skoru baseline'ı → mevsimsel (hafta
  içi/sonu × saat, opsiyonel gün-of-week) + EWMA gün ağırlığı (yavaş drift'e
  uyum). Materyalize `anomaly_baseline` tablosu + lider-kapılı saatlik rebuild
  → değerlendirme başına canlı `LAG` taraması yok. **Çok boyutlu:** filo / saha /
  agent; **çok metrikli:** arayüz bant genişliği + DNS sorgu hızı + süreç
  trafiği (tünelleme / DGA / sızdırma erken sinyali). `|z| ≥ crit_z` → "crit".
  Yeni `/anomali` sayfası: elle-SVG "beklenen ±2σ bandı vs gerçek" grafiği +
  aktif sapmalar tablosu. `GET /api/v1/anomaly/{baseline,active}`.
- **Uyarı yaşam döngüsü.** `alert_events` artık severity / state
  (firing → ack → resolved) / site / tekrar sayacı / korelasyon grubu taşır.
  Aynı koşul tekrar ateşlenirse yeni satır değil `count++` (dedup). Otomatik
  çözülme: koşul `auto_resolve_min` dakika yinelenmezse kapanır (anomali için
  koşul-tabanlı da). Aynı sahada `correlate_window_sec` içinde ateşlenen
  olaylar ortak `group_id`. **Bakım pencereleri** (`alert_silences`) — kind /
  site / anahtar eşleşmesi + zaman aralığı; eşleşen uyarı bildirilmez.
  Filtreli + sayfalı `GET /api/v1/alerts/events` + `POST .../:id/{ack,resolve,
  note}`. Yeniden yazılmış Uyarılar sayfası (filtre çubuğu, işlem dialogu).
- **Bildirim yönlendirme + bilet sistemleri.** `notify_routes` — matcher
  (severity / kind / site) → kanal listesi + `continue`; kural yoksa mevcut
  "etkinlerin hepsine" davranışı. **Jira Cloud** (REST v3) + **ServiceNow**
  (Table API `incident`): uyarı grubu ilk ateşlendiğinde issue/incident açılır
  (`ext_ref`), grup çözüldüğünde geçiş yapar. API token / parola kimlik
  kasasında şifreli. (PagerDuty kanal id'si tanınır, gönderim ileride.)
- **Zamanlanmış kurumsal raporlar + SLA.** `internal/scheduler` — hub-içi
  lider-kapılı çalıştırıcı (harici bağımlılık yok; `daily:HH:MM` /
  `weekly:<gün>:HH:MM` / `monthly:<n>:HH:MM` / `interval:<dk>`; kaçırılan koşu
  bir kez telafi edilir). Rapor teslim hattı: üretilen rapor
  `<data>/reports/`'a yazılır + arşivlenir + (alıcı varsa) e-postalanır.
  **Enterprise rapor PDF** + `?site=` saha kırılımı (site-admin artık kendi
  kurumsal + uyumluluk raporunu çekebilir). **SLA hedefleri** (`sla_targets`,
  global + saha) — kurumsal raporda hedef-vs-gerçek + ihlal vurgusu; ihlalde
  `sla_breach` uyarısı (crit). Raporlar sayfasına zamanlama editörü + arşiv +
  SLA hedef ayarları. `GET/POST/DELETE /api/v1/reports/*`, `/api/v1/sla/targets`.

### Şema

`0009`–`0014` migrasyonları hub açılışında otomatik uygulanır (sqlite +
postgres): `anomaly_baseline`, `alert_events` yaşam döngüsü sütunları,
`alert_silences`, `scheduled_jobs`, `report_archive`, `sla_targets`. Geri alma
yok — yükseltmeden önce yedek.

### Düzeltildi
- **Agent 401 kendini-onarma sayacı restart'ta sıfırlanıyordu.** Hub veritabanı
  yeniden yaratıldığında (ya da kayıt silindiğinde) kayıtlı agent token'ı kalıcı
  401 döner; agent 3 ardışık 401'den sonra enroll token'ıyla yeniden kaydolur.
  Bu sayaç bellekte tutulduğu için sık yeniden başlayan (crash-loop, launchd
  `KeepAlive`, art arda kurulum) bir agent eşiğe hiç ulaşamıyor ve sonsuza dek
  401 atıyordu. Sayaç artık `bazntms-agent.state.json` içinde tutuluyor
  (`auth_fail_streak`) — restart'ları aşar, başarılı telemetride / yeniden
  enroll'de sıfırlanır.

## [1.0.0] — 2026-09-08

Faz 21 — **v1.0 sertleştirme + ölçek doğrulama**. Kurumsal kapasite hedefleri
(`docs/enterprise-plan.html`) sentetik yükle ölçüldü, bulunan darboğazlar
düzeltildi, sürekli operasyon için sertleştirildi. Tam rapor:
[`docs/CAPACITY.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/CAPACITY.md).

> **Doğrulanan ölçek** (tek-node `deploy/docker-compose.scale.yml`, `target`
> profili): 5.000 agent @ 30 sn (167 ist/sn, batch p95 6 ms, hata %0) +
> 50.000 flow/sn sürekli (`flows_dropped_total` = 0, flow yazım p95 1.1 ms) +
> 1.000 mock cihaz (poll döngüsü 0,21 sn). Panel sorgusu p95 < 200 ms.
> 200.000 flow/sn patlaması tek-node laptop yığınında **donanım-bağlı** —
> flow verisi kayıpsız (NATS buffer'lar + boşalır), agent telemetrisi geçici
> degrade eder ve otomatik kurtarır; üretim boyutlandırması `docs/CAPACITY.md`.

### Eklendi
- **`bazntms-loadgen` — flow + cihaz modları.** Agent moduna ek olarak sentetik
  NetFlow v5/v9 + IPFIX + sFlow üreteci (token-kova hız kontrolü, patlama
  desteği) ve mock SNMP cihaz filosu (`internal/driver` `mock` sürücüsü, ağ
  I/O'suz monoton sayaçlar). Kova-interpolasyonlu quantile + hata sınıflama +
  `-out` / `-warmup`.
- **`internal/metrics` paketi** — ingest hattı için çapraz-kesen Prometheus
  metrikleri: store yazım süresi/satır sayısı (tablo etiketli), telemetri
  çözümleme süresi, kuyruk bekleyen/batch, flow alınan/düşürülen (sebep
  etiketli), devpoll döngü/inflight, DB bağlantı havuzu (6 gauge/counter).
  `/metrics` bu kaydı ana kayıtla birleştirir.
- **Yük testi koşum takımı** — `scripts/loadtest.sh <profil>` (birleşik
  senaryo), `scripts/perf_summary.py` (Prometheus çok-replika toplamı +
  eşik kontrolü + markdown rapor), `scripts/profile.sh` (pprof toplama +
  diff), `scripts/query_bench.sh` (panel sorgu p95), `scripts/chaos.sh`
  (4 kaos senaryosu), `scripts/soak.sh` (gece sızıntı koşusu, medyan
  bant analizi). Profiller: `loadtest/profiles/{baseline,target,burst}.env`.
- **Grafana "bazNTMS — Kapasite" panosu** + Prometheus (`--profile obs`) —
  tüm hub replikalarını `dns_sd` ile toplar (nginx LB arkasında `/metrics`
  tek replika görür).
- **Agent dayanıklılığı** — offline kuyruk taşma sayacı (`DroppedBatches()`
  + "offline kuyruk dolu" logu), gönderme hatasında üstel backoff + jitter
  (taban × 2^min(fails,3), maks 5 dk), telemetride saat kayması clamp'i
  (`ClampTS` — `[now-7g, now+1g]` dışı → `now`), protokol sürümü graceful
  degrade (hub `maxProtocolVersion`'a düşürür, agent reply'den öğrenir).
- **`docs/CAPACITY.md`** — kapasite doğrulama raporu (hedefler × ölçülen,
  darboğazlar + düzeltmeler, önerilen üretim boyutlandırması, yeniden
  çalıştırma adımları).

### Değiştirildi
- **Toplu yazım** — `internal/store/bulk.go`: PostgreSQL'de satır-başına
  `stmt.Exec` (round-trip/satır) yerine chunk başına tek çok-satırlı
  `INSERT ... VALUES (…),(…)` (`pgMaxParams` 60000'de böler). SQLite yolu
  değişmedi. `SaveIfaceSamples` / `ReplaceConnLatest` / `SaveFlows` /
  `SaveL7*` / `SaveDNS` / `SaveProcessTraffic` / `SaveDeviceIfaceSamples`
  bu yolu kullanır.
- **`/api/v1/agents`** — agent başına ayrı rate + conn sorgusu (N+1, ~1 sn)
  → tüm filo için tek `ROW_NUMBER() OVER (PARTITION BY agent_id ORDER BY
  ts DESC)` sorgusu + tek `GROUP BY agent_id`. p95 1049 → 152 ms.
- **`/api/v1/flows`** — üst-N taraması `idx_flows_octets` (`0008`) kullanır,
  pencere üst sınırı 6 saat. p95 2369 → 16 ms.
- **Flow collector** — tek okuyucu + senkron `OnFlows` yerine reader/worker
  hattı: 1 okuyucu + N worker (`-`, varsayılan 6), havuzlanmış `*pkt`,
  `SO_RCVBUF` 8 MiB, `TemplateCache` `sync.RWMutex`. 50k flow/sn drop 0.
- **NATS flow yolu** — `js.PublishAsync` (bounded pending 8192) + store-writer
  worker'ında Fetch içinde flow mesajı birleştirme (~150 msg / `SaveFlows`).
  200k patlamada collector düşürmesi 127k → ~0.
- **`devpoll.pollAll`** — cihaz başına sınırsız goroutine yerine bounded
  semafor (`-devpoll-concurrency`, varsayılan 96; `p.stop` ile iptal edilir).
- **Rapor / geo uçları** — uzun pencerede ham `flows` UNION+GROUP BY (60–90 sn
  timeout) yerine continuous aggregate okur.
- **DB bağlantı havuzu** — `BAZNTMS_DB_MAX_CONNS` env (varsayılan 32, scale
  compose'ta 64); `SetMaxIdleConns(maxConns/4+1)`. `devpoll-concurrency`
  havuzu aşmamalı.
- **`cmd/bazntms-hub`** yeni bayraklar: `-queue-workers` (store-writer
  paralel worker, varsayılan 4), `-devpoll-concurrency` (96), `-pprof-rates`
  (block/mutex profil oranı), `-mock-devices` (yalnız yük testi).
- **`deploy/docker-compose.scale.yml`** — hub replikalarında
  `BAZNTMS_DB_MAX_CONNS=64`, `GOMEMLIMIT=512MiB`, `-pprof`; hub-ingest
  `-queue-workers 8`; `prometheus` + `grafana` servisleri (`profiles: [obs]`).

### Şema
- **`0008_perf_indexes`** — `idx_flows_octets` (`flows (octets DESC)`),
  `/api/v1/flows` üst-N taraması için.
- **Yeni continuous aggregate'ler** (yalnız TimescaleDB) — `agent_iface_1h`,
  `process_traffic_1h`, `flows_dst_1h`, `flows_src_1h` (her biri cagg
  yenileme + saklama politikasıyla, 1–2 yıl). Rapor / geo / filo trafiği
  uçları uzun pencerede bunları okur.

### Kırıcı / yükseltme
- **Mevcut TimescaleDB kurulumlarında** yeni cagg'ler boş oluşturulur —
  yükseltmeden sonra geçmiş dönem raporları eksik görünür. Bir kez
  `CALL refresh_continuous_aggregate('<ad>', NULL, NULL)` ile geri doldurun
  (saklama penceresi kadar ham veri tarar, saatler sürebilir). Adımlar:
  [`docs/UPGRADE-RUNBOOK.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/UPGRADE-RUNBOOK.md).
- SQLite / TimescaleDB dışı PostgreSQL kurulumları etkilenmez (cagg'ler
  yalnız TimescaleDB'de kurulur).

### Karar kaydı
- [`docs/decisions/0008-v1-scope.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/decisions/0008-v1-scope.md) —
  v1.0 API / protokol kararlılık taahhüdü, SemVer sözleşmesi,
  `protocol_version` uyumluluk politikası, v1 kapsamı **dışında** bırakılanlar
  (ETW L7, macOS Endpoint Security, gerçek KMS zarf şifreleme, çok-kiracılılık).

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
- [`docs/decisions/0007-attr-backends.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/decisions/0007-attr-backends.md) —
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
  [`docs/TROUBLESHOOTING.md`](https://github.com/gokayybaz/bazntms/blob/main/docs/TROUBLESHOOTING.md) "Süreçler / DNS / L7
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

[Yayımlanmamış]: https://github.com/gokayybaz/bazntms/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/gokayybaz/bazntms/compare/v1.1.0...v1.3.0
[1.1.0]: https://github.com/gokayybaz/bazntms/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/gokayybaz/bazntms/compare/v0.4.0...v1.0.0
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
