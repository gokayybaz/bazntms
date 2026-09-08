package alert

// SLA hedef ihlali uyarısı (Faz 22 S22.21). store.SLATargetFor ile tanımlı
// hedefler (global + per-site) periyodik değerlendirilir; ihlal → "sla_breach"
// (severity crit). Dedup açık ihlali sessizce tazeler; düzelince TTL
// otomatik-çözülme kapatır.

import (
	"fmt"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// checkSLA, global + agent'ların bulunduğu her sahayı değerlendirir.
func (m *Manager) checkSLA() {
	m.evalSLA("")
	agents, err := m.st.ListAgents(365*24*time.Hour, "")
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, a := range agents {
		if a.Site != "" && !seen[a.Site] {
			seen[a.Site] = true
			m.evalSLA(a.Site)
		}
	}
}

func (m *Manager) evalSLA(site string) {
	tgt, err := m.st.SLATargetFor(site)
	if err != nil || !tgt.Set() {
		return
	}
	scope := "Filo geneli"
	if site != "" {
		scope = "Saha " + site
	}
	base := "sla:" + site
	fire := func(sub, msg string) {
		m.fireCtx("sla_breach", base+":"+sub, scope+": "+msg, fireOpts{Site: site, Severity: "crit"})
	}

	if tgt.AgentUptimePct > 0 {
		agents, _ := m.st.ListAgents(2*time.Minute, site)
		if n := len(agents); n > 0 {
			online := 0
			for _, a := range agents {
				if a.Online {
					online++
				}
			}
			if up := 100 * float64(online) / float64(n); up < tgt.AgentUptimePct {
				fire("uptime", fmt.Sprintf("agent uptime %.1f%% — hedef %.1f%% (%d/%d online)", up, tgt.AgentUptimePct, online, n))
			}
		}
	}

	if tgt.DeviceHealthPct > 0 {
		devices, _ := m.st.ListDevices(site)
		now := time.Now().Unix()
		dtot, dok := 0, 0
		for _, d := range devices {
			if !d.Enabled {
				continue
			}
			dtot++
			fresh := d.LastPoll > 0 && now-d.LastPoll < int64(3*d.PollSeconds)
			if d.LastError == "" && fresh {
				dok++
			}
		}
		if dtot > 0 {
			if h := 100 * float64(dok) / float64(dtot); h < tgt.DeviceHealthPct {
				fire("device", fmt.Sprintf("cihaz sağlığı %.1f%% — hedef %.1f%% (%d/%d)", h, tgt.DeviceHealthPct, dok, dtot))
			}
		}
	}

	if tgt.IfaceErrCeiling > 0 {
		disc, errs, _ := m.st.FleetIfaceHealth(time.Now().Add(-24*time.Hour), site)
		if tot := int64(disc + errs); tot > tgt.IfaceErrCeiling {
			fire("iface", fmt.Sprintf("24 saatte %d arayüz iskarta+hata — tavan %d", tot, tgt.IfaceErrCeiling))
		}
	}
}

// SLA hedef CRUD (server için).
func (m *Manager) ListSLATargets() ([]store.SLATarget, error) { return m.st.ListSLATargets() }
func (m *Manager) UpsertSLATarget(t store.SLATarget) error    { return m.st.UpsertSLATarget(t) }
func (m *Manager) DeleteSLATarget(scope, site string) error   { return m.st.DeleteSLATarget(scope, site) }
