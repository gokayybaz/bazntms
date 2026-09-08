package server

// Olay (incident) yönetim uçları (Faz 24-B). Motor: internal/incident.

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// handleIncidentsList, GET /api/v1/incidents?status=&severity=&agent_id=&limit=
func (s *Server) handleIncidentsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	agentID, _ := strconv.ParseInt(q.Get("agent_id"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	site := SiteScope(identityFromCtx(r))
	if agentID > 0 && site != "" {
		if a, err := s.store.AgentByID(agentID); err != nil || a.Site != site {
			http.Error(w, "agent bulunamadi", http.StatusNotFound)
			return
		}
	}
	list, err := s.store.ListIncidents(store.IncidentFilter{
		Status:   q.Get("status"),
		Severity: q.Get("severity"),
		AgentID:  agentID,
		Site:     site, // site-kısıtlı kimlik → yalnız kendi sahası
		Limit:    limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"incidents": list})
}

// handleIncidentDetail, GET /api/v1/incidents/{id} — incident + kanıt zaman çizelgesi.
func (s *Server) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	in, evs, err := s.store.IncidentByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in == nil || !s.incidentInScope(r, in) {
		http.Error(w, "olay bulunamadi", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"incident": in, "evidence": evs})
}

// handleIncidentAction, POST /api/v1/incidents/{id}/{action}
// action ∈ ack | investigate | resolve | close.
func (s *Server) handleIncidentAction(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	action := r.PathValue("action")
	status := map[string]string{
		"ack":         "open", // ack yalnız ack_by/ack_ts doldurur, durumu open tutar
		"investigate": "investigating",
		"resolve":     "resolved",
		"close":       "closed",
	}[action]
	if status == "" {
		http.Error(w, "geçersiz eylem", http.StatusBadRequest)
		return
	}
	in, _, err := s.store.IncidentByID(id)
	if err != nil || in == nil || !s.incidentInScope(r, in) {
		http.Error(w, "olay bulunamadi", http.StatusNotFound)
		return
	}

	ident := identityFromCtx(r)
	by := ""
	if ident != nil {
		by = ident.Username
	}
	// "ack" → ack alanları + open; diğerleri → durum geçişi
	ackBy := ""
	if action == "ack" || action == "investigate" {
		ackBy = by
	}
	if err := s.store.SetIncidentStatus(id, status, ackBy, time.Now().Unix()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, ident, "incident."+action, fmt.Sprintf("incident:%d", id), in.Title)
	writeJSON(w, map[string]any{"ok": true, "status": status})
}

// incidentInScope, site-kısıtlı kimliğin bu incident'ı görüp göremeyeceği.
func (s *Server) incidentInScope(r *http.Request, in *store.Incident) bool {
	scope := SiteScope(identityFromCtx(r))
	return scope == "" || in.Site == scope
}
