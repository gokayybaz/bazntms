package store

import (
	"time"

	"github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- surec bazli trafik (Faz 2) ---

// SaveProcessTraffic, agent'in donemlik surec trafik deltalarini yazar.
func (s *sqlStore) SaveProcessTraffic(agentID int64, ts int64, samples []telemetry.ProcessTrafficSample) error {
	if len(samples) == 0 {
		return nil
	}
	defer metrics.ObserveStoreWrite("process_traffic", len(samples), time.Now())
	rows := make([][]any, 0, len(samples))
	for _, sm := range samples {
		if sm.BytesIn+sm.BytesOut == 0 {
			continue
		}
		rows = append(rows, []any{ts, agentID, sm.PID, sm.Process, sm.Proto, sm.RemoteIP, sm.Port, sm.BytesIn, sm.BytesOut})
	}
	return s.bulkInsert("process_traffic",
		[]string{"ts", "agent_id", "pid", "process", "proto", "remote_ip", "port", "bytes_in", "bytes_out"}, rows)
}

type ProcessTrafficUsage struct {
	Process  string `json:"process"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	Total    uint64 `json:"total"`
	AgentCnt int    `json:"agent_count"`
}

// TopProcessTraffic, donemdeki surec bazli trafiği toplar. agentID 0 ise
// tum agentlar dahildir. site bos degilse yalnizca o site'taki agent'lar
// (RBAC site scope).
func (s *sqlStore) TopProcessTraffic(since time.Time, agentID int64, limit int, site string) ([]ProcessTrafficUsage, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// Uzun pencerelerde (kapasite raporu 30/90g) ham `process_traffic`
	// retention'da (vars. 7g) duser — TimescaleDB modunda `process_traffic_1h`
	// continuous aggregate'ından oku (S21.12). Kısa pencerelerde ham tablo
	// (saatlik kova sınır hatası pencereye göre ihmal edilebilir hale gelir).
	src, tsCol := "process_traffic", "ts"
	if s.ts && time.Since(since) > 48*time.Hour {
		src, tsCol = "process_traffic_1h", "bucket"
	}
	q := `SELECT process, SUM(bytes_in), SUM(bytes_out), COUNT(DISTINCT agent_id)
		FROM ` + src + ` WHERE ` + tsCol + ` >= ?`
	args := []any{since.Unix()}
	if agentID > 0 {
		q += ` AND agent_id = ?`
		args = append(args, agentID)
	}
	if site != "" {
		q += ` AND agent_id IN (SELECT id FROM agents WHERE site = ?)`
		args = append(args, site)
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
