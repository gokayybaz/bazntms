package store

import (
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- surec bazli trafik (Faz 2) ---

// SaveProcessTraffic, agent'in donemlik surec trafik deltalarini yazar.
func (s *sqlStore) SaveProcessTraffic(agentID int64, ts int64, samples []telemetry.ProcessTrafficSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO process_traffic
		(ts, agent_id, pid, process, proto, remote_ip, port, bytes_in, bytes_out)
		VALUES (?,?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, sm := range samples {
		if sm.BytesIn+sm.BytesOut == 0 {
			continue
		}
		if _, err := stmt.Exec(ts, agentID, sm.PID, sm.Process, sm.Proto, sm.RemoteIP, sm.Port, sm.BytesIn, sm.BytesOut); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type ProcessTrafficUsage struct {
	Process  string `json:"process"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	Total    uint64 `json:"total"`
	AgentCnt int    `json:"agent_count"`
}

// TopProcessTraffic, donemdeki surec bazli trafiği toplar. agentID 0 ise
// tum agentlar dahildir.
func (s *sqlStore) TopProcessTraffic(since time.Time, agentID int64, limit int) ([]ProcessTrafficUsage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := `SELECT process, SUM(bytes_in), SUM(bytes_out), COUNT(DISTINCT agent_id)
		FROM process_traffic WHERE ts >= ?`
	args := []any{since.Unix()}
	if agentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	q += ` GROUP BY process ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcessTrafficUsage{}
	for rows.Next() {
		var p ProcessTrafficUsage
		if err := rows.Scan(&p.Process, &p.BytesIn, &p.BytesOut, &p.AgentCnt); err != nil {
			return nil, err
		}
		p.Total = p.BytesIn + p.BytesOut
		out = append(out, p)
	}
	return out, rows.Err()
}
