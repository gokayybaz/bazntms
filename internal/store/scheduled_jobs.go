package store

// Hub-içi zamanlanmış işler (Faz 22 S22.18). Lider-kapılı bir zamanlayıcı
// (internal/scheduler) enabled + next_run_ts <= now olan işleri çalıştırır.
// Şema: migrations/*/0012_scheduled_jobs.sql.

type ScheduledJob struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Spec       string `json:"spec"`
	Payload    string `json:"payload_json"`
	Enabled    bool   `json:"enabled"`
	LastRunTs  int64  `json:"last_run_ts"`
	NextRunTs  int64  `json:"next_run_ts"`
	LastStatus string `json:"last_status"`
	CreatedBy  string `json:"created_by"`
	CreatedTs  int64  `json:"created_ts"`
}

const scheduledJobCols = `id, kind, spec, payload_json, enabled, last_run_ts, next_run_ts, last_status, created_by, created_ts`

func scanScheduledJob(sc interface{ Scan(...any) error }) (ScheduledJob, error) {
	var j ScheduledJob
	err := sc.Scan(&j.ID, &j.Kind, &j.Spec, &j.Payload, &j.Enabled, &j.LastRunTs,
		&j.NextRunTs, &j.LastStatus, &j.CreatedBy, &j.CreatedTs)
	return j, err
}

func (s *sqlStore) CreateScheduledJob(j ScheduledJob) (int64, error) {
	if j.Payload == "" {
		j.Payload = "{}"
	}
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO scheduled_jobs
		(kind, spec, payload_json, enabled, next_run_ts, created_by, created_ts)
		VALUES (?,?,?,?,?,?,?) RETURNING id`),
		j.Kind, j.Spec, j.Payload, btoi(j.Enabled), j.NextRunTs, j.CreatedBy, j.CreatedTs).Scan(&id)
	return id, err
}

func (s *sqlStore) ListScheduledJobs() ([]ScheduledJob, error) {
	return s.queryScheduledJobs(`SELECT ` + scheduledJobCols + ` FROM scheduled_jobs ORDER BY id`)
}

// DueScheduledJobs, çalıştırılması gereken (enabled + next_run_ts <= now) işler.
func (s *sqlStore) DueScheduledJobs(now int64) ([]ScheduledJob, error) {
	return s.queryScheduledJobs(`SELECT `+scheduledJobCols+` FROM scheduled_jobs
		WHERE enabled = 1 AND next_run_ts > 0 AND next_run_ts <= ? ORDER BY id`, now)
}

func (s *sqlStore) queryScheduledJobs(query string, args ...any) ([]ScheduledJob, error) {
	rows, err := s.db.Query(s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScheduledJob{}
	for rows.Next() {
		j, err := scanScheduledJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// MarkScheduledJobRun, bir işin koşumunu kaydeder: son koşum + durum + bir
// sonraki koşum zamanı.
func (s *sqlStore) MarkScheduledJobRun(id, ranAt, nextRun int64, status string) error {
	_, err := s.db.Exec(s.q(`UPDATE scheduled_jobs
		SET last_run_ts = ?, next_run_ts = ?, last_status = ? WHERE id = ?`),
		ranAt, nextRun, status, id)
	return err
}

func (s *sqlStore) SetScheduledJobEnabled(id int64, enabled bool, nextRun int64) error {
	_, err := s.db.Exec(s.q(`UPDATE scheduled_jobs SET enabled = ?, next_run_ts = ? WHERE id = ?`),
		btoi(enabled), nextRun, id)
	return err
}

func (s *sqlStore) DeleteScheduledJob(id int64) error {
	_, err := s.db.Exec(s.q(`DELETE FROM scheduled_jobs WHERE id = ?`), id)
	return err
}
