package store

// Şema migrasyon runner'ı (Faz 13 S13.2).
//
// Faz 13 öncesi şema iki dev `CREATE TABLE IF NOT EXISTS` bloğuyla (migrate()
// / migratePostgres()) + elle `ensureXColumns` helper'larıyla yönetiliyordu;
// sürüm izleme yoktu. Artık:
//
//   - internal/store/migrations/{sqlite,postgres}/NNNN_ad.sql — gömülü
//     (//go:embed), sürüm ön ekine göre sıralı, dialect başına ayrı dizin.
//   - schema_migrations(version, name, applied_at) — uygulanmış sürümler.
//   - Uygulanmamış her migrasyon kendi transaction'ında çalışır (hem SQLite
//     hem PostgreSQL DDL transactional → yarım kalmış "dirty" durum yok).
//   - PostgreSQL'de tüm runner mevcut pg_advisory_lock(migrateLockKey) deseni
//     altında tek bağlantıda çalışır (çoklu replika aynı taze DB'ye karşı
//     başlarsa migrasyonu sıraya sokar — bkz. pg.go migrateLockKey yorumu).
//   - Baseline: schema_migrations boş ama `agents` tablosu varsa (Faz 13
//     öncesi kurulum) 0001_init ÇALIŞTIRILMAZ, doğrudan "uygulandı"
//     işaretlenir — 0001_init bugünkü şemayla birebir aynı.
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

// migration, gömülü tek bir sıralı şema adımı.
type migration struct {
	version int
	name    string // dosya adı (.sql'siz) — schema_migrations.name'e yazılır
	body    string
}

// sqlConn, hem *sql.DB hem *sql.Conn tarafından sağlanan ortak yüzey. Runner
// PostgreSQL'de advisory-lock'lu tek bir *sql.Conn, SQLite'ta doğrudan *sql.DB
// üzerinde çalışır.
type sqlConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
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

	// Baseline: schema_migrations boş ama şema zaten varsa (Faz 13 öncesi
	// kurulum), 0001_init'i çalıştırmadan "uygulandı" işaretle. 0002+ normal
	// çalışır ve mevcut DB'yi güncel şemaya taşır.
	if len(applied) == 0 {
		legacy, err := legacySchemaPresent(ctx, c, pg)
		if err != nil {
			return err
		}
		if legacy {
			if _, err := c.ExecContext(ctx, insertStmt, 1, "0001_init", time.Now().Unix()); err != nil {
				return fmt.Errorf("0001 baseline işaretlenemedi: %w", err)
			}
			applied[1] = true
		}
	}

	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		tx, err := c.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.body); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrasyon %s: %w", m.name, err)
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

func appliedVersions(ctx context.Context, c sqlConn) (map[int]bool, error) {
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

// legacySchemaPresent, Faz 13 öncesi bir kurulum olup olmadığını `agents`
// tablosunun (Faz 1'den beri var) varlığına bakarak anlar.
func legacySchemaPresent(ctx context.Context, c sqlConn, pg bool) (bool, error) {
	q := `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'agents'`
	if pg {
		q = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'agents'`
	}
	var n int
	if err := c.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
