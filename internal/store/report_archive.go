package store

// Zamanlanmış rapor teslim geçmişi (Faz 22 S22.19). Rapor içeriği dosya
// sisteminde; burada yalnız yol + meta. Şema: 0013_report_archive.

import "database/sql"

type ReportArchive struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Site        string `json:"site"`
	Days        int    `json:"days"`
	Format      string `json:"format"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	GeneratedTs int64  `json:"generated_ts"`
	DeliveredTo string `json:"delivered_to"`
	Status      string `json:"status"`
	JobID       int64  `json:"job_id"`
}

const reportArchiveCols = `id, kind, site, days, format, path, size, generated_ts, delivered_to, status, job_id`

func scanReportArchive(sc interface{ Scan(...any) error }) (ReportArchive, error) {
	var a ReportArchive
	err := sc.Scan(&a.ID, &a.Kind, &a.Site, &a.Days, &a.Format, &a.Path, &a.Size,
		&a.GeneratedTs, &a.DeliveredTo, &a.Status, &a.JobID)
	return a, err
}

func (s *sqlStore) InsertReportArchive(a ReportArchive) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO report_archive
		(kind, site, days, format, path, size, generated_ts, delivered_to, status, job_id)
		VALUES (?,?,?,?,?,?,?,?,?,?) RETURNING id`),
		a.Kind, a.Site, a.Days, a.Format, a.Path, a.Size, a.GeneratedTs,
		a.DeliveredTo, a.Status, a.JobID).Scan(&id)
	return id, err
}

// ListReportArchive, en yeni `limit` teslimi döndürür (site boş → hepsi).
func (s *sqlStore) ListReportArchive(site string, limit int) ([]ReportArchive, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + reportArchiveCols + ` FROM report_archive`
	var args []any
	if site != "" {
		q += ` WHERE site = ?`
		args = append(args, site)
	}
	q += ` ORDER BY generated_ts DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReportArchive{}
	for rows.Next() {
		a, err := scanReportArchive(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *sqlStore) ReportArchiveByID(id int64) (*ReportArchive, error) {
	a, err := scanReportArchive(s.db.QueryRow(s.q(`SELECT `+reportArchiveCols+` FROM report_archive WHERE id = ?`), id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// PruneReportArchive, generated_ts'i `before`'dan eski kayıtları siler ve
// silinen dosya yollarını döndürür (çağıran dosyaları da siler).
func (s *sqlStore) PruneReportArchive(before int64) ([]string, error) {
	rows, err := s.db.Query(s.q(`SELECT path FROM report_archive WHERE generated_ts < ?`), before)
	if err != nil {
		return nil, err
	}
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, err
		}
		paths = append(paths, p)
	}
	rows.Close()
	if _, err := s.db.Exec(s.q(`DELETE FROM report_archive WHERE generated_ts < ?`), before); err != nil {
		return nil, err
	}
	return paths, nil
}
