# bazNTMS Faz 23–25 Planı

> Kaynak: `bazNTMS-next-milestone-development-plan.md` (dış plan — 15 "PHASE",
> 3 milestone). Bu belge o planı projenin faz/spike konvansiyonuna oturtur ve
> her spike'ı mevcut kod tabanındaki gerçek dosya/şema/uçlara bağlar.
> Temel: `a05542b` / v1.1.0. Sonraki migrasyon: **0015**.

Sonraki kilometre taşı üç faz:

| Faz | Milestone | Konu |
|-----|-----------|------|
| **23** | A — Observability Depth | Gözlemlenebilirlik derinliği |
| **24** | B — Detection & Correlation | Tespit & korelasyon |
| **25** | C — Operational Maturity | Operasyonel olgunluk |

Her faz 5 alt-faz (A–E); spike'lar `S{faz}.{n}` ile faz boyunca sürekli
numaralanır (Faz 22'deki `S22.1…S22.24` gibi). **Bir faz bitmeden diğeri
başlamaz; tek değişiklikte birden çok faz uygulanmaz** (dış plan WORKING RULES).

---

## 00 — Kod tabanının durumu

Dış plan daha eski bir anlık görüntüye göre yazılmış. Faz 11–22'de istenenlerin
bir kısmı **zaten yapıldı**. Her alt-fazın gerçek kapsamı buna göre daraltıldı.

Durum etiketleri: **mevcut** (temel atılmış, küçük ekleme) · **kısmi** (Faz 22'de
çekirdek var, genişletme) · **sıfırdan** (yeni tablo/paket/motor).

| Dış plan | Durum | Bugün ne var | Bu planda ne eklenecek |
|----------|-------|--------------|------------------------|
| **P1** Süreç detayı | sıfırdan | `process_traffic` tablosu `remote_ip/port/proto` tutuyor ama `TopProcessTraffic` yalnız `GROUP BY process`; süreç-detay ucu yok | Süreç kimliği + uzak hedef toplama + DNS/SNI korelasyonu + zaman çizelgesi; `ProcessDetailPage` |
| **P2** NetFlow konuşma toplama | sıfırdan | `flows` düz 5'li tablo; `TopFlows` = `ORDER BY octets`; TS'de `flows_1h` cagg. NetFlow **v5/v9/IPFIX/sFlow zaten var** (plan "v5" diyor) | 5'li / uç-çifti konuşma toplama, zaman pencereleri, drill-down |
| **P3** Arayüz kapasitesi | sıfırdan | `device_iface_samples`: speed, oper_status, err/discard; `DeviceIfaceRate` Rx/TxBps | Kullanım %, arayüz tipi sınıflandırma (ifType), yapılandırılabilir eşik + debounce uyarı |
| **P4** Topoloji + canlı bağlantı | sıfırdan | `topology_links` (LLDP/CDP/ARP/subnet), SVG `TopologyCard` | Kenar↔arayüz telemetri bağlama, kenar görsel durumları, güven düzeyi, inspector |
| **P5** Hedef zenginleştirme | kısmi | `internal/geoip.Resolver` (ülke/ASN, MMDB + ip-api, 100k LRU); `EndpointDelta` sorgu anında zenginleşiyor — yalnız harita ucunda | Paylaşılan `internal/enrich`: domain normalizasyon, kayıtlı-alan, kategori, RFC1918 sınıfı; her görünüme entegrasyon |
| **P6** Normalleştirilmiş olay + uyarı modeli | kısmi | **Uyarı tarafı Faz 22-B'de bitti:** severity/state/dedup/count/group_id, ACK/RESOLVE/not (_denetime yazılıyor_), bakım pencereleri, filtreli `/api/v1/alerts/events` | Yalnız normalleştirilmiş **EVENT** katmanı: mevcut telemetri tablolarının üzerinde birleşik okuma modeli (yeni yazma hattı **YOK**) |
| **P7** Olay korelasyon motoru | sıfırdan | Yalnız uyarı `group_id` (zaman+saha); incident kavramı yok | `incidents` + `incident_evidence` tabloları, `internal/incident` lider-kapılı motor, 5 deterministik kural, risk skoru |
| **P8** Olay yönetim UI | sıfırdan | — | `/uyarilar` altında `[ALARMLAR][OLAYLAR]` sekmesi, incident liste + detay + kanıt zaman çizelgesi |
| **P9** İstatistiksel baseline | kısmi | **Faz 22-A:** materyalize `anomaly_baseline` (Welford), mevsimsel (haftaiçi×saat), EWMA, boyutlar local/fleet/site/agent, metrikler bps/dns_qps/proc_bps, z-skoru önem, min-örnek eşiği, `/anomali` paneli | Eksik metrik/varlıklar: bağlantı sayısı, yeni-hedef oranı, arayüz kullanımı, PPS |
| **P10** Tehdit istihbaratı adaptörü | kısmi | `internal/ioc` domain kara listesi (hash-set, hot-reload) + `alert/ioc.go` "ioc" uyarısı | Sağlayıcı-bağımsız `Provider` arayüzü, IP göstergeleri, itibar enum'u (trusted…malicious), önbellek, UI'da kaynak/sağlayıcı |
| **P11** Ağ sağlık skoru | sıfırdan | — | `internal/health` saf fonksiyon: 0–100 ağırlıklı skor + açıklanabilir kesintiler; panoda + raporda |
| **P12** Raporlama v2 | kısmi | **Faz 22-D:** zamanlı/PDF/SLA/arşiv/saha-kırılımı, sağlık% + kapasite notu içeren kurumsal rapor | Tam 13-bölüm yapısı, Top Konuşmalar (P2), Top Hedefler (P5), Olaylar (P7), deterministik Öneri motoru |
| **P13** Denetim izi v2 | kısmi | `audit_events` (ts/user/role/action/target/detail/ip/prev_hash/hash), hash-zincir doğrulama, saha kapsamı (0004) | `actor_type, request_id, user_agent, result, before/after` diff + redaksiyon + filtre API |
| **P14** Kayıt sertleştirme | kısmi | `enroll_tokens` (name/site/expires_at/last_used/revoked), Faz 12 bootstrap-only, Faz 14B token-saha bağlama, hash'li saklama | `max_uses/used_count, allowed_cidrs, created_by, revoked_at`; varsayılan 1 gün / 1 kullanım; `token.*` denetim olayları |
| **P15** UI/UX cilası | mevcut | **Faz 17/18:** tam htop/TUI dönüşümü, semantik tokenler, Meter/TuiTable/Panel, klavye modeli | Yalnız tutarlılık denetimi: satır aralığı, seçili-satır, odak halkaları, boş/yükleniyor/hata durumları, klavye taraması |

### Plan zaten karşılanmış — tekrar yapma

- Terminal/NOC kimliği, klavye-öncelikli nav, üst navigasyon, yoğun düzen,
  semantik cyan/mor/yeşil/sarı/kırmızı → **Faz 17/18**
- Güvenli migrasyon çerçevesi (embed.FS + `schema_migrations` + Go-fn adımları)
  → **Faz 13**; `0001_init` donuk
- Uyarı ACK/RESOLVE + durum değişikliği denetime yazılıyor
  (`alert.ack/alert.resolve/alert.note`) → **Faz 22-B**
- Uyarı dedup / yaşam döngüsü / bastırma (silenced≈suppressed) → **Faz 22-B**
- Anomali: yuvarlanan ortalama/stddev/z / min-örnek / pencere → **Faz 22-A**
- Zamanlı rapor + PDF + lider-kapılı zamanlayıcı (`internal/scheduler`) → **Faz 22-D**
- Kayıt token'ı `expires_at` / saha / iptal → **Faz 12/14**

---

## 01 — Sıralama & bağımlılık

Dış planın "Recommended Execution Order"ı korunur: Milestone A → Faz 23,
B → Faz 24, C → Faz 25.

### Faz-ötesi bağımlılık zinciri

- **23-E (enrich)** → 23-A hedef satırları · 23-B drill-down · 24-E itibar
  birleşik render · 25-B Top Hedefler
- **23-B (konuşmalar)** → 25-B rapor "Top Conversations" bölümü
- **23-C (kullanım %)** → 23-D kenar telemetrisi · 24-D `iface_util` metriği ·
  25-A sağlık girdisi
- **24-B (incident)** → 24-C UI · 25-A "açık olay" girdisi · 25-B rapor
  "Incidents" bölümü
- **24-A (event modeli)** → 24-B kanıt kaynağı · 24-C olay akışı sekmesi

> **Kural:** Bir alt-faz atlanabilir ama bağımlı alt-faz beklemeli. 23-E'yi
> 23-A/23-B'den önce bitirmek en az yeniden-işi verir.

### Migrasyon numaralandırma

| Migrasyon | Faz | İçerik |
|-----------|-----|--------|
| `0015` | 23-B | `flows` konuşma toplama bileşik indeksleri |
| `0016` | 23-C | `device_iface_samples` + `if_type`, `high_speed` |
| `0017` | 23-D | `topology_links` + `confidence` |
| `0018` | 24-B | `incidents` + `incident_evidence` |
| `0019` | 25-C | `audit_events` v2 kolonları |
| `0020` | 25-D | `enroll_tokens` sertleştirme kolonları |

Her migrasyon hem `sqlite/` hem `postgres/` altına; dialect-koşullu DDL
gerekirse `migrate.go` `goMigrations`. CI `db-upgrade-test` kapsar.

### Her faz kapanışında (sabit ritüel)

1. `go test ./... && go vet ./... && gofmt -l . && (cd frontend && npm test && npm run build)`
2. `docker compose -f deploy/docker-compose.scale.yml up -d --build` + `:8080`
   smoke (frontend-only alt-fazda yalnız `hub-controller` + `hub-ingest`)
3. `git checkout -- web/dist/.gitkeep`
4. CHANGELOG sürüm girişi · ilgili `docs/*.md` + gerekiyorsa
   `docs/decisions/00NN-*.md` · memory `faz-NN-ilerleme.md`

Her spike, dış plan WORKING RULES'una uyar: **repo analizi → spike (kod yok)
→ en küçük temiz uygulama → test → doğrulama → dokümantasyon**.

---

# FAZ 23 — Gözlemlenebilirlik Derinliği

Operatörün "bu süreç / bu konuşma / bu link tam olarak ne yapıyor" sorusunu tek
ekranda yanıtlaması. **Yeni telemetri hattı yok** — mevcut `process_traffic /
agent_dns / l7_endpoints / flows / device_iface_samples` tabloları sunucu-tarafı
toplanır.

## 23-A — Süreç Detayı & Derin İnceleme · sıfırdan

Giriş: `Agent → Süreç Trafiği → Enter`. `AgentDetailPage.tsx` ve
`ProcessesCard.tsx` zemin; drill-down rotası yok. `TopProcessTraffic` uzak-uç
kolonlarını atıyor.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S23.1** | Süreç kimliği + özet toplama sorgusu — `MIN/MAX(ts)`→ilk/son görülme, DISTINCT pid, SUM bytes, son-iki-kova delta ile anlık hız. **dosya:** `internal/store/process_traffic.go`, `interfaces.go (AgentStore)`. **desen:** `LatestDeviceIfaces` first/last kova delta. **test:** `process_traffic_test.go` — totaller `TopProcessTraffic` ile mutabık |
| **S23.2** | Uzak hedef toplama: `ProcessRemotes(agentID,process,since,site)` — `GROUP BY (remote_ip,port,proto)`, RX/TX/bağlantı sayısı, ilk/son. **rbac:** site-scope parametresi (`agent_id IN (SELECT id FROM agents WHERE site=?)`). **test:** toplam RX/TX = süreç özeti |
| **S23.3** | Uygulama görünürlüğü + bağlantılar: `ProcessDomains` = `agent_dns` ∪ `l7_endpoints` (süreç filtreli); `ProcessConnections` = `agent_conn_latest` süreç filtreli. **dosya:** `agent_dns.go`, `l7.go`, `agents.go`. **risk:** bağlantı "yaş"ı türetilemiyor (`agent_conn_latest`'te ts yok) → alan "-" |
| **S23.4** | Uç: `GET /api/v1/agents/{id}/processes/{ad}` → `{identity,summary,remotes[],connections[],app_visibility[]}`. **dosya:** `internal/server/agents.go`, `server.go` (UI auth + `SiteScope`). **api:** openapi.yaml; ad path-encode. **test:** `agents_test.go` — DNS/SNI yokken de 200 |
| **S23.5** | Zaman çizelgesi ucu: `GET /api/v1/agents/{id}/processes/{ad}/timeline` — sunucu-tarafı birleştirilmiş kronoloji: ilk görülme · DNS kova · L7 host ilk görülme · trafik sıçraması · ilişkili `alert_events` (key LIKE process). **sınır:** pencere bağlı; sıçrama = kova ortalamasının Nσ üstü |
| **S23.6** | `ProcessDetailPage.tsx` + rota `/agentlar/:id/surec/:ad` — Enter açar, Esc döner; paneller Özet/Hedefler/Bağlantılar/Uygulama/Zaman Çizelgesi. **fe:** `Panel · TuiTable · Meter · Sparkline · RangeTabs · useRegisterKeys` yeniden kullanım. **dosya:** `frontend/src/pages/ProcessDetailPage.tsx`, `App.tsx`, `ProcessesCard.tsx` (satır Enter). **test:** `ProcessDetailPage.test.tsx` — pcap kapalı boş-durum. **dep:** ülke/ASN 23-E'den; inmemişse ham IP |

**Doğrulama:** `claude` sürecini seç → `api.anthropic.com` görünmeli; süreç
totalleri Süreç Trafiği tablosuyla mutabık; bağlantılar seçili agent/sürece
kapsamlı; süreç tablosunda perf regresyonu yok. Docs: ARCHITECTURE "Süreç Bazlı
Trafik Atfı" + routing tablosu.

## 23-B — NetFlow Konuşma Toplama · sıfırdan

`flows(ts,device,src,dst,src_port,dst_port,proto,packets,octets)`,
`idx_flows_ts` + `idx_flows_octets`. `TopFlows` = `ORDER BY octets`. TS modunda
`flows_1h` cagg.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S23.7** | `FlowConversations(since,window,key,sort,limit,site)` — key ∈ {`5tuple`,`pair`}; `SUM(packets/octets)`, `COUNT(*)` akış, `MIN/MAX(ts)`. **dosya:** `internal/store/devices.go` (FlowRow bölgesi), `interfaces.go (DeviceStore)`. **pencere:** 15dk/1s/6s/24s → clamp; >48s ise `flows_1h`. **test:** agregat totali ≈ ham |
| **S23.8** | Migrasyon `0015` — EXPLAIN'in gösterdiği bileşik indeks(ler): `(ts,proto)` ve/veya `(src,dst,ts)`. **şema:** `migrations/{sqlite,postgres}/0015_flow_convo_idx.sql`. **test:** `migrate_test.go` + CI db-upgrade-test |
| **S23.9** | Uçlar: `GET /api/v1/flows/conversations` (`?window=&by=&sort=&limit=`) + `.../conversations/{anahtar}` drill-down. **drill:** ham akışlar + `s.geo` GeoIP/ASN + `process_traffic.remote_ip` eşleşmesiyle best-effort agent/süreç. **test:** ham `/api/v1/flows` hâlâ çalışıyor |
| **S23.10** | `TOP KONUŞMALAR` paneli — `TrafficFlowPage` altında. **fe:** `RangeTabs` pencere, sıralanır `TuiTable`, Enter → drill alt-panel. **dosya:** `frontend/src/pages/TrafficFlowPage.tsx` + `TopConversationsCard.tsx`. **test:** ilk 20 verimli; zaman seçici çalışır |

**Doğrulama:** Ham NetFlow görünümü bozulmadı · agregat ≈ ham · ilk 20 hızlı ·
drill-down (kaynak/hedef/port/proto/yön/GeoIP/ASN) açılıyor.

## 23-C — Arayüz Kapasitesi & Kullanım · sıfırdan

`internal/devpoll/poller.go` IF-MIB yürüyor. `device_iface_samples`
speed/oper_status/err/discard tutuyor; kullanım % / tip / eşik yok.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S23.11** | Poller: `ifType` (.1.3.6.1.2.1.2.2.1.3) + `ifHighSpeed` yürü; migrasyon `0016` → `if_type INTEGER`, `high_speed INTEGER`. **dosya:** `poller.go`, `devices.go (SaveDeviceIfaceSamples)`. **test:** `poller_test.go` |
| **S23.12** | Saf fn `classifyIfType(ifType)` → ethernet/wifi/loopback/tunnel/vpn/bridge/vlan/ppp/unknown; `DeviceIfaceRate` + `RxUtilPct/TxUtilPct` (yalnız speed>0 && oper=up). **test:** `devices_test.go` — bilinmeyen hız → util yok |
| **S23.13** | `GET /api/v1/devices/{id}/interfaces`: `util%, class, speed_source` ekle (katkısal, geriye uyumlu). **dosya:** `internal/server/devices.go (handleDeviceIfaces)`. **test:** eski alanlar korunur |
| **S23.14** | `checkIfaceUtil` — (device,ifIndex) başına sürekli-aşım sayacı; `IfaceConfig{WarnPct:70,CritPct:90,SustainSec}`; loopback/tunnel varsayılan hariç. **dosya:** `internal/alert/alert.go` (`checkBandwidth` debounce deseni), `alertKinds.ts`. **kind:** `iface_util` + `kindSeverity`. **test:** `alert_test.go` — eşik süresi respekte; flap yok |
| **S23.15** | `DeviceDetailPage` arayüz tablosu: TYPE / UTIL↓ / UTIL↑ kolonları + satır-içi `Meter`; bilinmeyen hız → "-". **fe:** `DeviceDetailPage.tsx`, `Meter.tsx` |

**Doğrulama:** Yalnız güvenilir hızda util% · bilinmeyen hız "-" ·
loopback/tunnel yanıltıcı uyarı üretmez · eşik süresi respekte.

## 23-D — Topoloji + Canlı Bağlantı Telemetrisi · sıfırdan (bağımlı: 23-C)

`topology_links` + `UpsertTopologyLink/RecentTopologyLinks`, `/api/v1/topology`,
SVG `TopologyCard`. Kenarlarda telemetri/güven yok.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S23.16** | Migrasyon `0017` → `topology_links.confidence TEXT DEFAULT 'discovered'` (discovered\|inferred\|manual); mevcut satırlar 'discovered'. **kural:** trafik çıkarımından link **yaratma** (yaratılırsa 'inferred') |
| **S23.17** | `handleTopology`: source_type='device' + `local_port` → ifName/ifAlias eşleşen kenara `LatestDeviceIfaces`'ten `{speed,rx_bps,tx_bps,util,oper_status}` iliştir. **dosya:** `internal/server/topology.go`. **risk:** eşleşme yoksa kenar telemetrisiz render (graf bozulmaz) — regresyon testi |
| **S23.18** | `TopologyCard`: kenar görsel durumları normal/warning/critical/down/unknown (util + oper_status); elle-SVG konvansiyonu korunur. **fe:** `TopologyCard.tsx`, `TopologyCard.test.tsx` |
| **S23.19** | Link inspector paneli (kenara Enter) — plandaki LINK bloğu; güven rozeti; `refreshKey` ile tam-reload'suz güncelleme. **fe:** `TopologyPage.tsx`. **test:** SNMP yokken topoloji değişmeden render |

**Doğrulama:** Mevcut topoloji SNMP'siz render · SNMP-destekli kenar canlı
telemetri · eksik arayüz eşlemesi grafı bozmuyor · link durumu
tam-reload'suz güncelleniyor.

## 23-E — Hedef Zenginleştirme · kısmi

`internal/geoip.Resolver` (ülke/ASN, MMDB + ip-api batch, 100k LRU).
`EndpointDelta.Country/ASN` sorgu anında yalnız `handleGeo`'da. Domain
normalizasyon / kategori / paylaşılan servis yok.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S23.20** | Yeni paket `internal/enrich`: `Service` = `geoip.Resolver` sarmalar + kayıtlı-alan çözümü; `IPInfo{Country,ASN,Org,Private}` (RFC1918/ULA/loopback → Private, lookup yok), `DomainInfo{Normalized,Registrable,Category}`. **dep:** kayıtlı-alan `x/net/publicsuffix` (dolaylı bağımlılık — **doğrula**) _veya_ gömülü kısa PSL. **önbellek:** bellek-içi LRU + TTL; kategori isteğe bağlı `-domain-category-file`. **test:** saf-fn |
| **S23.21** | Hub'a tak (`cmd/bazntms-hub/main.go`); zenginleştirme yalnız okuma-yolu → ingest'i asla bloklamaz. `handleGeo` ad-hoc lookup'ları servise taşı. **test:** zenginleştirme hatası → uç yine 200, alanlar boş |
| **S23.22** | Süreç-detay hedefleri (23-A), akış konuşmaları (23-B), bağlantılara zenginleştirme iliştir; `GET /api/v1/enrich?ip=&domain=` anlık uç. **api:** openapi.yaml |
| **S23.23** | Paylaşılan `frontend/src/lib/enrich.tsx` render yardımcısı — ASN/ORG/COUNTRY/CATEGORY bloğu; ProcessDetail + Konuşmalar + GeoPage kullanır; RFC1918 → `YEREL` pill. **desen:** `lib/isms.tsx` paylaşımı |
| **S23.24** | **Faz 23 kapanış** — testler + docker smoke + CHANGELOG `[1.2.0]` + ARCHITECTURE enrich bölümü + memory |

**Doğrulama:** Enrichment başarısızsa ingest sürüyor · aynı IP önbellekten ·
RFC1918 adres LOCAL/PRIVATE gösteriyor.

---

# FAZ 24 — Tespit & Korelasyon

Telemetri → EVENT → ALERT → INCIDENT hiyerarşisini tamamla. Uyarı yaşam döngüsü
Faz 22-B'de bitti; bu faz eksik EVENT katmanını, incident motorunu ve AI'sız
korelasyonu ekler.

## 24-A — Normalleştirilmiş Olay Modeli · kısmi

> **Faz 22-B'de hazır:** `alert_events` severity/state(firing\|ack\|resolved\|silenced)/
> count/first_ts/last_ts/group_id/ext_ref · `QueryAlertEvents` filtre · ACK/RESOLVE/
> NOTE uçları _ve denetim kayıtları_ · bakım pencereleri. **Bu alt-faz uyarı
> tarafına dokunmaz.**

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S24.1** | Tasarım spike'ı — ADR `0010-event-model.md`: event_type sayımı + her tür için türetilmiş-mi/materyalize-mi kararı. **eşleme:** `process.*/connection.* → connection_events` · `dns.query → agent_dns` · `tls.sni_observed → l7_endpoints` · `netflow.flow → flows` · `interface.utilization_high / syslog.received → türetilmiş`. **sonuç:** birleşik OKUMA modeli — yeni yazma hattı yok |
| **S24.2** | `QueryEvents(EventFilter{types,agent,device,since,until,limit,cursor})` — kaynak tablolardan `UNION ALL` normalleştirilmiş kolonlar + `source_type`; sınırlı, indeksli, cursor sayfalı. **dosya:** yeni `internal/store/events.go`, `interfaces.go`. **test:** `events_test.go` |
| **S24.3** | `GET /api/v1/events?type=&agent_id=&since=&limit=` — plan alan seti. **dosya:** yeni `internal/server/events.go`, openapi |
| **S24.4** | Uyarı-modeli boşluk kapatma: `suppressed` statüsü (bugün `silenced` — eşleştir/isimlendir); `handleAlertEventAck/Resolve` zaten denetimde olduğunu doğrula. **dosya:** `internal/alert/alert.go`, `internal/store/alerts.go`. **test:** `alert_lifecycle_test.go` |
| **S24.5** | `/uyarilar`'a `OLAYLAR` alt-sekmesi (TabBar) — salt-okunur, filtrelenebilir olay akışı. **fe:** `AlertsPage.tsx` + `EventsPanel.tsx` (`AlertEventsPanel` kardeşi) |

**Doğrulama:** Tekrar eden eşleşen olaylar tek uyarıyı güncelliyor (zaten) ·
uyarılar kabul/çözüm alıyor (zaten) · operatör aksiyonları denetimde (zaten) ·
`/api/v1/events` ham telemetriyi normalleştirerek döndürüyor.

## 24-B — Olay (Incident) Korelasyon Motoru · sıfırdan

**AI/LLM bağımlılığı YOK** — 5 deterministik kural. Lider-kapılı (`store.Leader`,
alert/scheduler ile aynı desen).

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S24.6** | Migrasyon `0018` → `incidents` (id,title,severity,status,created/updated/first/last_seen,summary,correlation_reason,risk_score) + `incident_evidence` (incident_id,kind∈event\|alert,ref_id,ts). **durum:** open\|investigating\|resolved\|closed. **indeks:** status, last_seen, agent |
| **S24.7** | Yeni paket `internal/incident`: `Engine` ~30sn'de bir son `alert_events` + türetilmiş event'ler üstünde. **Kural 1** (yeni süreç + yeni hedef, aynı agent/süreç ≤5dk) + risk skoru fn (0–100, katkısal ağırlık). **dosya:** `internal/incident/engine.go`, `interfaces.go` (yeni alt-arayüz `IncidentStore`). **test:** `incident_test.go` |
| **S24.8** | **Kural 2** (yeni süreç + şüpheli port ≤5dk) · **Kural 3** (yeni hedef + giden trafik>eşik ≤10dk). Dedup: `correlation_key` → açık incident güncelle, `last_seen` ilerlet, severity monoton artar. **test:** her kural + severity yükselmesi |
| **S24.9** | **Kural 4** (yüksek anomali + bant sıçraması, aynı agent/arayüz ≤5dk) · **Kural 5** (≥3 şüpheli uyarı, aynı agent ≤10dk). `IncidentConfig`. **test:** kural 4/5; tüm kanıt incelenebilir |
| **S24.10** | Uçlar: `GET /api/v1/incidents` (status/sev/agent filtre) · `GET .../{id}` (kanıt zaman çizelgesiyle) · `POST .../{id}/{ack,investigate,resolve}` → `s.audit`. **dosya:** `internal/server/incidents.go`, `server.go`, openapi |
| **S24.11** | Incident aç/yükselt → mevcut `Notifier` yolu + isteğe bağlı bilet (`jira.go/servicenow.go` grup mekanizması). **test:** `ticket_test.go` komşusu |

**Doğrulama:** İlişkili olaylar tek incident'a · sebepsiz mükerrer incident yok
· daha yüksek önem kanıtı gelince severity artıyor · tüm destekleyici kanıt
incelenebilir.

## 24-C — Olay Yönetim UI · sıfırdan

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S24.12** | `/uyarilar` → üst `[ ALARMLAR ] [ OLAYLAR ]` TabBar; `IncidentsPanel` listesi (SEV/DURUM/BAŞLIK/AGENT/OLAY/İLK/SON) `TuiTable`, klavye nav. **fe:** `AlertsPage.tsx` + `IncidentsPanel.tsx` |
| **S24.13** | `IncidentDetailPage` / rota `/uyarilar/olay/:id` — özet, korelasyon nedeni, kronolojik kanıt, RELATED linkleri (süreç→ProcessDetail, agent, cihaz, DNS, NetFlow), ACTIONS (`useDialog`). **fe:** `frontend/src/pages/IncidentDetailPage.tsx`, `App.tsx` |
| **S24.14** | `AlertEventsPanel`'de "ilişkili alarmdan olay aç" (uyarıda `incident_id` varsa); durum değişiklikleri denetimli. **fe:** `components/AlertEventsPanel.tsx`, `lib/alertKinds.ts`. **docs:** ARCHITECTURE yeni "Korelasyon motoru" bölümü, ALERTING.md |

**Doğrulama:** İlişkili alarmdan incident açılıyor · kanıt kronolojik · durum
değişiklikleri denetimde · ilişkili süreç/agent/cihaz sayfaları erişilebilir ·
klavye nav korunuyor.

## 24-D — İstatistiksel Baseline Genişletme · kısmi

> **Faz 22-A'da hazır:** materyalize `anomaly_baseline` (Welford), mevsimsel
> weekday×hour, EWMA yarı-ömür, boyutlar local/fleet/site/agent, metrikler
> bps/dns_qps/proc_bps, `CritZ` önem, `MinSamples`. Bu alt-faz yalnız yeni
> metrik/varlık ekler.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S24.15** | `AvgMetricByDim` + `BaselineDayBuckets`: metrik `conn_count` (`agent_conn_latest` agent başına sayım) + `pps`. **dosya:** `internal/store/topology.go`, `internal/alert/anomaly.go` (`knownMetrics`). **test:** `anomaly_metric_test.go` |
| **S24.16** | Metrik `iface_util`, boyut `device`/`iface` — `device_iface_samples` util% üstünde baseline. **dep:** 23-C util%. **test:** rebuild + check |
| **S24.17** | Metrik `new_dest_rate` — agent başına kova başına ilk-görülen uzak IP sayısı (alert `target` seen-set yeniden kullan). **risk:** "yeni agent anomali seli yaratmasın" → `MinSamples` kapsar; doğrula |
| **S24.18** | Anomali event kaydı plandaki alanları taşısın (`metric/actual/expected/z_score/entity/timestamp`); `/anomali` paneline yeni metrik seçici (`AnomalyBandChart` yeniden kullan). **fe:** `AnomalyPage.tsx`, `AnomalyBandChart.tsx`. **docs:** ANALYTICS.md |

**Doğrulama:** Yeni agent anomali seli yaratmıyor · baseline zamanla toparlıyor ·
aykırı değerler baseline'ı kalıcı bozmuyor.

## 24-E — Tehdit İstihbaratı Adaptörü · kısmi

`internal/ioc` (domain kara liste) + `alert/ioc.go` "ioc" uyarısı var.
Sağlayıcı-bağımsız arayüz, IP, itibar, önbellek yok.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S24.19** | Yeni paket `internal/threatintel`: `Provider` arayüzü `LookupIP/LookupDomain → Indicator{indicator,type,reputation,confidence,categories[],source,first/last_seen,raw_ref}`; mevcut `ioc.List` → `localfile` sağlayıcı. **itibar:** trusted\|neutral\|suspicious\|malicious\|unknown. **önbellek:** LRU + TTL · **oto-blok YOK**. **test:** `threatintel_test.go` |
| **S24.20** | Hub'a tak (`-threatintel-*` flag/config); `alert/ioc.go checkIOC` adaptörü çağıracak şekilde refactor (kind `ioc` korunur, mesaja itibar). Kötücül hedef → event/alert → 24-B kural eklentisi ile incident'a korele. **dosya:** `cmd/bazntms-hub/main.go`, `internal/alert/ioc.go`. **test:** `ioc_test.go` |
| **S24.21** | `GET /api/v1/threatintel?ip=&domain=` (önbellekli, kaynak/sağlayıcı gösterir); itibarı süreç-detay hedeflerine, bağlantı detayına, akış konuşmalarına iliştir. **dosya:** yeni `internal/server/threatintel.go`, openapi |
| **S24.22** | `lib/enrich.tsx` içine itibar pill'i (trusted…malicious/unknown) + sağlayıcı adı. **fe:** 23-E'nin `lib/enrich.tsx`'i genişletilir |
| **S24.23** | **Faz 24 kapanış** — testler + docker smoke + CHANGELOG `[1.3.0]` + ADR `0010-event-model` + `0011-incident-engine` + THREAT-MODEL güncelleme + memory |

**Doğrulama:** Sağlayıcı hatası telemetriyi bozmuyor · sonuçlar önbellekli ·
UI kaynağı/sağlayıcıyı tanımlıyor · trafik otomatik bloklanmıyor.

---

# FAZ 25 — Operasyonel Olgunluk

Yorumlanabilir skor, yönetici raporu, denetim ve kayıt sertleştirmesi,
tutarlılık cilası. **Yeni gözlemlenebilirlik verisi yok** — Faz 23/24 çıktıları
paketlenir.

## 25-A — Ağ Sağlık Skoru · sıfırdan

Deterministik ağırlıklı skorlama — **opak AI skoru YOK**, her kesinti
açıklanabilir.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S25.1** | Yeni paket `internal/health`: saf `Score(inputs)→{Score 0-100, Deductions[]{Reason,Points}}`. **girdi:** agent erişilebilirliği (`FleetSummary`) · cihaz poll (`ListDevices.last_poll`) · arayüz err/discard (`FleetIfaceHealth`) · arayüz util (23-C) · aktif crit uyarı (`QueryAlertEvents`) · açık incident (24-B) · telemetri tazeliği. **test:** ağır birim test — tekrarlanabilir, her kesinti açıklanabilir |
| **S25.2** | `GET /api/v1/health` → skor + kesintiler + girdiler; ~30sn önbellek. **dosya:** yeni `internal/server/health.go`, openapi |
| **S25.3** | Panoda `NETWORK HEALTH` paneli (`Overview`) — skor + kesinti listesi; TUI `Meter`/gauge. **fe:** `components/Overview.tsx` + `HealthCard.tsx` |
| **S25.4** | Kurumsal rapora sağlık skoru + kesinti bölümü. **dosya:** `internal/report/enterprise.go`. **docs:** ANALYTICS.md |

**Doğrulama:** Skor tekrar üretilebilir · her kesinti açıklanabilir · opak AI
skoru yok.

## 25-B — Raporlama v2 · kısmi

> **Faz 22-D'de hazır:** zamanlı teslim + arşiv (`report_archive`), PDF
> (`enterprise_pdf.go`), SLA hedef/ihlal (`sla_targets`), saha kırılımı,
> sağlık% + kapasite notu içeren kurumsal rapor, "graceful degrade" deseni
> (`Empty`/`$missing`).

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S25.5** | `enterprise.go` → 13 adlı bölüme yeniden yapılandır; yönetici KPI başlığı (HEALTH/AVAILABILITY/CRIT INCIDENTS/WARNINGS/CAPACITY RISKS/TOP TALKER/TOP DESTINATION); her bölüm telemetri yoksa zarif düşer. **test:** `report_test.go` |
| **S25.6** | Yeni veri kaynakları: Top Conversations (`FlowConversations`/23-B) · Top Destinations (zenginleşmiş `FleetTopEndpoints`/23-E) · Incidents (24-B) · DNS/App (`TopL7/TopAgentDNS`). **dep:** 23-B, 23-E, 24-B |
| **S25.7** | Yeni `internal/report/recommend.go`: deterministik şablon motoru — "Arayüz X, Y dk boyunca %N aştı" · "Agent X, N yeni hedef yarattı" · "Cihaz X yüksek iskarta". Saf fn, tablo-güdümlü. **LLM yok.** **test:** `recommend_test.go` |
| **S25.8** | Yeni bölümlerin PDF/HTML render'ı (`enterprise_pdf.go`), yazdırılabilir; `ReportsPage` önizleme etkilenmez. **docs:** ANALYTICS.md |

**Doğrulama:** Mevcut HTML/PDF çalışıyor · raporlar yazdırılabilir · eksik
telemetri bölümleri zarif düşüyor.

## 25-C — Denetim İzi v2 · kısmi

Mevcut: `audit_events` hash-zincir + `VerifyAuditChain` + saha (0004) +
`s.audit(r,id,action,target,detail)` yardımcısı.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S25.9** | Migrasyon `0019` → `actor_type, request_id, user_agent, result, before_json, after_json`. Hash-zincir girdisi **kararlı kalır** (hash kanonik alt-küme üstünde) → `VerifyAuditChain` hâlâ geçer. **dosya:** `internal/store/users.go` (audit bölgesi), `migrate.go`. **test:** `migrate_test.go` + zincir doğrulama |
| **S25.10** | `s.audit` genişletme: `request_id` (middleware), `user_agent`, `result`; redaksiyonlu BEFORE/AFTER diff yardımcısı (alert-config sır maskeleme listesi yeniden kullan). Uygula: `alert.rule.update, device.update, user.update, token.revoke, incident.resolve`. **kural:** düz metin sır/token/parola ASLA. **test:** `rbac_test.go` / audit testleri |
| **S25.11** | `GET /api/v1/audit`: filtreler actor/action/resource/ip/date (query param); `AuditAdminPage` filtre çubuğu (TUI). **dosya:** `internal/server/users.go (handleAuditList)`, `frontend/src/pages/yonetim/…`. **docs:** THREAT-MODEL / ARCHITECTURE audit |

**Doğrulama:** Hash-zincir doğrulama hâlâ çalışıyor · hassas değerler redakte ·
diff'ler okunabilir.

## 25-D — Agent Kayıt Sertleştirme · kısmi

Mevcut `enroll_tokens(name,token_hash,site,created_at,expires_at,last_used,revoked)`
+ hash'li saklama + `json:"-"`.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S25.12** | Migrasyon `0020` → `max_uses INTEGER DEFAULT 1, used_count INTEGER DEFAULT 0, allowed_cidrs TEXT DEFAULT '', created_by TEXT, revoked_at INTEGER`. `CreateEnrollToken`: `expires_at=0` ise varsayılan 1 gün, "sınırsız" açıkça seçilmedikçe. **dosya:** `internal/store/enroll_tokens.go`. **test:** `enroll_tokens_test.go` |
| **S25.13** | `handleAgentHello` enroll yolu: expiry, `max_uses/used_count` atomik artış, `allowed_cidrs` ↔ remote IP. Süresi geçmiş / tükenmiş / iptal / CIDR-dışı → ayrı 4xx. **dosya:** `internal/server/auth.go` veya `agents.go`. **test:** P14'ün 4 kabul senaryosu |
| **S25.14** | Denetim olayları `token.created/used/revoked`; liste API sırrı asla döndürmüyor doğrula; `EnrollWizard`/`TokensCard` UI: max-uses + CIDR + expiry alanları, "sınırsız" açık toggle. **fe:** `components/EnrollWizard.tsx`, `TokensCard.tsx`. **docs:** DEPLOYMENT-MODEL / THREAT-MODEL |

**Doğrulama:** Süresi geçmiş token reddedilir · tükenmiş token reddedilir ·
iptal token reddedilir · CIDR kısıtı yapılandırılınca zorlanır.

## 25-E — UI/UX Cilası · mevcut temel (Faz 17/18)

> **Faz 17/18'de hazır:** tam TUI dönüşümü, semantik tokenler (cyan=nav/RX,
> mor=TX, yeşil=sağlıklı, sarı=uyarı, kırmızı=kritik), `Meter/TuiTable/Panel/
> Sparkline`, klavye modeli (1-9/F1/F5/F10/`/`/Enter/Esc/oklar), `HelpOverlay`.
> Bu alt-faz **yalnız tutarlılık denetimi** — yeniden tasarım yok.

| Spike | Hedef · dosya · test |
|-------|----------------------|
| **S25.15** | Denetim geçişi: satır aralığı, seçili-satır görünürlüğü, odak halkaları — tüm `TuiTable` kullanımlarında; tutarsızlıklar yalnız `index.css` tokenlerinde. **dosya:** `frontend/src/index.css`, `components/TuiTable.tsx` |
| **S25.16** | Boş / yükleniyor / hata durumları envanteri → küçük `<PanelState kind=>` yardımcısı; eksik sayfalara uygula. Uzun-metin kırpma + tooltip yardımcısı. **fe:** yeni `components/PanelState.tsx` |
| **S25.17** | Klavye tutarlılık taraması: 1-9/F1/F5/F10/`/`/Enter/Esc/oklar her rotada — yeni 23-A/24-C/25 sayfaları dahil; `HelpOverlay` güncel. **dosya:** `lib/useHotkeys.ts`, `KeymapContext.tsx`, `components/HelpOverlay.tsx` |
| **S25.18** | **Faz 25 kapanış** — testler + docker smoke + CHANGELOG `[1.4.0]` (veya kilometre taşı `v2.0.0`) + doc taraması + memory |

**Doğrulama:** Tüm etkileşimli öğeler klavyeyle erişilebilir · terminal/NOC
kimliği korunmuş · 1920×1080'de büyük regresyon yok.

---

## 99 — Kapanış & bilinen sınırlar

- **Bağlantı yaşı** (P1) türetilemiyor — `agent_conn_latest`'te timestamp yok.
  "-" gösterilir; gerçek yaş için ayrı ts kolonu + yazma değişikliği gerekir
  (kapsam dışı).
- **L7/SNI yalnız pcap** — eBPF/ETW arka uçlarında yok (Faz 20 kullanıcı
  kararı). Süreç-detay "Uygulama Görünürlüğü" paneli bu agent'larda DNS ile
  sınırlı kalır.
- **Konuşma toplama pencereleri retention'a bağlı** — `flows` ham retention'ı
  (vars. 7g) dışında yalnız `flows_1h` cagg (TimescaleDB). SQLite'ta uzun
  pencere = kısıtlı.
- **Tehdit istihbaratı** ilk sürümde yalnız `localfile` sağlayıcı — harici API
  sağlayıcıları (OTX, VirusTotal…) arayüzü hazır ama ayrı iş.
- **Incident motoru deterministik** — 5 kural, LLM yok. Kural ayarı config'te;
  yeni kural = kod değişikliği.
- **25-E yeniden tasarım değil** — Faz 17/18 kimliği donuk; yalnız tutarlılık.

### Kapsam dışı (bu kilometre taşında yapılmayacak)

- Otomatik trafik bloklama / aktif yanıt — bazNTMS observability-first kalır.
- Çekirdek rapor üretiminde LLM — yalnız deterministik şablon.
- Yeni telemetri toplama hattı — mevcut agent/NetFlow/SNMP/DNS/SNI verisi
  yeniden kullanılır.
- UI dili / üst navigasyon / API-DB konvansiyon değişikliği (non-negotiable).
