package store

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// --- agent registry (Faz 1) ---

type Agent struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Site            string `json:"site"`
	TokenHash       string `json:"-"`
	FirstSeen       int64  `json:"first_seen"`
	LastSeen        int64  `json:"last_seen"`
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	RemoteIP        string `json:"remote_ip"`
}

func TokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *sqlStore) RegisterAgent(a Agent) (int64, error) {
	now := time.Now().Unix()
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO agents (name, site, token_hash, first_seen, last_seen, version, protocol_version, remote_ip)
		VALUES (?,?,?,?,?,?,?,?) RETURNING id`),
		a.Name, a.Site, a.TokenHash, now, now, a.Version, a.ProtocolVersion, a.RemoteIP).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *sqlStore) AgentByTokenHash(hash string) (*Agent, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, site, first_seen, last_seen, version, protocol_version, remote_ip
		FROM agents WHERE token_hash = ?`), hash)
	var a Agent
	a.TokenHash = hash
	err := row.Scan(&a.ID, &a.Name, &a.Site, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// TouchAgent, telemetri/heartbeat'te cagrılır: son gorulme + meta gunceller.
func (s *sqlStore) TouchAgent(id int64, version, remoteIP string) error {
	_, err := s.db.Exec(s.q(`UPDATE agents SET last_seen = ?, version = ?, remote_ip = ? WHERE id = ?`),
		time.Now().Unix(), version, remoteIP, id)
	return err
}

func (s *sqlStore) SaveIfaceSamples(agentID int64, ts int64, samples []telemetry.InterfaceSample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO agent_iface_samples
		(agent_id, ts, name, rx_bytes, tx_bytes, rx_packets, tx_packets) VALUES (?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, sm := range samples {
		if _, err := stmt.Exec(agentID, ts, sm.Name, sm.RxBytes, sm.TxBytes, sm.RxPackets, sm.TxPackets); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) ReplaceConnLatest(agentID int64, conns []telemetry.ConnectionSample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(s.q(`DELETE FROM agent_conn_latest WHERE agent_id = ?`), agentID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(s.q(`INSERT INTO agent_conn_latest
		(agent_id, proto, local_addr, remote_addr, status, pid, process) VALUES (?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range conns {
		if _, err := stmt.Exec(agentID, c.Proto, c.LocalAddr, c.RemoteAddr, c.Status, c.PID, c.Process); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AgentRate, son iki ornekten hesaplanan arayuz verim bilgisidir.
type AgentRate struct {
	Name     string  `json:"name"`
	RxBps    float64 `json:"rx_bps"`
	TxBps    float64 `json:"tx_bps"`
	RxBytes  uint64  `json:"rx_bytes"`
	TxBytes  uint64  `json:"tx_bytes"`
	LastSeen int64   `json:"last_seen"`
}

type AgentWithRates struct {
	Agent
	Online bool        `json:"online"`
	Rates  []AgentRate `json:"rates"`
	Conns  int         `json:"conns"`
}

// ListAgents, filo gorunumu: online durumu + son orneklerden hesaplanmis verimler.
func (s *sqlStore) ListAgents(onlineWindow time.Duration) ([]AgentWithRates, error) {
	rows, err := s.db.Query(s.q(`SELECT id, name, site, first_seen, last_seen, version, protocol_version, remote_ip
		FROM agents ORDER BY last_seen DESC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentWithRates
	now := time.Now().Unix()
	for rows.Next() {
		var a AgentWithRates
		if err := rows.Scan(&a.ID, &a.Name, &a.Site, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP); err != nil {
			return nil, err
		}
		a.Online = now-a.LastSeen <= int64(onlineWindow.Seconds())
		out = append(out, a)
	}
	if out == nil {
		out = []AgentWithRates{}
	}

	// her agent icin son orneklerden arayuz verimleri (ilk/son karsilastirma)
	for i := range out {
		rows2, err := s.db.Query(s.q(`SELECT name, rx_bytes, tx_bytes, ts FROM (
			SELECT name, rx_bytes, tx_bytes, ts FROM agent_iface_samples
			WHERE agent_id = ? ORDER BY ts DESC LIMIT 400) sq ORDER BY ts ASC`), out[i].ID)
		if err != nil {
			continue
		}
		type sample struct {
			rx, tx uint64
			ts     int64
		}
		first := map[string]sample{}
		last := map[string]sample{}
		var order []string
		for rows2.Next() {
			var name string
			var sm sample
			if err := rows2.Scan(&name, &sm.rx, &sm.tx, &sm.ts); err != nil {
				break
			}
			if _, ok := first[name]; !ok {
				first[name] = sm
				order = append(order, name)
			}
			last[name] = sm
		}
		rows2.Close()

		for _, name := range order {
			f, l := first[name], last[name]
			if l.ts <= f.ts {
				continue
			}
			out[i].Rates = append(out[i].Rates, AgentRate{
				Name:     name,
				RxBps:    float64(l.rx-f.rx) / float64(l.ts-f.ts),
				TxBps:    float64(l.tx-f.tx) / float64(l.ts-f.ts),
				RxBytes:  l.rx,
				TxBytes:  l.tx,
				LastSeen: l.ts,
			})
		}
		if err := s.db.QueryRow(s.q(`SELECT COUNT(*) FROM agent_conn_latest WHERE agent_id = ?`), out[i].ID).Scan(&out[i].Conns); err != nil {
			out[i].Conns = 0
		}
	}
	return out, nil
}

func (s *sqlStore) LatestAgentConnections(agentID int64) []telemetry.ConnectionSample {
	rows, err := s.db.Query(s.q(`SELECT proto, local_addr, remote_addr, status, pid, process
		FROM agent_conn_latest WHERE agent_id = ? ORDER BY process, local_addr`), agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []telemetry.ConnectionSample
	for rows.Next() {
		var c telemetry.ConnectionSample
		if err := rows.Scan(&c.Proto, &c.LocalAddr, &c.RemoteAddr, &c.Status, &c.PID, &c.Process); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (s *sqlStore) AgentByID(id int64) (*Agent, error) {
	row := s.db.QueryRow(s.q(`SELECT id, name, site, token_hash, first_seen, last_seen, version, protocol_version, remote_ip
		FROM agents WHERE id = ?`), id)
	var a Agent
	err := row.Scan(&a.ID, &a.Name, &a.Site, &a.TokenHash, &a.FirstSeen, &a.LastSeen, &a.Version, &a.ProtocolVersion, &a.RemoteIP)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *sqlStore) DeleteAgent(id int64) error {
	for _, q := range []string{
		`DELETE FROM agents WHERE id = ?`,
		`DELETE FROM agent_iface_samples WHERE agent_id = ?`,
		`DELETE FROM agent_conn_latest WHERE agent_id = ?`,
	} {
		if _, err := s.db.Exec(s.q(q), id); err != nil {
			return err
		}
	}
	return nil
}
