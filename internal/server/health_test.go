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
	"github.com/gokayybaz/bazntms/internal/health"
	"github.com/gokayybaz/bazntms/internal/store"
)

// TestHealthEndpoint, Faz 25-A: GET /api/v1/health — deterministik skor +
// açıklanabilir kesintiler. Boş filo → 100.
func TestHealthEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	get := func() health.Score {
		r := apiReq(t, http.MethodGet, ts.URL+"/api/v1/health", nil)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("200: %d", r.StatusCode)
		}
		var s health.Score
		json.NewDecoder(r.Body).Decode(&s)
		r.Body.Close()
		return s
	}

	if s := get(); s.Score != 100 || len(s.Deductions) != 0 {
		t.Fatalf("boş filo 100: %+v", s)
	}

	// 1 açık kritik uyarı + 1 açık olay (agent online/offline ayrımı
	// health_test.go birim testinde — RegisterAgent last_seen'i now'a damgalar).
	now := time.Now().Unix()
	st.InsertAlertEvent(store.AlertEvent{Ts: now, Kind: "ioc", Key: "k1", Message: "x", Severity: "crit", State: "firing", FirstTs: now, LastTs: now, Count: 1})
	st.CreateIncident(store.Incident{Title: "t", Severity: "crit", Status: "open", CorrelationKey: "r1|a1", RiskScore: 80, FirstSeen: now, LastSeen: now})

	srv.hcache.mu.Lock()
	srv.hcache.loaded = false
	srv.hcache.mu.Unlock()

	s := get()
	if s.Score != 87 || len(s.Deductions) != 2 {
		t.Fatalf("crit(5) + olay(risk80→8) → skor 87: %+v", s)
	}
	var haveCrit, haveInc bool
	for _, d := range s.Deductions {
		if d.Points <= 0 || d.Reason == "" {
			t.Fatalf("kesinti açıklanamıyor: %+v", d)
		}
		if d.Reason == "1 açık kritik uyarı" {
			haveCrit = true
		}
		if d.Reason == "1 açık olay" {
			haveInc = true
		}
	}
	if !haveCrit || !haveInc {
		t.Fatalf("beklenen kesinti kalemleri eksik: %+v", s.Deductions)
	}
	// skor kesinti toplamıyla tutarlı
	total := 0
	for _, d := range s.Deductions {
		total += d.Points
	}
	if s.Score != 100-total {
		t.Fatalf("skor tutarsız: %d vs 100-%d", s.Score, total)
	}
}
