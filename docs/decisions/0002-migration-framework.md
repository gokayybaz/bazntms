# 0002 — Şema migrasyon çerçevesi (C2 / Faz 13 S13.1)

**Tarih:** 2026-09-05 · **Durum:** kabul edildi

## Sorun

Şema iki yerde, tek dev `CREATE TABLE IF NOT EXISTS` bloğu olarak tanımlı
(`migrate()` SQLite, `migratePostgres()` PostgreSQL). Sonradan eklenen kolonlar
elle yazılmış `ensureDeviceColumns()` / `ensureSyslogColumns()` helper'larıyla
(`ALTER TABLE … ADD COLUMN`) geliyor. Eksikler:

- Sürüm izleme yok (`schema_version` / `PRAGMA user_version` yok).
- Her yeni kolon yeni bir `ensureX` helper'ı gerektiriyor; kolon anlamı
  değiştirme / tablo bölme / backfill mekanizması yok.
- Kullanıcılar sürüm atlayarak yükseltiyor; otomatik güvence yok, yalnızca
  UPGRADE-RUNBOOK.

Faz 14 (kiracı modeli — her tabloya `tenant_id`) ve sonrası, sürümlü ve
geri-izlenebilir bir migrasyon zemini olmadan güvenli değil.

## Karar

**Kendi minimal runner'ımız** — harici bağımlılık yok.

- `internal/store/migrations/{sqlite,postgres}/NNNN_ad.sql` — gömülü
  (`//go:embed`), sürüm ön ekine göre sıralı, dialect başına ayrı dizin.
  İki dialect'in DDL'i zaten ayrı (AUTOINCREMENT↔BIGSERIAL, hypertable için
  birleşik PK, INTEGER↔BIGINT) — ayrı dosya bunu doğal kılıyor, dialect-içi
  `if pg` dallanması gerekmiyor.
- `schema_migrations(version INTEGER PK, name TEXT, applied_at INTEGER)` —
  uygulanmış sürümler burada.
- Runner: uygulanmamış her migrasyonu **kendi transaction'ında** çalıştırır
  (hem SQLite hem PostgreSQL DDL transactional — yarım kalan migrasyon diye
  bir "dirty" durum yok, `golang-migrate`'in `force` ihtiyacı doğmuyor).
- PostgreSQL'de tüm runner mevcut `pg_advisory_lock(migrateLockKey)` deseni
  altında, tek bir bağlantıda çalışır (çoklu replika aynı taze DB'ye karşı
  başlarsa migrasyonu sıraya sokar — `deploy/docker-compose.scale.yml` CI
  duman testi bu yarışı yakalıyordu).
- **Faz 13 öncesi DB:** `schema_migrations` boştur. `0001_init` tamamen
  `IF NOT EXISTS` olduğu için mevcut şema üzerinde zararsız çalışır (eksik
  tablo varsa oluşturur, yoksa no-op) ve "uygulandı" işaretlenir. Ayrı bir
  "baseline atla" heuristik'i yok — `agents` tablosu var mı gibi bir kontrol
  gerekmiyor. `schema_migrations` satırı "0001'in tüm içeriği mevcut"
  güvencesidir; sonraki `ALTER` içeren migrasyonlar buna güvenir.
- **Go-fonksiyon migrasyonları:** dialect-koşullu DDL düz SQL ile ifade
  edilemediğinde (SQLite'ta `ADD COLUMN IF NOT EXISTS` yok) `migration.fn`
  kullanılır. `goMigrations` slice'ında kayıtlı, `.sql` dosyalarıyla aynı
  sıralı sürüm uzayında. `0002_device_syslog_columns` = eski
  `ensureDeviceColumns`/`ensureSyslogColumns` (S13.3).

İleride veri migrasyonu (rename / tip değişikliği / backfill) da aynı
`migration.fn` mekanizmasıyla yazılır.

## Reddedilen

**`pressly/goose`** — embed.FS + modernc/sqlite uyumlu, Go-fonksiyon migrasyonu
dahil olgun bir araç. Ama bir doğrudan + geçişli bağımlılık getiriyor; projenin
"elle yaz" kültürüne (SVG grafikler, NetFlow/sFlow parser'ları, GeoIP
merkezleri hep stdlib-only) aykırı. Go-fonksiyon migrasyonu kendi runner'a
~20 satırla eklendi (bkz. `migration.fn`). İki dialect için yine ayrı dizin
gerekiyordu (goose dialect-içi switch yapmaz).

**`golang-migrate/migrate`** — yalnız SQL, ayrı driver paketleri, `force` ile
dirty-state kurtarma seremonisi. DDL transactional olduğu için o seremoniye
ihtiyacımız yok. En az uyan seçenek.
