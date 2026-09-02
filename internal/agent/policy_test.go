package agent

// Kayitli agent enrollment'i (hello) atladigi icin hub politikasi —
// telemetri araligi + PCAP izni — telemetri yanitiyla tazelenir. Bu test
// postBatch'in yaniti isleyip c.opts'u guncelledigini dogrular.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestTelemetryReplyRefreshesPolicy(t *testing.T) {
	reply := telemetry.TelemetryReply{OK: true, Interval: 30}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(reply)
	}))
	defer srv.Close()

	c := New(Options{HubURL: srv.URL, Name: "n", StateFile: filepath.Join(t.TempDir(), "s.json")})
	st := State{AgentID: 1, Token: "t"}

	// Baslangic: restart sonrasi hello atlandi → PCAPEnabled sifir degeri.
	if c.PCAPEnabled() {
		t.Fatal("baslangicta PCAP kapali olmali")
	}

	// Hub politikasi acik: yanit pcap_enabled=true tasir.
	on := true
	reply = telemetry.TelemetryReply{OK: true, Interval: 45, PCAPEnabled: &on}
	if err := c.postBatch(st, 1, telemetry.TelemetryBatch{TS: 1}); err != nil {
		t.Fatalf("postBatch: %v", err)
	}
	if !c.PCAPEnabled() {
		t.Error("yanit sonrasi PCAP acik olmali")
	}
	if c.Interval() != 45 {
		t.Errorf("interval=45 beklenirdi, gelen: %d", c.Interval())
	}

	// Alan yok (eski hub) → mevcut deger korunur.
	reply = telemetry.TelemetryReply{OK: true, Interval: 45}
	if err := c.postBatch(st, 2, telemetry.TelemetryBatch{TS: 2}); err != nil {
		t.Fatalf("postBatch: %v", err)
	}
	if !c.PCAPEnabled() {
		t.Error("alan yokken PCAP degeri korunmali (acik kalmali)")
	}

	// Politika kapandi: yanit pcap_enabled=false.
	off := false
	reply = telemetry.TelemetryReply{OK: true, Interval: 45, PCAPEnabled: &off}
	if err := c.postBatch(st, 3, telemetry.TelemetryBatch{TS: 3}); err != nil {
		t.Fatalf("postBatch: %v", err)
	}
	if c.PCAPEnabled() {
		t.Error("yanit sonrasi PCAP kapali olmali")
	}
}
