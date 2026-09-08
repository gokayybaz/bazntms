package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

// TestIncidentEndpoints, Faz 24-B: liste + detay (kanıt) + eylem (ack/resolve →
// durum + denetim).
func TestIncidentEndpoints(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "inc.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	now := time.Now().Unix()
	id, err := st.CreateIncident(store.Incident{
		Title: "Şüpheli çıkış aktivitesi", Severity: "crit", Status: "open",
		AgentID: 5, CorrelationKey: "r1|agent5", CorrelationReason: "yeni süreç + yeni hedef",
		Summary: "proc → target", RiskScore: 72, FirstSeen: now - 120, LastSeen: now,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	st.AddIncidentEvidence(id, store.IncidentEvidence{Kind: "alert", Ref: "1", Ts: now - 120, Summary: "[proc/info] yeni süreç"})
	st.AddIncidentEvidence(id, store.IncidentEvidence{Kind: "alert", Ref: "2", Ts: now - 60, Summary: "[target/info] yeni hedef"})

	// --- liste ---
	var list struct {
		Incidents []store.Incident `json:"incidents"`
	}
	r := apiReq(t, http.MethodGet, ts.URL+"/api/v1/incidents", nil)
	json.NewDecoder(r.Body).Decode(&list)
	r.Body.Close()
	if len(list.Incidents) != 1 || list.Incidents[0].RiskScore != 72 {
		t.Fatalf("liste: %+v", list.Incidents)
	}

	// status filtresi: resolved yok
	r2 := apiReq(t, http.MethodGet, ts.URL+"/api/v1/incidents?status=resolved", nil)
	json.NewDecoder(r2.Body).Decode(&list)
	r2.Body.Close()
	if len(list.Incidents) != 0 {
		t.Fatalf("resolved filtresi: %+v", list.Incidents)
	}

	// --- detay + kanıt kronolojik ---
	var detail struct {
		Incident store.Incident           `json:"incident"`
		Evidence []store.IncidentEvidence `json:"evidence"`
	}
	rd := apiReq(t, http.MethodGet, ts.URL+"/api/v1/incidents/"+i64(id), nil)
	if rd.StatusCode != http.StatusOK {
		t.Fatalf("detay 200: %d", rd.StatusCode)
	}
	json.NewDecoder(rd.Body).Decode(&detail)
	rd.Body.Close()
	if len(detail.Evidence) != 2 || detail.Evidence[0].Ts > detail.Evidence[1].Ts {
		t.Fatalf("kanıt kronolojik değil: %+v", detail.Evidence)
	}

	// --- eylem: investigate → durum + denetim ---
	ra := apiReq(t, http.MethodPost, ts.URL+"/api/v1/incidents/"+i64(id)+"/investigate", nil)
	ra.Body.Close()
	if ra.StatusCode != http.StatusOK {
		t.Fatalf("investigate: %d", ra.StatusCode)
	}
	if in, _, _ := st.IncidentByID(id); in.Status != "investigating" {
		t.Fatalf("durum investigating olmalı: %+v", in)
	}

	// resolve
	apiReq(t, http.MethodPost, ts.URL+"/api/v1/incidents/"+i64(id)+"/resolve", nil).Body.Close()
	if in, _, _ := st.IncidentByID(id); in.Status != "resolved" || in.ResolvedTs == 0 {
		t.Fatalf("resolve: %+v", in)
	}

	// denetim kaydı
	auds, _ := st.RecentAuditEvents(20, "")
	var found bool
	for _, a := range auds {
		if a.Action == "incident.resolve" {
			found = true
		}
	}
	if !found {
		t.Fatalf("incident.resolve denetime yazılmalı: %+v", auds)
	}

	// geçersiz eylem → 400
	rbad := apiReq(t, http.MethodPost, ts.URL+"/api/v1/incidents/"+i64(id)+"/frobnicate", nil)
	rbad.Body.Close()
	if rbad.StatusCode != http.StatusBadRequest {
		t.Fatalf("geçersiz eylem 400 beklenirdi: %d", rbad.StatusCode)
	}
}
