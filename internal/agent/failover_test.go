package agent

// Faz 5.4: coklu-hub failover birim testi — birinci hub hata verirse
// agent ikinciye gecer ve basarili hub'a yapisir.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/version"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestHubFailover(t *testing.T) {
	primaryHit := 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHit++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/agent/hello":
			json.NewEncoder(w).Encode(telemetry.HubReply{
				Accepted: true, AgentID: 42, AgentToken: "tok-abc",
				TelemetryIntervalSeconds: 30,
			})
		case "/api/v1/agent/telemetry":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "interval": 30})
		default:
			http.NotFound(w, r)
		}
	}))
	defer secondary.Close()

	c := New(Options{
		HubURLs:     []string{primary.URL, secondary.URL},
		EnrollToken: "enroll",
		Name:        "failover-test",
		StateFile:   filepath.Join(t.TempDir(), "agent.state.json"),
	})

	// hello: birinci hub 503 → ikinci basarili
	st, err := c.Enroll()
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if st.AgentID != 42 || st.Token != "tok-abc" {
		t.Fatalf("state hatali: %+v", st)
	}
	if primaryHit == 0 {
		t.Fatal("birinci hub denenmedi")
	}
	if c.hubIdx != 1 {
		t.Fatalf("aktif hub ikinci olmali, idx=%d", c.hubIdx)
	}

	// sonraki gonderim dogrudan ikinci hub'a gider (yapisma)
	before := primaryHit
	if err := c.postBatch(st, 1234, telemetry.TelemetryBatch{TS: 1234}); err != nil {
		t.Fatalf("telemetri: %v", err)
	}
	if primaryHit != before {
		t.Fatal("basarili hub'a yapisma yok: birinci hub tekrar denendi")
	}

	// ver ve tek-hub modu geriye uyumlu
	if got := c.opts.IntervalSec; got != 30 {
		t.Fatalf("interval: %d", got)
	}
	if v := version.Version; v == "" {
		t.Fatal("surum bos")
	}
}

func TestHubSingleURL(t *testing.T) {
	var hit int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		json.NewEncoder(w).Encode(telemetry.HubReply{Accepted: true, AgentID: 1, AgentToken: "t"})
	}))
	defer srv.Close()

	c := New(Options{HubURL: srv.URL, EnrollToken: "e", Name: "n", StateFile: filepath.Join(t.TempDir(), "agent.state.json")})
	st, err := c.Enroll()
	if err != nil || st.AgentID != 1 {
		t.Fatalf("enroll: %v %+v", err, st)
	}
	if c.hubIdx != 0 {
		t.Fatalf("idx: %d", c.hubIdx)
	}
	_ = hit
}
