package store

import "time"

// AgentDomainSeen, bir agent'ın son pencerede gözlemlediği tek bir alan adı —
// L7 (TLS SNI / HTTP Host) veya DNS sorgusu kaynağından. IOC eşleştirmesi
// (internal/alert/ioc.go) bunu kullanır.
type AgentDomainSeen struct {
	AgentID   int64
	AgentName string
	Domain    string
	Process   string
	Source    string // "l7" | "dns"
	LastTs    int64
}

// RecentAgentDomains, `since`'ten beri agent'ların gördüğü (agent, domain,
// süreç, kaynak) dörtlülerini döndürür. l7_endpoints.host ve agent_dns.domain
// birleştirilir. Çok-hub kurulumunda her hub yalnızca kendi agent'larını görür.
func (s *sqlStore) RecentAgentDomains(since time.Time) ([]AgentDomainSeen, error) {
	q := `SELECT a.id, a.name, x.domain, x.process, x.source, MAX(x.ts)
		FROM (
			SELECT agent_id, host AS domain, process, ts, 'l7' AS source
				FROM l7_endpoints WHERE ts >= ? AND host <> ''
			UNION ALL
			SELECT agent_id, domain, process, ts, 'dns' AS source
				FROM agent_dns WHERE ts >= ? AND domain <> ''
		) x
		JOIN agents a ON a.id = x.agent_id
		GROUP BY a.id, a.name, x.domain, x.process, x.source`

	cut := since.Unix()
	rows, err := s.db.Query(s.q(q), cut, cut)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AgentDomainSeen{}
	for rows.Next() {
		var d AgentDomainSeen
		if err := rows.Scan(&d.AgentID, &d.AgentName, &d.Domain, &d.Process, &d.Source, &d.LastTs); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
