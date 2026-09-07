// Package store, hub'in kalici veri katmanidir (Faz 4.1).
//
// Store arayuzunun iki arka ucu vardir; Open(path) DSN semasindan secim yapar:
//   - SQLite (dev modu): path bir dosya yolu (modernc.org/sqlite, CGO'suz)
//   - PostgreSQL/TimescaleDB (olcek modu): path postgres:// veya postgresql://
//     ile baslar (pgx stdlib driver); TimescaleDB kuruluysa hypertable,
//     continuous aggregate ve retention politikalari otomatik acilir (Faz 4.3)
//
// Zaman kolonlari her iki dialect'te de unix saniye (BIGINT/INTEGER) tutulur;
// boylece sorgu mantigi dialect'ten bagimsizdir. Sorgu metinleri '?' yer
// tutucusuyla yazilir; PostgreSQL'e gonderilirken $n'ye cevrilir.
//
// Dosya duzeni: Store arayuzu alan bazli alt arayuzlerin birlesimidir
// (interfaces.go). Implementasyon (sqlStore metotlari) alan basina ayri
// dosyalarda: queries.go (yakalama + dashboard), alerts.go, agents.go,
// devices.go, topology.go, users.go, compliance.go, isms.go, fortigate.go,
// fleet_report.go. Sema migrasyonlari: migrate.go + migrations/.
package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // postgres:// DSN icin driver kaydi
	_ "modernc.org/sqlite"             // SQLite (dev modu) driver kaydi
)

// sqlStore, Store arayuzunun tek somut gerceklemesidir; SQLite ve PostgreSQL
// arasindaki fark (driver, yer tutucu, id dondurme) dialect kontroluyle
// yonetilir.
type sqlStore struct {
	db *sql.DB
	pg bool // PostgreSQL/TimescaleDB modu
	ts bool // TimescaleDB eklentisi aktif (pg modunda anlamlı)

	auditMu sync.Mutex // audit hash-zinciri tutarliligi (Faz 5.3)

	complianceMu sync.Mutex // 5651 log zinciri tutarliligi (Faz 9.1)
}

// Open, veri deposunu acar: DSN postgres:// ile basliyorsa PostgreSQL
// (pgx), aksi halde SQLite dosyasi kullanilir.
func Open(path string) (Store, error) {
	if strings.HasPrefix(path, "postgres://") || strings.HasPrefix(path, "postgresql://") {
		return openPostgres(path)
	}
	return openSQLite(path)
}

func openSQLite(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	if err := runMigrations(db, false); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite migrasyon: %w", err)
	}
	s := &sqlStore{db: db}
	if err := s.seedIsmsSoa(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("SoA seed: %w", err)
	}
	return s, nil
}

func openPostgres(dsn string) (Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// yuksek eszamanli ingest (5000 agent @ 30sn ≈ 170 ist/sn) icin havuz siniri
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(time.Hour)
	if err := runMigrations(db, true); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres migrasyon: %w", err)
	}
	s := &sqlStore{db: db, pg: true}
	if err := s.seedIsmsSoa(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("SoA seed: %w", err)
	}
	s.ts = setupTimescale(db) // best-effort; yoksa duz PostgreSQL modu
	return s, nil
}

// q, sorgu metnini dialect'e uyarlar: SQLite '?' yer tutucusu kullanir,
// PostgreSQL $n bekler.
func (s *sqlStore) q(query string) string {
	if !s.pg {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullI64, nullable INTEGER kolonuna yazarken *int64 → driver değeri.
func nullI64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// i64ptr, nullable INTEGER kolonundan okurken sql.NullInt64 → *int64.
func i64ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func (s *sqlStore) Close() error { return s.db.Close() }

func (s *sqlStore) Ping() error { return s.db.Ping() }
