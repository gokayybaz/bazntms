# 0012 — Denetim kaydı v2: hash zinciri stabil kalarak zenginleştirme

- Durum: kabul edildi
- Tarih: 2026-09-08
- Faz: 25-C

## Bağlam

Faz 5.3'ten beri `audit_events` append-only bir SHA-256 hash zinciridir
(`hash = SHA256(prev_hash | ts|username|role|action|target|detail|ip)`).
Dış plan (PHASE 13) SOC2/ISO 27001 delili için daha fazlasını ister:

- **actor_type** — aktör insan kullanıcı mı, API token mı, OIDC mi, sistem mi
  (`role` bunu karıştırıyor).
- **request_id** — denetim olayını sunucu loglarıyla ilişkilendirme.
- **user_agent** — aktörün istemci imzası.
- **result** — işlem başarılı mı (`ok` / `error` / `denied`).
- **before_json / after_json** — asıl durum değişikliği (yapılandırma diff'i).

Kısıt: **mevcut zincir bozulmamalı** — `VerifyAuditChain` üretimdeki eski
kayıtlarda geçmeye devam etmeli.

## Karar

### Şema: `0019_audit_v2` — 6 düz `ADD COLUMN` (hepsi `TEXT NOT NULL DEFAULT ''`)

`actor_type, request_id, user_agent, result, before_json, after_json` +
`idx_audit_action`. Migrasyon çerçevesi ekleri izler (sqlite + postgres).

### Hash: v2 segmenti **koşullu**

```
hash = SHA256(prev | ts|username|role|action|target|detail|ip
              [ | actor_type|result|request_id|user_agent|before|after ])
```

Ek segment **yalnızca 6 v2 alanından en az biri doluysa** hash'e katılır
(`AuditEvent.auditV2()`). Sonuç:

- **Eski kayıtlar** (tüm v2 alanları `''`) → segment atlanır → hash birebir
  eski formül → **mevcut zincir aynen doğrulanır**.
- **v2 kayıtları** — `audit()` her zaman `actor_type` + `result` set eder →
  segment daima katılır → `before_json`/`after_json` dahil **tamper-evident**.

`site` alanının zaten hash dışı olması (S14.B2) bu "metadata kolonu"
yaklaşımının presedanıdır; fark: v2'de diff alanları zincir korumasındadır.

### Sır maskeleme: `redactAuditJSON`

`before`/`after` `json.Marshal` → `map[string]any` → özyinelemeli tarama;
alan adı `password|secret|token|apikey|passphrase|community|authpass|
privpass|credential|private_key|seed` içeriyorsa değer `"•••"` (boş değer
maskeye çevrilmez — gürültü). Alert config PUT'unda `handleAlertsPut` zaten
`cur`/`cfg`'yi maskeli veriyor; walker ikinci savunma hattı.

### `user_agent` 256'ya kırpılır

Hash'e girdiği için `InsertAuditEvent` sınırlar.

### Uygulanan durum farkı (`auditDiff`)

| Eylem | before / after |
|-------|----------------|
| `alerts.update` | tam alert.Config (maskeli) |
| `user.update` | `{role, site, enabled}` |
| `device.uplink` | `{uplink_device_id}` |
| `incident.<action>` | `{status, ack_by}` |

Diğer ~40 `audit()` çağrısı yalnızca oto-yakalanan
`actor_type/request_id/user_agent/result=ok` alır. `denied` → `result=denied`
(rbac.go), `login.failed` → `result=error`.

### Sorgu: `QueryAuditEvents(AuditFilter)`

`actor` (username LIKE), `action` (tam veya `x.*` öneki), `resource`
(target LIKE), `ip`, `result`, `since`/`until`, `limit`. Parametreli.
`RecentAuditEvents` artık bunun sarmalayıcısı. `GET /api/v1/audit` query
paramları; `AuditCard` süzgeç barı + satır seçince öncesi/sonrası paneli.

## Sonuçlar

- Eski DB'ler sorunsuz yükselir; ilk v2 yazımından sonra zincir yeni formülle
  devam eder (karışık zincir doğrulanır — `TestAuditChainStableAcrossV2`).
- Yeni bir yüksek-değerli diff eklemek = ilgili handler'da `auditDiff`
  çağrısı; şema değişmez.
- `before_json`/`after_json` büyük olabilir (alert Config ~2 KB). Denetim
  yazımı seyrek olduğu için kabul; ileride sıkıştırma opsiyonel.
