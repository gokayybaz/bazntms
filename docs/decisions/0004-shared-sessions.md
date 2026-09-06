# 0004 — Paylaşımlı oturum deposu (A4 / Faz 15 S15.1)

**Tarih:** 2026-09-06 · **Durum:** kabul edildi

## Sorun

`AuthManager.sessions map[string]*session` süreç-içi bellekte (7 gün TTL).
Sonuçlar:

- Panel (hub-controller) **tek replika** olmak zorunda — `docker-compose.scale.yml`
  bunu açıkça kabul ediyor ("oturumlar replika-içi bellekte").
- Controller yeniden başlayınca **tüm kullanıcılar düşer**.
- OIDC oturumları da aynı map'te.

Controller'ı durumsuzlaştırmanın (C1) ön koşulu.

## Değerlendirilen

**A — Stateless imzalı token (PASETO / JWT + refresh):** her istek DB'ye
gitmez. Ama:

- Yeni bağımlılık (projede `golang-jwt` / `go-paseto` yok; "elle yaz" kültürü).
- **İptal zor:** geçerli bir 7-günlük token'ı süresi dolmadan geçersiz kılmak
  için yine bir denylist (DB) gerekir → stateless avantajı erir. Faz 14'te RBAC'i
  yeni sertleştirdik; kill-switch'siz uzun-ömürlü token bir gerileme.
- Kısa-ömürlü access + refresh = daha çok hareketli parça, refresh endpoint,
  rotasyon.
- Logout / "tüm oturumları kapat" / rol değişince oturum düşürme hepsi ekstra iş.

**B — Paylaşımlı oturum tablosu (Postgres):** ✅ **seçildi.**

## Karar

`SessionStore` arayüzü + iki gerçekleme:

```go
type SessionStore interface {
    Put(tokenHash string, ident Identity, expiresAt time.Time) error
    Get(tokenHash string) (*Identity, bool)
    Delete(tokenHash string) error
    Prune() error
}
```

- **`memSessionStore`** (varsayılan) — bugünkü `map` + mutex davranışı birebir.
  Dev modu ve tek-replika kurulumlar için.
- **`dbSessionStore`** — `store.Store` üzerinden `sessions` tablosu.
  `-session-store=db` ile açılır; `srv.UseDBSessions()` `New()` sonrası,
  ilk istekten önce çağrılır.
- Tablo: `sessions(token_hash TEXT PK, username, role, site, kind, expires_at INTEGER)`.
  Migrasyon `0005_sessions`. **Ham token diskte tutulmaz** — yalnızca
  `sha256(token)` (mevcut `TokenHashString`); bir DB dökümü oturum forge etmeye
  yetmez. Çerezde ham token.
- OIDC dahil tüm giriş yolları (`Login`, `LoginUser`, `handleOIDCCallback`) aynı
  `SessionStore`'a yazar.
- Logout = `Delete(hash)` (replikalar arası anında). Prune: her yazımda +
  10 dk'da bir janitor goroutine (`DELETE WHERE expires_at < now`).
- `Get` her istekte 1 indeksli PK SELECT — Postgres'te sub-ms. İleride kısa-TTL
  (~30sn) pozitif önbellek eklenebilir (logout replikalar arası ~30sn gecikir);
  şimdilik korrektlik için önbelleksiz.
- Rate-limit `attempts` map'i replika-başına kalır (kabul edilebilir — hafif
  daha zayıf, paylaşımlı limiter kapsam dışı).

## Reddedilen

Stateless token (A) — iptal/logout/rol-değişimi ihtiyaçları stateless
avantajını götürüyor; yeni bağımlılık; daha fazla karmaşa. Tek gerçek kazanç
(DB'siz doğrulama) `dbSessionStore`'a küçük bir önbellekle telafi edilebilir.
