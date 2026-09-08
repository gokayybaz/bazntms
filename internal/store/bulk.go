package store

import (
	"database/sql"
	"strings"
)

// Toplu yazım yardımcıları (S21.8). Ölçek koşusunda ingest darboğazı satır
// başına bir round-trip'ti (`tx.Prepare` + döngüde `stmt.Exec`; PostgreSQL'de
// prepared statement pipeline'lanmaz). PG yolunda artık chunk başına tek
// çok-satırlı `INSERT ... VALUES (...),(...)` gönderilir. SQLite yolu
// değişmez — satır-başına prepared exec (modernc sürücüsü zaten hızlı, ve
// dev modu ölçek hedefi değil).

// execer, *sql.DB ve *sql.Tx tarafından sağlanır.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// pgMaxParams, PostgreSQL genişletilmiş protokol parametre sınırının (65535)
// altında güvenli bir tavan.
const pgMaxParams = 60000

// insertRows, verilen satırları tabloya yazar. ex bir tx veya db olabilir.
// PG: chunk'lı çok-satırlı VALUES. SQLite: satır-başına prepared exec.
func (s *sqlStore) insertRows(ex execer, table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	colList := strings.Join(cols, ", ")

	if !s.pg {
		stmt, err := prepareInsert(ex, s.q("INSERT INTO "+table+" ("+colList+") VALUES ("+placeholders(len(cols))+")"))
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range rows {
			if _, err := stmt.Exec(r...); err != nil {
				return err
			}
		}
		return nil
	}

	maxRows := pgMaxParams / len(cols)
	if maxRows < 1 {
		maxRows = 1
	}
	rowPH := "(" + placeholders(len(cols)) + ")"
	for start := 0; start < len(rows); start += maxRows {
		end := start + maxRows
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		var b strings.Builder
		b.Grow(64 + len(table) + len(colList) + len(chunk)*(len(rowPH)+1))
		b.WriteString("INSERT INTO ")
		b.WriteString(table)
		b.WriteString(" (")
		b.WriteString(colList)
		b.WriteString(") VALUES ")
		args := make([]any, 0, len(chunk)*len(cols))
		for i, r := range chunk {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(rowPH)
			args = append(args, r...)
		}
		if _, err := ex.Exec(s.q(b.String()), args...); err != nil {
			return err
		}
	}
	return nil
}

// bulkInsert, kendi transaction'ında insertRows çalıştırır.
func (s *sqlStore) bulkInsert(table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.insertRows(tx, table, cols, rows); err != nil {
		return err
	}
	return tx.Commit()
}

// prepareInsert, execer'ın Prepare'ini (tx veya db) çağırır.
func prepareInsert(ex execer, query string) (*sql.Stmt, error) {
	type preparer interface {
		Prepare(string) (*sql.Stmt, error)
	}
	return ex.(preparer).Prepare(query)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
