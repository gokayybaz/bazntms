# 0011 — Olay (incident) korelasyon motoru: deterministik, agent-kapsamlı

- Durum: kabul edildi
- Tarih: 2026-09-08
- Faz: 24-B

## Bağlam

Dış plan (PHASE 7) `TELEMETRY → EVENT → ALERT → INCIDENT` hiyerarşisini ve 5
deterministik korelasyon kuralı ister. **AI/LLM YOK.**

## Karar

`internal/incident.Engine` — lider-kapılı (uyarı motoruyla aynı lider,
`LeaderKeyAlerts`), ~30 sn'de bir `AlertEventsSince(now - longWindow)`'i
değerlendirir.

### Korelasyon ölçütü: `alert_events.agent_id`

Uyarı `key`'leri tür-bağımlı ve tutarsız (`proc` → `"ad:süreç"`, `bw` →
`"agent-in:ad"`, `ioc` → `"id|domain"`, `target` → IP, `iface_util` →
`"devID|ifIdx|dir"`). Sağlam "aynı agent" ölçütü için **`alert_events`'e
`agent_id` kolonu eklendi** (`0018`); `fireCtx` artık `fireOpts.AgentID` ile
doğrudan yazar. Agent-tabanlı kurallar (`checkAgent*`) + `checkIOC` dolduruldu.

- **`agent_id = 0` uyarılar korelasyona girmez** — hub-yerel kurallar
  (`checkNewTarget`, `checkBandwidth`), cihaz kuralları (`iface_util`,
  `vpn_down`), saha-geneli anomali. Bunlar bir agent'a atfedilemez.
- Sonuç: çoklu-hub filoda `target` (hub-yerel yeni-hedef) incident'lara
  girmez; onun yerine `ioc` (agent'ın gördüğü kötücül domain) kullanılır.

### 5 kural (agent kümesi, ts sıralı)

| # | Koşul | Pencere | Önem tabanı |
|---|-------|---------|-------------|
| 1 | `proc` + (`target` \| `ioc`) | 5 dk | crit |
| 2 | `proc` + `port` | 5 dk | crit |
| 3 | (`target` \| `ioc`) + `bw` | 10 dk | warn / crit (ioc) |
| 4 | `anomaly` + `bw` | 5 dk | warn |
| 5 | ≥ N şüpheli uyarı (vars. 3) | 10 dk | warn / crit (≥N+2) |

Her kural bir `correlation_key = "r<kural>|agent<id>"` üretir. Bir anahtar için
**tek açık incident**: `OpenIncidentByCorrelation` → yoksa oluştur, varsa
`BumpIncident` (last_seen ilerler, **severity yalnız yükselir**, risk en
yükseği tutar). `WHERE status IN ('open','investigating')` kısmi unique indeks
güvenlik ağı.

### Risk skoru (0-100, açıklanabilir)

`base[kural] + önem(+25 crit / +10 warn) + ek-kanıt(+5/kanıt, ≤20) +
crit-kanıt(+5/adet)`, 100'de kırpılır. Şeffaf — panel her bileşeni gösterir.

### Bildirim

Motor `Notifier` geri-çağrısını çağırır (`alerts.NotifyIncident`) → sentetik
bir `AlertEvent` (`kind="incident"`) ile mevcut kanallar (masaüstü/Telegram/
Slack/webhook/SIEM). **Bilet (Jira/ServiceNow) entegrasyonu şimdilik yok** —
grup mekanizması ext_ref alanı hazır; ayrı iş (Faz 22-C'de PagerDuty'nin
atlanması gibi).

## Sonuçlar

- Operatör aksiyonları (`ack`/`investigate`/`resolve`/`close`) denetim
  zincirine yazılır (`incident.<action>`).
- Yeni kural = kod değişikliği (`internal/incident/rules.go`). Config yalnız
  pencere/eşik ayarı.
- İki agent'a yayılan bir saldırı iki ayrı incident üretir (agent-kapsamlı).
  Saha-geneli korelasyon gelecekte ayrı bir kural olarak eklenebilir.
