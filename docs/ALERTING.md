# Uyarı Motoru — Yaşam Döngüsü, Yönlendirme, Bakım Pencereleri

Faz 22 (v1.1). Kaynak: `internal/alert/`. Yapılandırma `alert_config` tek-satır
JSON'unda (`GET/PUT /api/alerts`, yönetici); bakım pencereleri + zamanlamalar
ayrı tablolarda.

## Olay yaşam döngüsü

Bir uyarı ateşlendiğinde `alert_events`'e **durumlu** bir kayıt girer:

```
firing ──ack──▶ ack ──resolve──▶ resolved
   │                                 ▲
   └──── otomatik / koşul-tabanlı ───┘
   └──── (bakım penceresi) ──▶ silenced
```

| alan | anlam |
|---|---|
| `severity` | `info` / `warn` / `crit` — `Config.Severities[kind]` veya `kindSeverity` varsayılanı; anomali `crit_z` ile dinamik |
| `state` | `firing` / `ack` / `resolved` / `silenced` |
| `site` | saha kapsamı (agent uyarıları + anomali site/agent boyutundan) |
| `count` / `first_ts` / `last_ts` | **dedup**: aynı `(kind,key)` açık olay varsa yeni satır değil `count++` + `last_ts` |
| `group_id` | korelasyon (aşağıda) |
| `ext_ref` | bağlı bilet (`jira:PROJ-1` / `snow:<sys_id>`) |
| `ack_by` / `ack_ts` / `note` | operatör aksiyonu |

### Otomatik çözülme (`auto_resolve_min`, varsayılan 15 dk; `< 0` kapalı)

Açık bir olay `auto_resolve_min` dakika boyunca yinelenmezse (bump gelmezse)
motor `resolved` işaretler ve `notify_resolve` ise "**[ÇÖZÜLDÜ]**" bildirimi
gönderir. Cooldown temizlenir → koşul tekrarlarsa yeni olay açılır.

Anomali ayrıca **koşul-tabanlı** çözülür: değerlendirmede mevcut kova artık
aday değilse (z eşiğin altına döndü) açık anomali olayı hemen kapatılır.

### Korelasyon (`correlate_window_sec`, varsayılan 120)

Aynı sahada pencere içinde ateşlenen olaylar ortak `group_id` alır
(`g-<en_eski_id>`). Panelde tek kök-neden olarak katlanır; bilet sistemleri
grup başına **tek** issue/incident açar. Sahasız (hub-yerel / filo) olaylar
gruplanmaz.

## Operatör aksiyonları — `POST /api/v1/alerts/events/{id}/...`

| uç | etki |
|---|---|
| `.../ack` `{note?}` | `state=ack`, `ack_by/ack_ts`, opsiyonel not |
| `.../resolve` | `state=resolved`, cooldown temizlenir |
| `.../note` `{note}` | notu günceller |

`GET /api/v1/alerts/events` filtreler (joker): `kind`, `severity`, `state`,
`site`, `group`, `since` (unix) + cursor sayfalama (`cursor` → `next_cursor`).
Site-kapsamlı kimlik yalnız kendi sahasının olaylarını görür/işler.

## Bakım pencereleri (susturma) — `alert_silences`

`GET` (görüntüleme) / `POST` + `DELETE` (operatör) `/api/v1/alerts/silences`.
Eşleşme alanları boşsa joker:

| alan | eşleşme |
|---|---|
| `match_kind` | uyarı türü (tam) |
| `match_site` | saha (tam) |
| `match_key` | uyarı anahtarında **alt-dize** |

Aktif pencere (`starts_ts ≤ now < ends_ts`) içindeyken eşleşen **yeni** uyarı
`state=silenced` kaydedilir ve **bildirilmez**. Zaten açık olaylar
etkilenmez. Site-kapsamlı kimlik `match_site`'ı kendi sahasına kilitlenir.
Motor önbelleği 20 sn'de bir + POST/DELETE sonrası tazelenir.

## Bildirim kanalları + yönlendirme

Kanallar: `desktop` · `generic` (webhook) · `discord` · `slack` · `telegram` ·
`teams` · `webhook_v2` (HMAC imzalı) · `email` (SMTP) · `siem` (CEF/LEEF/JSON/
syslog) · `jira` · `servicenow`. Sırlar (`*_token`, `*_pass`, `*_secret`)
GET'te maskeli; Jira/ServiceNow kimlikleri ayrıca kimlik kasasında şifreli.

### `notify_routes`

Kural yoksa **etkin tüm kanallara** gider (mevcut davranış). Kural varsa
yukarıdan aşağı değerlendirilir:

```
route = { severity?, kind?, site?, channels: [...], continue? }
```

Boş matcher alanı joker. İlk eşleşen kuralın kanalları toplanır; `continue`
yoksa durulur. **Hiçbir kural eşleşmezse** güvenli-varsayılan: tüm kanallar
(uyarı düşürülmez).

Örnek — kritikler PagerDuty + Slack, gerisi yalnız Slack:

```json
"notify_routes": [
  { "severity": "crit", "channels": ["slack", "pagerduty"] },
  { "channels": ["slack"] }
]
```

## Bilet sistemleri

### Jira Cloud (`internal/alert/jira.go`)

`base_url` + `email` + `api_token` (kasada şifreli) + `project` +
`issue_type` (vars. `Task`) + `resolve_transition` (vars. `Done`). REST v3,
Basic auth, ADF gövde. Uyarı **grubu** ilk ateşlendiğinde issue oluşturulur
(`ext_ref=jira:PROJ-1`); gruba yeni uyarı → yorum. Grup çözüldüğünde yorum +
transition.

### ServiceNow (`internal/alert/servicenow.go`)

`base_url` + `user` + `password` (kasada şifreli). Table API `incident`.
`severity → impact/urgency` (`crit`=1). Çözüldüğünde `state=6 (Resolved)` +
`close_notes`. **"Temel destek"**: out-of-box `incident` tablosu alanlarına
göre; özelleştirilmiş instance'larda `impact`/`urgency`/`state` kodları ve
alan adları farklı olabilir — kendi instance'ınızda bir kez doğrulayın.

### "Test Et"

`POST /api/alerts/test` — issue/incident **açmadan** hafif bağlantı denemesi
(`/rest/api/3/myself`, `incident?sysparm_limit=1`) + diğer kanallara sentetik
uyarı. Sonuç kanal durumu `GET /api/alerts/status`.

## Arayüz kullanım eşiği (`iface`, Faz 23-C)

`internal/alert/iface.go` — SNMP arayüz verimi güvenilir hızla (ifSpeed veya
ifHighSpeed) karşılaştırılır. Config (`/api/alerts` JSON → `iface`):

| alan | varsayılan | anlam |
|---|---|---|
| `enabled` | true | kontrol açık |
| `warn_pct` | 70 | uyarı eşiği (%) |
| `crit_pct` | 90 | bu yüzde aşılırsa olay önemi `crit` (aksi `warn`) |
| `sustain_sec` | 300 | eşik bu süre boyunca aşılmalı (flap koruması) |

Kontrol dakikada bir; `sustain_sec / 60` ardışık geçişte `iface_util` olayı
ateşlenir (anahtar `deviceID|ifIndex|rx|tx`). **Yalnız güvenilir hızlı**
(`speed_bps > 0`) **ve oper=up** arayüzler; `class ∈ {loopback, tunnel}`
atlanır (yanıltıcı). Kullanım eşik altına düşünce sayaç sıfırlanır →
yinelenmeyen açık olay `auto_resolve_min` sonrası otomatik kapanır.
Arayüz sınıfı `classifyIfType` (IANAifType → ethernet/wifi/loopback/tunnel/
vpn/bridge/vlan/ppp/unknown); tünel arayüzü ad/alias VPN ipucu taşırsa `vpn`.

## SLA hedefleri (`sla_targets`)

`GET` (görüntüleme) / `PUT` + `DELETE` (global-admin) `/api/v1/sla/targets`.
`scope=global` (temel) + `scope=site` (saha-özel geçersiz kılma). Alanlar
(0 = kontrol yok):

| alan | ihlal koşulu |
|---|---|
| `agent_uptime_pct` | online agent oranı bu tabanın altında |
| `device_health_pct` | sağlıklı (hatасız + taze poll) cihaz oranı altında |
| `iface_err_ceiling` | 24 saatte toplam arayüz iskarta+hata bu tavanın üstünde |

Motor 10 dk'da bir global + agent'lı her sahayı değerlendirir; ihlal →
`sla_breach` uyarısı (severity **crit**). Kurumsal raporda hedef-vs-gerçek
tablosu + ihlal vurgusu.

## Zamanlanmış raporlar

Bkz. [`decisions/0009-scheduled-jobs.md`](decisions/0009-scheduled-jobs.md).
`GET/POST/DELETE /api/v1/reports/schedules` (kind=`report` `scheduled_jobs`),
`GET /api/v1/reports/archive[/{id}]`, `POST /api/v1/reports/generate` (şimdi
üret). Üretilen rapor `<data>/reports/`'a yazılır, `report_archive`'a
kaydedilir, alıcı varsa alert SMTP'siyle e-postalanır.
