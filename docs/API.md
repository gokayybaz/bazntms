# API Referansı

Tüm uçlar `/api` altındadır. Kimlik doğrulama açıkken (`-auth-password`) korumalı
uçlar şunlardan birini ister:

- **Cookie**: `nm_session` (login yanıtında HttpOnly olarak set edilir; tarayıcı otomatik taşır, WS handshake'i dahil)
- **Header**: `Authorization: Bearer <token>` (login yanıtındaki `token`)

Yetkisiz istek: `401` + `{"error":"oturum gerekli","auth_required":true}`
Çok fazla hatalı giriş: `429` + `Retry-After: 60`

---

## Kimlik Doğrulama

### `POST /api/login` *(korumasız)*

```json
{"password": "gizliSifre123"}
```

**200:** `{"ok":true,"token":"<hex>"}` + `Set-Cookie: nm_session=...; HttpOnly; SameSite=Lax; Max-Age=604800`
**401:** hatalı şifre · **429:** bloklandı

### `POST /api/logout`

Cookie ve/veya Bearer oturumunu iptal eder. → `{"ok":true}`

### `GET /api/auth/status` *(korumasız)*

```json
{"required": true, "authenticated": false}
```

---

## Canlı Veri

### `WS /ws`

Her saniye bir `tick` gönderir:

```json
{
  "type": "tick",
  "stats": { "running": true, "device": "en0", "bps_in": 52000.0, "...": "..." },
  "connections": [ { "proto": "tcp", "local_addr": "...", "remote_addr": "...", "...": "" } ],
  "alert_events": [ { "id": 1, "ts": 1787938964, "kind": "bw", "key": "in", "message": "..." } ],
  "record": { "recording": false, "packets": 0, "bytes": 0 }
}
```

`stats` alanının tam şeması `GET /api/status` ile aynıdır.

---

## Yakalama

### `GET /api/status`

Anlık yakalama anlık görüntüsü (Snapshot):

| Alan | Tip | Açıklama |
|------|-----|----------|
| `running` | bool | yakalama açık mı |
| `device` | string | aktif arayüz |
| `error` | string | yakalama döngüsü hatası (varsa) |
| `total_packets` / `total_bytes` / `dropped` | int | oturum toplamları |
| `bps_in` / `bps_out` / `bps_local` | float | son saniyenin hızı (bit/sn) |
| `pps` | int | son saniyenin paket/sayısı |
| `protocols` | object | `"TCP": n, "UDP": n, ...` |
| `top_endpoints` | array | `ip, hostname, local, country, asn, in, out, total, packets` (en fazla 50) |
| `top_ports` | array | `port, name, bytes, packets` (25) |
| `top_domains` | array | `domain, queries, responses, ips[]` (30) |
| `history` | array | son 120 saniyelik kova: `ts, in, out, local, packets` (bayt) |

### `GET /api/interfaces`

Ağ arayüzleri: `name, hw_addr, mtu, up, loopback, can_sniff, addresses[], rx_bytes, tx_bytes, rx_packets, tx_packets`

### `POST /api/capture/start`

```json
{"device": "en0"}
```

**200:** `{"ok":true}` — **hata:** `{"ok":false,"error":"... (root/admin yetkisi gerekli olabilir)"}`

### `POST /api/capture/stop`

Yakalamayı durdurur; açık PCAP kaydı da temizce kapanır. → `{"ok":true}`

---

## Geçmiş ve Karşılaştırma

### `GET /api/history?minutes=60`

DB'den dakikalık seriler:

```json
{
  "range_minutes": 60,
  "db_bytes": 106936,
  "totals": { "avg_bps_in": 0, "avg_bps_out": 0, "peak_bps_in": 0, "peak_bps_out": 0, "seconds": 0, "samples": 0 },
  "buckets": [ { "ts": 1787900000, "in": 2500.0, "out": 625.0, "local": 0, "pps": 12 } ]
}
```

`buckets[i].in/out` **bayt/sn** cinsindendir (dakikalık ortalama). `minutes` 1–10080.

### `GET /api/compare?days=7`

```json
{
  "days": [ { "day": 1787864400, "avg_bps_in": 0, "avg_bps_out": 0, "peak_bps_in": 0, "peak_bps_out": 0, "samples": 0 } ],
  "today_hours":     [ { "hour": 0, "bps_in": 0, "bps_out": 0 } ],
  "yesterday_hours": [ { "hour": 0, "bps_in": 0, "bps_out": 0 } ]
}
```

`days[].day` yerel gece yarısı (unix). Saat dizileri her zaman 24 kayıttır; olmayan saatler 0'dır. `days` 2–30.

---

## Rapor

### `GET /api/report?days=7&format=html|pdf`

- `format=html`: `text/html` — tarayıcıda görüntülenir, yazdırılabilir
- `format=pdf`: `application/pdf` + `Content-Disposition: attachment`

İçerik: yönetici özeti, günlük trafik (+grafik), en yoğun hedefler (ülke/ASN),
süreçler, DNS, protokoller, uyarılar, son 3 AI analizi. `days` 1–90.

---

## PCAP Kayıt

| Uç | Açıklama |
|----|----------|
| `POST /api/record/start` | Kaydı başlat. Yakalama açık değilse hata: `kayit için önce yakalama açık olmalı` |
| `POST /api/record/stop` | Kaydı kapat: `{"ok":true,"file":"2026-08-28_20-05-00_en0.pcap","packets":12345,"bytes":987654}` |
| `GET /api/record/status` | `{recording, file, packets, bytes, error}` |
| `GET /api/record/files` | `{name, bytes, mod_time}` dizini (yeni→eski) |
| `GET /api/record/download?file=<ad>` | `.pcap` indirir. Yalnızca kayıt dizinindeki `.pcap` dosyaları; yoksa `400/404` |

---

## Uyarılar

### `GET /api/alerts`

```json
{
  "enabled": true,
  "cooldown_min": 10,
  "bandwidth": { "enabled": true, "in_mbps": 100, "out_mbps": 50, "seconds": 10 },
  "ports": { "enabled": true, "ports": [23, 4444, 1337, 31337] },
  "new_proc": { "enabled": true, "ignore": ["bazntms"] },
  "new_target": { "enabled": true, "min_total_mb": 10 },
  "notifiers": {
    "desktop": true, "generic_url": "", "discord_url": "",
    "slack_url": "", "telegram_token": "", "telegram_chat_id": ""
  }
}
```

### `PUT /api/alerts`

Aynı şemayla gövde; kaydeder ve anında uygular. → `{"ok":true}`

### `GET /api/alerts/events?limit=50`

```json
[ { "id": 1, "ts": 1787939000, "kind": "port", "key": "23", "message": "Şüpheli porta bağlantı: ..." } ]
```

`kind`: `bw` (bant genişliği) · `port` (şüpheli port) · `proc` (yeni süreç) · `target` (yeni hedef)

---

## AI Analizi

### `GET /api/ai/status`

`{"enabled":true,"model":"gpt-4o-mini"}` — `enabled`, MMDB/bayraklar gibi
yapılandırmaya bağlıdır; yerel adres varsa anahtar gerekmez.

### `GET /api/ai/models`

OpenAI-uyumlu `/models` ucunu sorgular: `{"ok":true,"models":[{"id":"qwen2.5:7b"}]}`

### `POST /api/ai/analyze`

```json
{"minutes": 60, "model": "qwen2.5:7b", "chunked": true}
```

- `model` boşsa sunucu varsayılanı kullanılır
- `chunked: true`: parça parça mod (önerilen; küçük modeller için)

**200:** `{"ok":true,"id":1,"model":"...","summary":"1) ... 4) ..."}`
Dönemde veri yoksa: `{"ok":false,"error":"Bu dönem için veritabanında kayıt yok. ..."}`

### `GET /api/ai/insights`

Kayıtlı analizler (son 10): `{id, ts, model, period_minutes, summary}`

---

## Agent Filosu (Faz 1)

Agent uçları UI oturumundan bağımsızdır. İki kimlik yolu:

- **Bearer token** — `Authorization: Bearer <agent_token>`. Token enrollment ile
  verilir, diskte (`bazntms-agent.state.json`) saklanır; hub yalnızca SHA-256
  hash'ini tutar.
- **Karşılıklı TLS (mTLS)** — hub `-tls` ile çalışıyorsa: agent'ın enrollment'ta
  aldığı istemci sertifikası (`CN=bazntms-agent-<id>`, hub CA'sınca imzalı) TLS
  el sıkışmasında sunulur ve Bearer'a eşdeğer kimlik sayılır. Silinmiş agent'ın
  sertifikası reddedilir (CRL yok, `agent_id` çözümü yeterli).

### `POST /api/v1/agent/hello` *(UI auth muaf)*

Enrollment: header `X-Enroll-Token: <hub -enroll-token VEYA aşağıdaki
/api/v1/enroll-tokens ile üretilmiş bir token>` zorunlu.

```json
{"name":"workstation-01","site":"merkezi-ofis","version":"0.1.0","protocol_version":1,"os":"darwin","arch":"arm64","csr_pem":"-----BEGIN CERTIFICATE REQUEST-----\n…"}
```

`csr_pem` opsiyoneldir; verilir ve hub `-tls` ile çalışıyorsa yanıt `client_cert_pem`
+ `ca_cert_pem` içerir (mTLS). CSR'daki CN yok sayılır — hub `bazntms-agent-<id>`
koyar.

**200:** `{"accepted":true,"agent_id":1,"agent_token":"<hex>","telemetry_interval_seconds":30,"pcap_enabled":false,"client_cert_pem":"…","ca_cert_pem":"…"}`

IP başına deneme sınırlaması vardır (login sayfasıyla aynı politika: 1 dakikada
5 başarısız denemeden sonra 1 dakika bloklanır) — token tahmin etmeye çalışan
istekleri engeller. Bloklandığında **429** + `Retry-After: 60` döner; başarılı
bir enrollment sayacı sıfırlar.

### `POST /api/v1/agent/cert` *(Bearer/mTLS agent kimliği)*

İstemci sertifikasını yeniler (agent ömrünün yarısı geçince çağırır). Gövde:
`{"csr_pem":"-----BEGIN CERTIFICATE REQUEST-----\n…"}` →
**200:** `{"client_cert_pem":"…","ca_cert_pem":"…"}`. Hub `-tls` kapalıysa **404**.

### `POST /api/v1/agent/telemetry` *(Bearer agent token veya mTLS)*

```json
{
  "ts": 1788021229,
  "interfaces": [{"name":"en0","rx_bytes":123456,"tx_bytes":9876,"rx_packets":100,"tx_packets":90}],
  "connections": [{"proto":"tcp","local_addr":"192.168.1.43:5000","remote_addr":"1.2.3.4:443","status":"ESTABLISHED","pid":123,"process":"chrome"}]
}
```

**200:** `{"ok":true,"interval":30}` — `interval`, hub politikası (değişirse agent uyar).

### Filo yönetimi *(UI auth ile korunur)*

| Uç | Açıklama |
|----|----------|
| `GET /api/v1/agents` | `{id, name, site, online, last_seen, version, remote_ip, rates[{name, rx_bps, tx_bps, pps, rx_bytes, tx_bytes, rx_packets, tx_packets}], conns}` |
| `GET /api/v1/agents/{id}` | `{agent: <yukarıdaki şema>, connections: [{proto, local_addr, remote_addr, status, pid, process}]}` — son telemetri anındaki gerçek bağlantı envanteri |
| `GET /api/v1/agents/{id}/history?minutes=60` | `[{ts, in, out, local, pps}]` — tüm arayüzlerin toplamı, ardışık örnekler arası bayt/sn + paket/sn (Agent Detay throughput grafiği) |
| `DELETE /api/v1/agents/{id}` | Agent'ı ve telemetrisini sil |
| `PATCH /api/v1/agents/{id}` | Gövde `{name}` — agent'ın hub'taki görüntü adını değiştir (agent tarafında hiçbir şey değişmez) |

### `GET /api/v1/processes?minutes=60&agent_id=0&limit=20` *(UI auth)*

Süreç bazlı trafik top-listesi (Faz 2, nethogs yöntemi: agent'ta pcap + soket→PID
eşlemesi; `agent_iface_samples` gibi `ts` filtresiyle):

```json
[ { "process": "chrome", "bytes_in": 900, "bytes_out": 100, "total": 1000, "agent_count": 1 } ]
```

Hub `-agent-pcap` politikası + agent'ta `-pcap` ikisi de açık olmalı; ham PCAP
kaydı için ek olarak agent'ta `-record`.

Agent çalıştırma:

```bash
# hub tarafı
sudo ./bazntms -enroll-token gizli-token

# uç tarafı (ilk çalıştırma enrollment yapar, token diskte saklanır)
./bazntms-agent -hub-url https://hub.example.com -enroll-token gizli-token -name ws-01 -site ofis
```

Offline iken batch'ler disk kuyruğuna yazılır (`*.state.json.queue.jsonl`,
en fazla 100 batch) ve bağlantı geri gelince otomatik bosaltilir.

---

## RBAC, Kullanıcılar, Token'lar ve Denetim (Faz 5, admin yetkisi gerekir)

Giriş yolları:
- **Legacy**: `POST /api/login {"password":"..."}` → admin kimliği (geriye uyumlu)
- **Kullanıcı (RBAC)**: `POST /api/login {"username":"bob","password":"..."}` → rol + site scope
- **SSO (OIDC)**: `GET /api/auth/oidc/login` → sağlayıcı → callback → oturum
- **API token**: `Authorization: Bearer bnt_...` (entegrasyonlar için)

Roller ve yetkiler:

| Yetki | admin | netops | analyst | viewer |
|---|---|---|---|---|
| görüntüleme (`view`) | ✓ | ✓ | ✓ | ✓ |
| yakalama/kayıt kontrolü (`operate`) | ✓ | ✓ | ✗ | ✗ |
| AI analiz/rapor (`analyze`) | ✓ | ✓ | ✓ | ✗ |
| cihaz ekle/sil (`devices`) | ✓ | ✓ | ✗ | ✗ |
| agent sil (`agents`) | ✓ | ✗ | ✗ | ✗ |
| kullanıcı/token/audit (`admin`) | ✓ | ✗ | ✗ | ✗ |

### `GET /api/v1/users` · `POST /api/v1/users`

Kullanıcı listesi ve oluşturma. Oluşturma: `{username, password (≥8), role, site}`.
`site` doluysa kullanıcı yalnızca o sitenin agent'larını görür.

### `PUT /api/v1/users/{id}` · `DELETE /api/v1/users/{id}`

Rol/site/enabled/sifre güncelleme: `{role?, site?, enabled?, password?}`.
Kendi hesabını silme/kilitleme korumalıdır.

### `GET /api/v1/tokens` · `POST /api/v1/tokens` · `DELETE /api/v1/tokens/{id}`

Entegrasyon API token'ları. Oluşturma: `{name, role, site}` → düz token
**yalnızca bir kez** döner (`{"token":"bnt_..."}`); sunucuda yalnız hash tutulur.
Silme = revoke (Bearing anında geçersizleşir).

### `GET /api/v1/enroll-tokens` · `POST /api/v1/enroll-tokens` · `DELETE /api/v1/enroll-tokens/{id}`

Hub'ın `-enroll-token` bayrağındaki **tek statik sırra ek**, DB'de saklanan
ve hub yeniden başlatılmadan oluşturulup iptal edilebilen enrollment
token'ları — statik sır sızarsa hub'ı yeniden başlatmadan onu değiştirmenin
yolu yoktu; bu, o sırra dokunmadan ayrı, kolayca iptal edilebilir token'lar
eklemenin yoludur. Agent'lar `-enroll-token`/`hub.token` alanına statik sır
yerine bunlardan birini de verebilir — ikisi de `X-Enroll-Token`
başlığında aynı şekilde kabul edilir.

Oluşturma: `{name, site?, expires_in_days?}` (`site` şu an yalnız bilgi
amaçlıdır, zorlanmaz; `expires_in_days` 0/boş = süresiz) → düz token
**yalnızca bir kez** döner (`{"ok":true,"id":1,"token":"ent_..."}`).
Silme = revoke (bir sonraki enrollment denemesinde 401 döner).
Liste: `[{id, name, site, created_at, expires_at, last_used, revoked}]`.

### `GET /api/v1/audit?limit=100`

Append-only denetim kayıtları: `{id, ts, username, role, action, target, detail, ip, prev_hash, hash}`.
Her kayıt bir öncekinin hash'ini taşır (SHA-256 zinciri).

### `GET /api/v1/audit/verify`

Zincir bütünlüğü: `{"ok":true,"broken_at":0,"checked":N}`. `ok:false` ise
`broken_at` ID'sinden itibaren veri tabanı dışarıdan değiştirilmiştir.

### `GET /api/auth/oidc/login` · `GET /api/auth/oidc/callback`

SSO akışı (config'te `oidc.issuer` tanımlıysa aktif). Grup→rol eşlemesi
`oidc.group_roles` ile yapılır; eşleşmeyen kullanıcılar `default_role` alır.

---

## İleri Analiz (Faz 6, UI auth ile korunur)

### `GET /api/v1/topology`

Canlı ağ haritası modeli: `{generated_at, devices[], agents[], links[]}`.
Kenar kaynakları: **LLDP/CDP** (SNMP keşfi), **ARP** (cihaz port uç noktaları),
**subnet** (agent'ların bildirdiği yerel ağlar — CIDR). Kenarlar dedupe edilir;
`ts` = son görülme.

---

## FortiGate REST API (Faz 8, UI auth ile korunur)

Cihaz `vendor: "fortigate"` ile eklendiğinde poller SNMP yerine FortiOS
REST API'yi kullanır. Cihaz ekleme isteğine ek alanlar:

```json
{
  "name": "fgt-ofis", "host": "10.9.9.1", "kind": "firewall", "vendor": "fortigate",
  "api_url": "https://10.9.9.1", "api_token": "REST-API-TOKEN",
  "api_verify_tls": true, "vdom": "root", "poll_seconds": 60
}
```

- `api_token` **vault'ta AES-256-GCM ile şifreli** saklanır ve hiçbir API
  yanıtında döndürülmez. Read-only REST API admin profili önerilir.
- `vdom`: boş/`root` → tek VDOM; `all` → tüm VDOM'lar taranır (veri VDOM
  etiketli saklanır).
- `api_verify_tls: false` → self-signed sertifika kabul edilir.
- Toplanan veriler: `monitor/system/status|resource/usage|interface`,
  `monitor/vpn/ipsec|ssl`, `monitor/virtual-wan/health-check`,
  `cmdb/firewall/policy` (hit sayaçları).

Cihaz veri uçları (`?minutes=` pencere parametreli):

| Uç | İçerik |
|---|---|
| `GET /api/v1/devices/{id}/resources` | CPU/RAM/disk % + oturum sayısı zaman serisi |
| `GET /api/v1/devices/{id}/vpn` | IPsec tüneller + SSL kullanıcıları (durum, uptime, trafiği) |
| `GET /api/v1/devices/{id}/sdwan` | SD-WAN health-check: member bazlı latency/jitter/loss |
| `GET /api/v1/devices/{id}/policies` | Penceredeki en aktif politika hit'leri (delta) |

Yeni uyarı tipleri (Faz 8.5): `vpn_down`, `sdwan_sla_breach`,
`high_sessions` — `PUT /api/alerts` ile `forti` bölümünden yönetilir:
`{"forti":{"vpn_down":true,"sdwan_latency_ms":200,"sdwan_jitter_ms":50,"sdwan_loss_pct":5,"max_sessions":20000}}`.

---

## 5651 Uyumluluk (Faz 9, imzalama `-compliance` ile açılır)

### `GET /api/v1/compliance/status` *(view)*

Panel durumu: `{config, records, last_record_ts, last_hourly?, last_daily?}`.
`last_daily.tsa_status`: `ok` (nitelikli zaman damgası alındı), `none`
(TSA yapılandırılmamış) veya `error:<mesaj>`.

### `GET /api/v1/compliance/evidence?from=2026-08-01&to=2026-09-01&mask=true` *(admin)*

Delil paketi (A.5.28): `{generated_at, from, to, masked, logs[], checkpoints[], verification}`.
Paket, kayıt hash zinciri + saatlik Merkle checkpoint'leri + günlük mühür
(TSA token + imza) içerir. `mask=true` → IP/MAC/kullanıcı PII maskeleme
(A.5.34/A.8.11; paket düzeyinde, ham kayıt değişmez). Doğrulama:

```bash
bazntmsctl verify -bundle paket.json -pubkey compliance.key.pub
```

### `GET /api/v1/compliance/reviews?limit=50` *(admin)* · `POST` *(admin)*

İnceleme tutanakları: `{"kind":"log"|"access","period":"2026-08","notes":"...","finding":""}`.
Tutanaklar audit zincirine de işlenir (A.8.15 log incelemesi / A.8.2 erişim incelemesi).

### `GET /api/report?type=compliance` *(view)*

ISO 27001:2022 Annex A kontrol haritası + 5651 motoru durumu (HTML) —
denetçiye sunulabilir uyum raporu.

---


### Uyarı tipleri (Faz 6.2 anomali)

`GET /api/alerts/events` içinde `kind:"anomaly"`: saat-of-day istatistiksel
baseline (son 7 gün) ile 5 dakikalık pencere verimi arasındaki z-skoru sapması.
Eşik/duyarlılık `PUT /api/alerts` ile `anomaly` bölümünden yönetilir:
`{"anomaly":{"enabled":true,"sensitivity":3.0,"min_samples":120,"window_min":5}}`.

### Bildirim kanalları (Faz 6.3)

`PUT /api/alerts` → `notifiers` bölümüne yeni kanallar:

```json
{
  "notifiers": {
    "teams_url": "https://outlook.office.com/webhook/...",
    "webhook_v2_url": "https://siem.example.com/bazntms",
    "webhook_v2_secret": "paylastiginiz-gizli",
    "email_host": "smtp.kurum.local",
    "email_port": 587,
    "email_from": "bazntms@kurum.local",
    "email_to": ["noc@kurum.local"],
    "email_user": "bazntms",
    "email_pass": "gizli"
  }
}
```

Webhook v2, gövdeyi `X-BazNTMS-Signature: sha256=<hmac>` ile imzalar.

### Kurumsal rapor

`GET /api/report?type=enterprise&days=30` — SLA (agent uptime, cihaz sağlığı,
paket düşme), kapasite (büyüme, toplam trafik) ve banding (p50/p95/p99)
tablo­ları; HTML olarak üretilir.

---

## Cihazlar, NetFlow ve Syslog (Faz 3, UI auth ile korunur)

### `GET /api/v1/devices`

Kayıtlı cihazlar: `{id, name, host, kind, snmp_version, poll_seconds, enabled, sys_name, sys_descr, last_poll, last_error}`.
Kimlik bilgileri yanıt масkedelenir.

### `POST /api/v1/devices`

```json
{
  "name": "core-sw", "host": "10.0.0.2", "kind": "switch",
  "snmp_version": 2, "community": "gizli",
  "poll_seconds": 60
}
```

v3 için: `"v3_user"`, `"v3_auth_proto"` (SHA/SHA256/SHA512/MD5), `"v3_auth_pass"`,
`"v3_priv_proto"` (AES/AES256/DES), `"v3_priv_pass"`. Hassas alanlar AES-256-GCM
kasada şifrelenir (`vault.key` dosyası, ilk çalıştırmada üretilir — **kaybedilirse
cihaz kimlikleri kurtarılamaz**).

### Cihaz verileri

| Uç | Açıklama |
|----|----------|
| `DELETE /api/v1/devices/{id}` | Cihazı ve örneklerini sil |
| `GET /api/v1/devices/{id}/interfaces` | Son örneklerden arayüz listesi + ↓/↑ verimleri + hata sayaçları |

### `GET /api/v1/flows?minutes=15&limit=20`

NetFlow v5 akışları: `{ts, device, src, dst, src_port, dst_port, proto, packets, octets}`
(octets'e göre azalan). Collector: `-flow-port 2055`.

### `GET /api/v1/syslog?limit=100`

RFC3164 olayları: `{id, ts, host, severity (0-7), tag, message}`. Alıcı: `-syslog-port 5514`.

---

## Sysmon

### `GET /api/connections`

Aktif soketler (gopsutil): `{proto, local_addr, remote_addr, status, pid, process, count, country, asn}`

> PID/süreç bilgisi platform yetkilerine bağlıdır: Linux'ta aynı kullanıcıya ait
> veya root; Windows'ta tam; macOS'ta root gerekir.

---

## Örnek: curl ile uçtan uca

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"password":"gizliSifre123"}' | jq -r .token)

curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/api/status | jq '.bps_in'
curl -s -H "Authorization: Bearer $TOKEN" -X POST localhost:8080/api/capture/start \
  -H 'Content-Type: application/json' -d '{"device":"en0"}'
curl -s -H "Authorization: Bearer $TOKEN" -X POST localhost:8080/api/ai/analyze \
  -H 'Content-Type: application/json' -d '{"minutes":60,"chunked":true}' | jq -r .summary
```
