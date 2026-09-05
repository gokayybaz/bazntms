// Komut: go run ./tools/dbcheck -db <path|dsn> [-ping]
//
// DB yükseltme testi için minimal yardımcı (Faz 13 S13.4, bkz.
// .github/workflows/ci.yml → db-upgrade-test). CI'da iki kez çağrılır:
//  1. önceki release worktree'sinde  → eski kod ile şema oluşturur
//  2. HEAD worktree'sinde -ping ile   → yeni kod ile aynı DB'yi açar,
//     migrasyonları uygular, Ping atar ve uygulanmış sürümleri yazar
//
// Eski release worktree'sinde bu dosya bulunmadığından CI onu oraya kopyalar;
// yalnızca internal/store'a bağımlıdır (modül yolu her iki ağaçta da aynı).
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gokayybaz/bazntms/internal/store"
)

func main() {
	db := flag.String("db", "", "SQLite dosyası veya postgres:// DSN")
	ping := flag.Bool("ping", false, "aç + Ping + schema_migrations dök (yükseltme adımı)")
	flag.Parse()

	if *db == "" {
		fmt.Fprintln(os.Stderr, "dbcheck: -db zorunlu")
		os.Exit(2)
	}

	st, err := store.Open(*db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store.Open:", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	if !*ping {
		fmt.Println("şema oluşturuldu:", *db)
		return
	}

	if err := st.Ping(); err != nil {
		fmt.Fprintln(os.Stderr, "Ping:", err)
		os.Exit(1)
	}
	dumpMigrations(*db)
	fmt.Println("yükseltme başarılı:", *db)
}

// dumpMigrations, schema_migrations tablosunu ayrı bir bağlantıdan okur
// (Store arayüzü bu tabloyu açmaz).
func dumpMigrations(dsn string) {
	driver := "sqlite"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driver = "pgx"
	}
	raw, err := sql.Open(driver, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump aç:", err)
		os.Exit(1)
	}
	defer raw.Close()
	rows, err := raw.Query(`SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump sorgu:", err)
		os.Exit(1)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var v int
		var name string
		if err := rows.Scan(&v, &name); err != nil {
			fmt.Fprintln(os.Stderr, "dump scan:", err)
			os.Exit(1)
		}
		fmt.Printf("  uygulandı: %d %s\n", v, name)
		n++
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "schema_migrations boş — migrasyon kaydı yok")
		os.Exit(1)
	}
}
