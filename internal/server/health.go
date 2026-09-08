package server

// Ağ sağlık skoru ucu (Faz 25-A) — deterministik, açıklanabilir. Motor:
// internal/health. ~30 sn önbellek (pano her yenilemede çağırır).

import (
	"net/http"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/health"
	"github.com/gokayybaz/bazntms/internal/store"
)

type healthCache struct {
	mu     sync.Mutex
	score  health.Score
	at     time.Time
	loaded bool
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.hcache.mu.Lock()
	if s.hcache.loaded && time.Since(s.hcache.at) < 30*time.Second {
		sc := s.hcache.score
		s.hcache.mu.Unlock()
		writeJSON(w, sc)
		return
	}
	s.hcache.mu.Unlock()

	sc := health.Compute(s.collectHealthInputs())

	s.hcache.mu.Lock()
	s.hcache.score, s.hcache.at, s.hcache.loaded = sc, time.Now(), true
	s.hcache.mu.Unlock()
	writeJSON(w, sc)
}

// collectHealthInputs, sağlık sinyallerini store'dan toplar. Bir sorgu
// başarısız olursa o sinyal 0 kalır (sağlık ucu asla 500 vermez).
func (s *Server) collectHealthInputs() health.Inputs {
	var in health.Inputs
	now := time.Now()
	onlineWin := 2 * time.Duration(s.telemetryInterval) * time.Second

	if fs, err := s.store.FleetSummary(onlineWin); err == nil {
		in.AgentsTotal = fs.AgentsTotal
		in.AgentsOnline = fs.AgentsOnline
	}
	// bayat: online değil AND son görülme > 1 saat (arşivlenmemiş ama ölü)
	if agents, err := s.store.ListAgents(onlineWin, ""); err == nil {
		staleCut := now.Add(-time.Hour).Unix()
		for _, a := range agents {
			if !a.Online && a.LastSeen > 0 && a.LastSeen < staleCut {
				in.StaleAgents++
			}
		}
	}

	if devices, err := s.store.ListDevices(""); err == nil {
		for _, d := range devices {
			if !d.Enabled {
				continue
			}
			in.DevicesTotal++
			if d.LastPoll > 0 && now.Unix()-d.LastPoll < int64(3*d.PollSeconds) && d.LastError == "" {
				in.DevicesOnline++
			}
		}
	}

	if disc, errs, err := s.store.FleetIfaceHealth(now.Add(-24*time.Hour), ""); err == nil {
		in.IfaceDiscards = disc
		in.IfaceErrors = errs
	}

	if evs, _, err := s.store.QueryAlertEvents(store.AlertEventFilter{Severity: "crit", State: "firing", Limit: 200}); err == nil {
		in.CritAlertsOpen = len(evs)
	}

	if incs, err := s.store.RecentOpenIncidents(now.Add(-24 * time.Hour)); err == nil {
		in.OpenIncidents = len(incs)
		for _, i := range incs {
			if i.RiskScore > in.MaxIncidentRisk {
				in.MaxIncidentRisk = i.RiskScore
			}
		}
	}

	return in
}
