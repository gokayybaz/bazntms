# 0013 — Enrollment token sertleştirme: kullanım sınırı + CIDR + güvenli varsayılanlar

- Durum: kabul edildi
- Tarih: 2026-09-08
- Faz: 25-D

## Bağlam

DB enroll token'ları (Faz 10 / ADR yok, `enroll_tokens` tablosu) sızarsa
`revoke` veya `expire` olana dek **sınırsız** agent'ı **herhangi bir IP'den**
kaydediyordu. Dış plan (PHASE 14) kullanım sayacı, kaynak-IP kısıtı ve güvenli
varsayılanlar ister.

## Karar

### Şema: `0020_enroll_token_hardening` — 5 düz `ADD COLUMN`

| kolon | tip | anlam |
|-------|-----|-------|
| `max_uses` | int, **DEFAULT 0** | izinli başarılı enrollment. 0 = sınırsız |
| `used_count` | int, DEFAULT 0 | yapılan enrollment |
| `allowed_cidrs` | text, DEFAULT '' | virgüllü CIDR; boş = her IP |
| `created_by` | text, DEFAULT '' | üreten yönetici |
| `revoked_at` | int, DEFAULT 0 | iptal zamanı |

**`max_uses` şemada `DEFAULT 0`** (plan "DEFAULT 1" diyordu): mevcut,
dağıtılmış çok-agent token'ları migrasyonla sessizce tek-kullanımlığa
dönüşmemeli (bir filo yaygınlaştırması yarıda kalırdı). **Güvenli varsayılan
handler'da**: yeni token API'si `max_uses` atlanırsa **1** yazar. Böylece hem
geriye uyum hem "yeni token = tek kullanım".

### Atomik tüketim: `ConsumeEnrollToken(id) → (ok, err)`

```sql
UPDATE enroll_tokens SET used_count = used_count + 1, last_used = ?
WHERE id = ? AND revoked = 0 AND (max_uses = 0 OR used_count < max_uses)
```

`RowsAffected > 0` = hak verildi. Tek `UPDATE` atomik → eş zamanlı iki agent
son slotu paylaşamaz (çoklu-controller dahil — DB seviyesinde). Eski
`TouchEnrollToken` (async, best-effort) kaldırıldı; `resolveEnrollToken`
artık senkron `ConsumeEnrollToken` çağırır.

### `handleAgentHello` akışı (Faz 25-D)

1. rate-limit (`r.RemoteAddr`)
2. `resolveEnrollToken(r, token)` → statik sır | DB kaydı; **tüketmez**.
   Ayrık 4xx: `401` geçersiz/iptal/süresi dolmuş · `403` CIDR uyumsuz ·
   `409` max_uses zaten dolu
3. çoklu-saha site kontrolü (S14.B1)
4. gövde çöz + `name` doğrula
5. **`ConsumeEnrollToken`** (yalnız DB token) → `false` ise `409`
6. `RegisterOrReuseAgent`
7. `enroll_token.used` denetim kaydı (actor_type=`enroll`, kalan hak)

Adım 6 başarısız olursa bir kullanım tüketilmiş olur — kabul edilir (yönetici
`used_count`'u görür, gerekirse `max_uses` artırır); güvenlik açığı değil.

### CIDR: `r.RemoteAddr`'a göre denetlenir

Login rate-limit ile aynı gerekçe — XFF spoof edilebilir, agent kendi kaynak
IP'sini seçip kısıtı aşamamalı. **L7 reverse proxy arkasında** kaynak IP =
proxy IP'si; operatör ya CIDR'a proxy'yi yazar ya L4 passthrough / doğrudan
bağlantı kullanır (`docs/DEPLOYMENT-MODEL.md`). L4 / doğrudan kurulumda
gerçek per-agent IP görünür.

### Güvenli varsayılanlar (handler)

- `max_uses` atlanmış → `1`
- `expires_in_days` `0`/atlanmış → **1 gün**; `-1` → süresiz; `N>0` → N gün
- `allowed_cidrs` her parça `net.ParseCIDR` ile doğrulanır → hata `400`

## Sonuçlar

- `expires_in_days` semantik değişimi: `0` artık "1 gün" (eski: süresiz).
  Otomasyon `-1` göndermeli. CHANGELOG'da vurgulanır.
- Mevcut token'lar migrasyondan `max_uses=0` (sınırsız) ile geçer — davranış
  değişmez; yalnız yeni token'lar sıkı.
- Yeni bir kısıt boyutu eklemek (ör. `allowed_versions`) = kolon + resolve
  kontrolü; tüketim yolu değişmez.
