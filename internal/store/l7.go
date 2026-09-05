package store

import (
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- L7 uygulama gorunurlugu (surec × SNI/Host) ---

// SaveL7, agent'in donemlik L7 gozlemlerini yazar.
func (s *sqlStore) SaveL7(agentID int64, ts int64, samples []telemetry.L7Sample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO l7_endpoints
		(ts, agent_id, pid, process, kind, host, remote_ip, bytes, hits)
		VALUES (?,?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, sm := range samples {
		if sm.Host == "" || sm.Count == 0 {
			continue
		}
		if _, err := stmt.Exec(ts, agentID, sm.PID, sm.Process, sm.Kind, sm.Host, sm.RemoteIP, sm.Bytes, sm.Count); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type L7Usage struct {
	Host     string `json:"host"`
	Kind     string `json:"kind"`
	Process  string `json:"process"`
	Bytes    uint64 `json:"bytes"`
	Hits     uint64 `json:"hits"`
	AgentCnt int    `json:"agent_count"`
}

// TopL7, donemdeki en cok gorulen alan adlarini (host) surec+tur ile toplar.
// agentID 0 ise tum filo dahildir. site bos degilse yalnizca o site'taki
// agent'lar (RBAC site scope).
func (s *sqlStore) TopL7(since time.Time, agentID int64, limit int, site string) ([]L7Usage, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	q := `SELECT host, kind, process, SUM(bytes), SUM(hits), COUNT(DISTINCT agent_id)
		FROM l7_endpoints WHERE ts >= ?`
	args := []any{since.Unix()}
	if agentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if site != "" {
		q += ` AND agent_id IN (SELECT id FROM agents WHERE site = ?)`
		args = append(args, site)
	}
	q += ` GROUP BY host, kind, process ORDER BY SUM(hits) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []L7Usage{}
	for rows.Next() {
		var u L7Usage
		if err := rows.Scan(&u.Host, &u.Kind, &u.Process, &u.Bytes, &u.Hits, &u.AgentCnt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
