package store

// Faz 13 S13.2: migrasyon runner testleri (SQLite). Postgres tarafı
// pg_integration_test.go içindeki testcontainer testleriyle kapsanır.

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// migVersions, schema_migrations tablosundaki sürümleri sıralı döndürür.
func migVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("schema_migrations sorgu: %v", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	return out
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("tablo kontrolü: %v", err)
	}
	return n > 0
}

// TestMigrateFreshDB, boş bir DB'de 0001_init çalışır ve sürüm 1 işaretlenir.
func TestMigrateFreshDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()
	db := st.(*sqlStore).db

	if got := migVersions(t, db); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("beklenen [1 2], alınan %v", got)
	}
	for _, tbl := range []string{"agents", "devices", "users", "isms_soa", "schema_migrations"} {
		if !tableExists(t, db, tbl) {
			t.Fatalf("%s tablosu oluşmadı", tbl)
		}
	}
}

// TestMigrateIdempotent, ikinci Open() hiçbir migrasyonu tekrar çalıştırmaz.
func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idem.db")
	st1, err := Open(path)
	if err != nil {
		t.Fatalf("ilk Open: %v", err)
	}
	st1.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("ikinci Open: %v", err)
	}
	defer st2.Close()
	if got := migVersions(t, st2.(*sqlStore).db); len(got) != 2 {
		t.Fatalf("ikinci açılışta sürüm listesi değişti: %v", got)
	}
}

// legacyDB, Faz 13 öncesi bir kurulumu taklit eder: 0001_init gövdesini
// (bugünkü şemayla birebir) elle uygular, schema_migrations tablosunu bırakmaz.
// extra ile ek DDL (ör. kolon düşürme) çalıştırılabilir.
func legacyDB(t *testing.T, path string, extra ...string) {
	t.Helper()
	migs, err := loadMigrations("sqlite")
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Exec(migs[0].body); err != nil {
		t.Fatalf("eski şema: %v", err)
	}
	for _, s := range extra {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("ek DDL %q: %v", s, err)
		}
	}
}

// TestMigrateLegacyBaseline, Faz 13 öncesi bir DB'yi (schema_migrations yok)
// hatasız açar; 0001_init idempotent çalışır, sürüm 1 kaydedilir, eski veri korunur.
func TestMigrateLegacyBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB(t, path,
		`INSERT INTO agents (name, site, token_hash, first_seen, last_seen) VALUES ('eski-agent', '', 'h1', 1, 1)`)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("eski DB açılamadı: %v", err)
	}
	defer st.Close()
	db := st.(*sqlStore).db

	got := migVersions(t, db)
	if len(got) < 1 || got[0] != 1 {
		t.Fatalf("sürüm 1 kaydedilmedi: %v", got)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM schema_migrations WHERE version=1`).Scan(&name); err != nil {
		t.Fatalf("sürüm 1 adı: %v", err)
	}
	if name != "0001_init" {
		t.Fatalf("sürüm 1 adı beklenmedik: %q", name)
	}
	var agentName string
	if err := db.QueryRow(`SELECT name FROM agents WHERE token_hash='h1'`).Scan(&agentName); err != nil {
		t.Fatalf("eski veri kayboldu: %v", err)
	}
	if agentName != "eski-agent" {
		t.Fatalf("eski agent adı bozuldu: %q", agentName)
	}
}

// TestMigrate0002LegacyColumns, Faz 8 öncesi bir devices tablosunda (vendor /
// api_* / vdom / site kolonları yok) ve source_ip'siz syslog_events'te 0002
// migrasyonunun eksik kolonları eklediğini doğrular (S13.3 — eski
// ensureDeviceColumns/ensureSyslogColumns davranışı).
func TestMigrate0002LegacyColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefaz8.db")
	legacyDB(t, path,
		`ALTER TABLE devices DROP COLUMN vendor`,
		`ALTER TABLE devices DROP COLUMN api_url`,
		`ALTER TABLE devices DROP COLUMN api_token_enc`,
		`ALTER TABLE devices DROP COLUMN api_verify_tls`,
		`ALTER TABLE devices DROP COLUMN vdom`,
		`ALTER TABLE devices DROP COLUMN site`,
		`ALTER TABLE syslog_events DROP COLUMN source_ip`,
	)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()
	db := st.(*sqlStore).db

	for _, tc := range []struct{ table, col string }{
		{"devices", "vendor"}, {"devices", "api_url"}, {"devices", "api_token_enc"},
		{"devices", "api_verify_tls"}, {"devices", "vdom"}, {"devices", "site"},
		{"syslog_events", "source_ip"},
	} {
		set, err := columnSet(t.Context(), db, false, tc.table)
		if err != nil {
			t.Fatalf("columnSet %s: %v", tc.table, err)
		}
		if !set[tc.col] {
			t.Fatalf("%s.%s eklenmedi", tc.table, tc.col)
		}
	}
	if got := migVersions(t, db); len(got) != 2 || got[1] != 2 {
		t.Fatalf("0002 uygulanmadı: %v", got)
	}

	// idempotent: ikinci açılış 0002'yi tekrar çalıştırmamalı (kolon zaten var)
	st.Close()
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("ikinci Open: %v", err)
	}
	st2.Close()
}

// TestLoadMigrations, gömülü migrasyon setinin her iki dialect için de
// tutarlı yüklendiğini doğrular (dosya adı biçimi, sıralama, çift sürüm yok).
func TestLoadMigrations(t *testing.T) {
	for _, d := range []string{"sqlite", "postgres"} {
		migs, err := loadMigrations(d)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if len(migs) == 0 {
			t.Fatalf("%s: migrasyon bulunamadı", d)
		}
		if migs[0].version != 1 || migs[0].name != "0001_init" {
			t.Fatalf("%s: ilk migrasyon 0001_init değil: %+v", d, migs[0])
		}
		for i := 1; i < len(migs); i++ {
			if migs[i].version <= migs[i-1].version {
				t.Fatalf("%s: sürümler artan sırada değil: %v / %v", d, migs[i-1].version, migs[i].version)
			}
		}
	}
}
