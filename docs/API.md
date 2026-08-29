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

Agent uçları UI oturumundan bağımsızdır: `Bearer <agent_token>` kullanır.
Agent token'ı enrollment ile verilir ve diskte (`bazntms-agent.state.json`)
saklanır; hub yalnızca SHA-256 hash'ini tutar.

### `POST /api/v1/agent/hello` *(UI auth muaf)*

Enrollment: header `X-Enroll-Token: <hub -enroll-token>` zorunlu.

```json
{"name":"workstation-01","site":"merkezi-ofis","version":"0.1.0","protocol_version":1,"os":"darwin","arch":"arm64"}
```

**200:** `{"accepted":true,"agent_id":1,"agent_token":"<hex>","telemetry_interval_seconds":30}`

### `POST /api/v1/agent/telemetry` *(Bearer agent token)*

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
| `GET /api/v1/agents` | `{id, name, site, online, last_seen, version, remote_ip, rates[{name, rx_bps, tx_bps}], conns}` |
| `GET /api/v1/agents/{id}` | Agent detayı + güncel bağlantı envanteri |
| `DELETE /api/v1/agents/{id}` | Agent'ı ve telemetrisini sil |

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
