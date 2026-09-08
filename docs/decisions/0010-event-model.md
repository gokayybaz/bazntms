# 0010 — Normalleştirilmiş olay modeli: yeni pipeline değil, birleşik okuma modeli

- Durum: kabul edildi
- Tarih: 2026-09-08
- Faz: 24-A

## Bağlam

Dış milestone planı (PHASE 6) "EVENT" ile "ALERT"i ayırmayı ister: EVENT ham /
normalleştirilmiş gözlem (`dns.query`, `tls.sni_observed`, `netflow.flow`,
`connection.opened`, `syslog.received`, …), ALERT ise bir dedektörün bir veya
daha çok EVENT'e karşı eşleşmesi.

**Uyarı (ALERT) tarafı Faz 22-B'de zaten tamamlandı:** `alert_events` yaşam
döngüsü (severity/state/count/first_ts/last_ts/group_id), ACK/RESOLVE/NOTE
uçları (denetime yazılıyor), bakım pencereleri, filtreli sorgu. Bu ADR yalnız
eksik EVENT katmanını ele alır.

## Karar

**Yeni bir `events` yazma tablosu / toplama hattı KURULMAZ.** bazNTMS'nin
konvansiyonu (CLAUDE.md): "Do not create duplicate telemetry pipelines."
Aşağıdaki gözlemler zaten ilgili tablolara yazılıyor:

| Olay türü | Kaynak tablo |
|---|---|
| `dns.query` | `agent_dns` |
| `tls.sni_observed` / `http.host_observed` | `l7_endpoints` (`kind`) |
| `netflow.flow` | `flows` |
| `connection.seen` | `connection_events` (hub-yerel yakalama) |
| `syslog.received` | `syslog_events` |

`internal/store/events.go` → `QueryEvents(EventFilter)` bu tabloların üzerinde
**birleşik bir OKUMA sorgusu** (`UNION ALL`, normalize kolon kümesi +
`type`/`source` etiketi) sunar. `GET /api/v1/events` bunu döndürür. Her kaynak
tablosunun kendi `ts` indeksi taramayı sınırlı tutar; istenmeyen türlerin
`UNION` dalı hiç eklenmez.

### Kapsam dışı bırakılanlar (bilinçli)

- `process.started` / `process.stopped` — `process_traffic` MIN(ts) ile
  *türetilebilir* ama satır-başına-olay değil (GROUP BY); süreç detayı zaman
  çizelgesi (23-A) bunu zaten veriyor. v1'de olay akışında yok.
- `connection.opened` / `connection.closed` — `connection_events` yalnız
  hub-yerel yakalamada dolar; çoklu-hub'da boş. `agent_conn_latest`'te ts yok →
  aç/kapa event'i çıkarılamaz. `connection.seen` (yakalama olduğunda) sunulur.
- `interface.utilization_high` — `alert_events` kind=`iface_util` olarak zaten
  var (23-C); ayrı bir "event" olarak tekrarlanmaz.

### Uyarı-modeli boşluk kontrolü (S24.4)

- `state='silenced'` ≈ plan'daki `suppressed` — ayrı bir statü eklenmez, isim
  eşlenir (dokümantasyon).
- ACK/RESOLVE **zaten denetime yazılıyor** (`handleAlertEventAck/Resolve` →
  `s.audit("alert.ack"/"alert.resolve")`). Ek iş yok.

## Sonuçlar

- `/api/v1/events` çoklu-hub'da `agent_dns` + `l7_endpoints` + `flows` +
  `syslog_events`'ten beslenir; `connection.seen` yalnız tek-makine kurulumda.
- Olay akışı ham tabloların retention penceresiyle sınırlı.
- İleride bir olay türü gerçekten materyalize gerektirirse (ör. korelasyon
  motoru performansı) ayrı bir ADR ile eklenir.
