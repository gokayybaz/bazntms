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
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestEventsEndpoint, Faz 24-A: GET /api/v1/events ham kaynak tabloları
// normalleştirerek döndürür (uyarılardan ayrı — ADR 0010).
func TestEventsEndpoint(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "ev.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	id, _ := st.RegisterAgent(store.Agent{Name: "a", TokenHash: store.TokenHash("a")})
	now := time.Now().Unix()
	st.SaveAgentDNS(id, now-5, []telemetry.DNSSample{{Process: "curl", Domain: "example.com", Queries: 2, Responses: 2}})
	st.SaveL7(id, now-4, []telemetry.L7Sample{{Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.2.3.4", Count: 1}})
	st.SaveFlows([]store.FlowRow{{Ts: now - 3, Device: "fw", Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Packets: 1, Octets: 60}})

	var out struct {
		Events []store.Event `json:"events"`
		Next   int64         `json:"next"`
	}
	r := apiReq(t, http.MethodGet, ts.URL+"/api/v1/events?since_min=60", nil)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("200: %d", r.StatusCode)
	}
	json.NewDecoder(r.Body).Decode(&out)
	r.Body.Close()
	if len(out.Events) != 3 {
		t.Fatalf("3 olay: %+v", out.Events)
	}
	if out.Events[0].Ts < out.Events[2].Ts {
		t.Fatalf("ts azalan değil: %+v", out.Events)
	}

	// tür filtresi
	r2 := apiReq(t, http.MethodGet, ts.URL+"/api/v1/events?type=netflow.flow&since_min=60", nil)
	json.NewDecoder(r2.Body).Decode(&out)
	r2.Body.Close()
	if len(out.Events) != 1 || out.Events[0].Type != "netflow.flow" {
		t.Fatalf("tür filtresi: %+v", out.Events)
	}
}
