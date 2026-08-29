//go:build !windows

package store

// Faz 0 (0.5): testcontainers entegrasyon testi — Faz 4'te TimescaleDB'ye
// gecis icin hazirlik. Docker yoksa (yerel ortam) test skip edilir; CI
// ubuntu runner'inda gercekten calisir.

import (
	"context"
	"database/sql"
	"runtime"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresHarness(t *testing.T) {
	if testing.Short() {
		t.Skip("short mod")
	}
	if runtime.GOOS == "windows" {
		t.Skip("windows CI'da docker yok")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "bazntms",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "bazntms",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("docker kullanilamadi, atlanıyor: %v", err)
	}
	defer ctr.Terminate(ctx)

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	dsn := "postgres://bazntms:test@" + host + ":" + port.Port() + "/bazntms?sslmode=disable"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("pgx acilamadi: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Faz 4 semasinin on habercisi: basit hypertable benzeri tablo yaz/oku
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS probe (
		id SERIAL PRIMARY KEY, ts TIMESTAMPTZ NOT NULL, val DOUBLE PRECISION NOT NULL)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO probe (ts, val) VALUES ($1, $2)`, now, 42.5); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var got time.Time
	var val float64
	if err := db.QueryRowContext(ctx, `SELECT ts, val FROM probe ORDER BY id DESC LIMIT 1`).Scan(&got, &val); err != nil {
		t.Fatalf("select: %v", err)
	}
	if val != 42.5 || got.Unix() != now.Unix() {
		t.Fatalf("yuvarlama hatali: %v %v", got, val)
	}
}
