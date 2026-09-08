package store

// SLA hedefleri (Faz 22 S22.21). scope='global' (site='') temel; scope='site'
// bir sahaya özel geçersiz kılma. Şema: 0014_sla_targets.

import "time"

type SLATarget struct {
	Scope           string  `json:"scope"` // global | site
	Site            string  `json:"site"`
	AgentUptimePct  float64 `json:"agent_uptime_pct"`
	DeviceHealthPct float64 `json:"device_health_pct"`
	IfaceErrCeiling int64   `json:"iface_err_ceiling"`
	UpdatedTs       int64   `json:"updated_ts"`
}

// Set, boş (sıfır) alanı var mı — "hedef tanımlı değil" ayrımı için.
func (t SLATarget) Set() bool {
	return t.AgentUptimePct > 0 || t.DeviceHealthPct > 0 || t.IfaceErrCeiling > 0
}

func (s *sqlStore) ListSLATargets() ([]SLATarget, error) {
	rows, err := s.db.Query(s.q(`SELECT scope, site, agent_uptime_pct, device_health_pct, iface_err_ceiling, updated_ts
		FROM sla_targets ORDER BY scope, site`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SLATarget{}
	for rows.Next() {
		var t SLATarget
		if err := rows.Scan(&t.Scope, &t.Site, &t.AgentUptimePct, &t.DeviceHealthPct, &t.IfaceErrCeiling, &t.UpdatedTs); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SLATargetFor, bir saha için etkin hedefi döndürür: saha-özel varsa o, yoksa
// global. Hiçbiri yoksa Set()=false boş hedef.
func (s *sqlStore) SLATargetFor(site string) (SLATarget, error) {
	all, err := s.ListSLATargets()
	if err != nil {
		return SLATarget{}, err
	}
	var global SLATarget
	for _, t := range all {
		if t.Scope == "site" && t.Site == site && site != "" {
			return t, nil
		}
		if t.Scope == "global" {
			global = t
		}
	}
	return global, nil
}

func (s *sqlStore) UpsertSLATarget(t SLATarget) error {
	if t.Scope == "" {
		t.Scope = "global"
	}
	_, err := s.db.Exec(s.q(`INSERT INTO sla_targets
		(scope, site, agent_uptime_pct, device_health_pct, iface_err_ceiling, updated_ts)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(scope, site) DO UPDATE SET
			agent_uptime_pct = excluded.agent_uptime_pct,
			device_health_pct = excluded.device_health_pct,
			iface_err_ceiling = excluded.iface_err_ceiling,
			updated_ts = excluded.updated_ts`),
		t.Scope, t.Site, t.AgentUptimePct, t.DeviceHealthPct, t.IfaceErrCeiling, time.Now().Unix())
	return err
}

func (s *sqlStore) DeleteSLATarget(scope, site string) error {
	_, err := s.db.Exec(s.q(`DELETE FROM sla_targets WHERE scope = ? AND site = ?`), scope, site)
	return err
}
