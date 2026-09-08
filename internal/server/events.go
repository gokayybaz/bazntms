package server

// Normalleştirilmiş olay akışı ucu (Faz 24-A) — bkz. docs/decisions/0010.
// Kaynak tablolar üzerinde birleşik okuma; yeni yazma hattı yok.

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// handleEvents, GET /api/v1/events?type=&agent_id=&device=&since_min=&before=&limit=
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var types []string
	if t := q.Get("type"); t != "" {
		types = strings.Split(t, ",")
	}

	sinceMin, _ := strconv.Atoi(q.Get("since_min"))
	if sinceMin <= 0 || sinceMin > 60*24*7 {
		sinceMin = 60
	}
	agentID, _ := strconv.ParseInt(q.Get("agent_id"), 10, 64)
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))

	// RBAC site scope: site-kısıtlı kimlik yalnız kendi sahasının agent'ını
	// filtreleyebilir; device filtresi site kapsamı dışındaysa reddet.
	site := SiteScope(identityFromCtx(r))
	if agentID > 0 && site != "" {
		if a, err := s.store.AgentByID(agentID); err != nil || a.Site != site {
			http.Error(w, "agent bulunamadi", http.StatusNotFound)
			return
		}
	}

	evs, next, err := s.store.QueryEvents(store.EventFilter{
		Types:   types,
		AgentID: agentID,
		Device:  q.Get("device"),
		Since:   time.Now().Add(-time.Duration(sinceMin) * time.Minute),
		Before:  before,
		Limit:   limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// site-kısıtlı kimlik: device-kaynaklı olayları görünür cihazlara daralt
	// (agent-kaynaklı olaylar zaten agentID filtresi ya da aşağıdaki süzgeçle).
	if site != "" {
		evs = s.scopeEvents(r, evs, site)
	}

	writeJSON(w, map[string]any{"events": evs, "next": next})
}

// scopeEvents, site-kısıtlı kimlik için olay listesini görünür agent/cihazlara
// daraltır (RecentTopologyLinks/handleTopology ile aynı yaklaşım).
func (s *Server) scopeEvents(r *http.Request, evs []store.Event, site string) []store.Event {
	agents, _ := s.store.ListAgents(2*time.Duration(s.telemetryInterval)*time.Second, site)
	visAgent := map[int64]bool{}
	for _, a := range agents {
		visAgent[a.ID] = true
	}
	devices, _ := s.store.ListDevices(site)
	visHost := map[string]bool{}
	for _, d := range devices {
		visHost[d.Host] = true
	}
	out := evs[:0]
	for _, e := range evs {
		switch e.Source {
		case "agent":
			if visAgent[e.AgentID] {
				out = append(out, e)
			}
		case "device":
			if visHost[e.Device] {
				out = append(out, e)
			}
		case "hub":
			// hub-yerel yakalama tek-makine kurulum → site kısıtı anlamsız, gizle
		}
	}
	return out
}
