package store

// Şema migrasyon runner'ı (Faz 13 S13.2–S13.3).
//
// Faz 13 öncesi şema iki dev `CREATE TABLE IF NOT EXISTS` bloğuyla (migrate()
// / migratePostgres()) + elle `ensureXColumns` helper'larıyla yönetiliyordu;
// sürüm izleme yoktu. Artık:
//
//   - internal/store/migrations/{sqlite,postgres}/NNNN_ad.sql — gömülü
//     (//go:embed), sürüm ön ekine göre sıralı, dialect başına ayrı dizin.
//     Dialect-koşullu DDL için `goMigrations` (migration.fn) — düz SQL'le
//     ifade edilemeyen adımlar (ör. SQLite'ta "ADD COLUMN IF NOT EXISTS" yok).
//   - schema_migrations(version, name, applied_at) — uygulanmış sürümler.
//   - Uygulanmamış her migrasyon kendi transaction'ında çalışır (hem SQLite
//     hem PostgreSQL DDL transactional → yarım kalmış "dirty" durum yok).
//   - PostgreSQL'de tüm runner mevcut pg_advisory_lock(migrateLockKey) deseni
//     altında tek bağlantıda çalışır (çoklu replika aynı taze DB'ye karşı
//     başlarsa migrasyonu sıraya sokar — bkz. pg.go migrateLockKey yorumu).
//   - Faz 13 öncesi bir DB'de schema_migrations boştur; 0001_init tamamen
//     "IF NOT EXISTS" olduğu için mevcut şema üzerinde zararsız çalışır ve
//     kaydedilir (ayrı "baseline atla" heuristik'i yok).
//
// Karar notu: docs/decisions/0002-migration-framework.md

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationFS embed.FS

// migration, tek bir sıralı şema adımı: ya gömülü bir .sql dosyası (body) ya
// da kayıtlı bir Go fonksiyonu (fn) — dialect-koşullu DDL (ör. SQLite'ta
// "ADD COLUMN IF NOT EXISTS" yok) düz SQL ile ifade edilemediğinde fn kullanılır.
type migration struct {
	version int
	name    string // .sql'siz dosya adı / fn adı — schema_migrations.name'e yazılır
	body    string
	fn      func(ctx context.Context, tx migExec, pg bool) error
}

// sqlConn, hem *sql.DB hem *sql.Conn tarafından sağlanan ortak yüzey. Runner
// PostgreSQL'de advisory-lock'lu tek bir *sql.Conn, SQLite'ta doğrudan *sql.DB
// üzerinde çalışır.
type sqlConn interface {
	migExec
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// migExec, bir migrasyonun (transaction içinde) ihtiyaç duyduğu dar yüzey;
// *sql.Tx, *sql.Conn ve *sql.DB'nin hepsi sağlar.
type migExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// runMigrations, dialect için gömülü migrasyonları sırayla uygular.
func runMigrations(db *sql.DB, pg bool) error {
	dialect := "sqlite"
	if pg {
		dialect = "postgres"
	}
	migs, err := loadMigrations(dialect)
	if err != nil {
		return err
	}
	ctx := context.Background()

	if pg {
		conn, err := db.Conn(ctx)
		if err != nil {
			return fmt.Errorf("migrasyon bağlantısı alınamadı: %w", err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrateLockKey); err != nil {
			return fmt.Errorf("migrasyon kilidi alınamadı: %w", err)
		}
		defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrateLockKey) }()
		return applyMigrations(ctx, conn, pg, migs)
	}
	return applyMigrations(ctx, db, pg, migs)
}

// loadMigrations, gömülü .sql dosyalarını dialect dizininden okur; dosya adı
// NNNN_ad.sql biçiminde olmalı, sürüm ön ekine göre sıralanır.
func loadMigrations(dialect string) ([]migration, error) {
	dir := "migrations/" + dialect
	entries, err := migrationFS.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gömülü migrasyon dizini %q: %w", dir, err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		numPart, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("migrasyon dosya adı NNNN_ad.sql biçiminde değil: %s", e.Name())
		}
		v, err := strconv.Atoi(numPart)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("migrasyon sürüm ön eki pozitif tamsayı değil: %s", e.Name())
		}
		body, err := migrationFS.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{
			version: v,
			name:    strings.TrimSuffix(e.Name(), ".sql"),
			body:    string(body),
		})
	}
	out = append(out, goMigrations...)
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i := 1; i < len(out); i++ {
		if out[i].version == out[i-1].version {
			return nil, fmt.Errorf("çift migrasyon sürüm numarası: %d", out[i].version)
		}
	}
	return out, nil
}

func applyMigrations(ctx context.Context, c sqlConn, pg bool, migs []migration) error {
	if _, err := c.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER NOT NULL PRIMARY KEY,
	name       TEXT    NOT NULL,
	applied_at INTEGER NOT NULL
)`); err != nil {
		return fmt.Errorf("schema_migrations oluşturulamadı: %w", err)
	}

	applied, err := appliedVersions(ctx, c)
	if err != nil {
		return err
	}

	insertStmt := `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?,?,?)`
	if pg {
		insertStmt = `INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1,$2,$3)`
	}

	// Faz 13 öncesi bir DB'de schema_migrations boştur; 0001_init tamamen
	// "IF NOT EXISTS" olduğu için mevcut şema üzerinde zararsız çalışır
	// (eksik tablo varsa oluşturur, yoksa no-op) ve "uygulandı" işaretlenir.
	// Sonraki migrasyonlar ALTER içerebileceğinden bu kayıt önemlidir:
	// schema_migrations satırı "0001'in tüm içeriği mevcut" güvencesidir.
	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		tx, err := c.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var mErr error
		if m.fn != nil {
			mErr = m.fn(ctx, tx, pg)
		} else {
			_, mErr = tx.ExecContext(ctx, m.body)
		}
		if mErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrasyon %s: %w", m.name, mErr)
		}
		if _, err := tx.ExecContext(ctx, insertStmt, m.version, m.name, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrasyon %s kaydedilemedi: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrasyon %s commit: %w", m.name, err)
		}
	}
	return nil
}

func appliedVersions(ctx context.Context, c migExec) (map[int]bool, error) {
	rows, err := c.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// goMigrations, .sql ile ifade edilemeyen (dialect-koşullu DDL) adımlar.
// Sürüm numaraları gömülü .sql dosyalarıyla çakışmamalıdır.
var goMigrations = []migration{
	{version: 2, name: "0002_device_syslog_columns", fn: migrate0002},
}

// migrate0002, Faz 13 öncesi elle yazılmış ensureDeviceColumns /
// ensureSyslogColumns helper'larının yerini alır. Kolonların bir kısmı
// 0001_init'te zaten var (taze DB) — eksik olanları koşullu ekler; hepsi
// varsa no-op. SQLite'ta "ADD COLUMN IF NOT EXISTS" olmadığı için Go adımı.
func migrate0002(ctx context.Context, tx migExec, pg bool) error {
	if err := addColumns(ctx, tx, pg, "devices", [][2]string{
		{"vendor", "TEXT NOT NULL DEFAULT 'snmp'"},
		{"api_url", "TEXT NOT NULL DEFAULT ''"},
		{"api_token_enc", "TEXT NOT NULL DEFAULT ''"},
		{"api_verify_tls", "INTEGER NOT NULL DEFAULT 1"},
		{"vdom", "TEXT NOT NULL DEFAULT ''"},
		{"site", "TEXT NOT NULL DEFAULT ''"},
	}); err != nil {
		return err
	}
	return addColumns(ctx, tx, pg, "syslog_events", [][2]string{
		{"source_ip", "TEXT NOT NULL DEFAULT ''"},
	})
}

// addColumns, tabloda olmayan kolonları ekler. table/name/def derleme zamanı
// sabitleridir (kullanıcı girdisi değil) — identifier parametrelenemez.
func addColumns(ctx context.Context, tx migExec, pg bool, table string, cols [][2]string) error {
	have, err := columnSet(ctx, tx, pg, table)
	if err != nil {
		return err
	}
	for _, c := range cols {
		if have[c[0]] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+c[0]+` `+c[1]); err != nil {
			return fmt.Errorf("%s.%s eklenemedi: %w", table, c[0], err)
		}
	}
	return nil
}

// columnSet, bir tablonun kolon adlarını döndürür.
func columnSet(ctx context.Context, tx migExec, pg bool, table string) (map[string]bool, error) {
	out := map[string]bool{}
	if pg {
		rows, err := tx.QueryContext(ctx,
			`SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1`, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			out[name] = true
		}
		return out, rows.Err()
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
