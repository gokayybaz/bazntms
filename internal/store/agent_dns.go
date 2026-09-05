package store

import (
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- agent DNS görünürlüğü (süreç × domain) ---

// SaveAgentDNS, agent'in dönemlik DNS gözlemlerini yazar.
func (s *sqlStore) SaveAgentDNS(agentID int64, ts int64, samples []telemetry.DNSSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO agent_dns
		(ts, agent_id, pid, process, domain, queries, responses)
		VALUES (?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, sm := range samples {
		if sm.Domain == "" || sm.Queries+sm.Responses == 0 {
			continue
		}
		if _, err := stmt.Exec(ts, agentID, sm.PID, sm.Process, sm.Domain, sm.Queries, sm.Responses); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type AgentDNSUsage struct {
	Domain    string `json:"domain"`
	Process   string `json:"process"`
	Queries   uint64 `json:"queries"`
	Responses uint64 `json:"responses"`
	AgentCnt  int    `json:"agent_count"`
}

// TopAgentDNS, dönemdeki en çok sorulan domain'leri süreç ile toplar.
// agentID 0 ise tüm filo dahildir. site boş değilse yalnızca o site'taki
// agent'lar (RBAC site scope).
func (s *sqlStore) TopAgentDNS(since time.Time, agentID int64, limit int, site string) ([]AgentDNSUsage, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	q := `SELECT domain, process, SUM(queries), SUM(responses), COUNT(DISTINCT agent_id)
		FROM agent_dns WHERE ts >= ?`
	args := []any{since.Unix()}
	if agentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if site != "" {
		q += ` AND agent_id IN (SELECT id FROM agents WHERE site = ?)`
		args = append(args, site)
	}
	q += ` GROUP BY domain, process ORDER BY SUM(queries + responses) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentDNSUsage{}
	for rows.Next() {
		var u AgentDNSUsage
		if err := rows.Scan(&u.Domain, &u.Process, &u.Queries, &u.Responses, &u.AgentCnt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
