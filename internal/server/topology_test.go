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

// TestTopologyEdgeTelemetry, Faz 23-D: source_type='device' + local_port bir
// ifName'e eşleşen kenara canlı SNMP arayüz telemetrisi bağlanır; eşleşmeyen
// kenar telemetrisiz döner (graf bozulmaz).
func TestTopologyEdgeTelemetry(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "topo.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	id, err := st.AddDevice(store.Device{Name: "core-sw", Host: "10.0.0.2", Kind: "switch", Vendor: "snmp", Enabled: true, PollSeconds: 60})
	if err != nil {
		t.Fatalf("dev: %v", err)
	}
	now := time.Now().Unix()
	// Gi0/1 1 Gbps, 30 sn'de ~950 Mbit/s rx → ~%95 util
	if err := st.SaveDeviceIfaceSamples(id, now-30, []store.DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6},
	}); err != nil {
		t.Fatalf("s1: %v", err)
	}
	if err := st.SaveDeviceIfaceSamples(id, now, []store.DeviceIface{
		{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6, RxBytes: 3_562_500_000},
	}); err != nil {
		t.Fatalf("s2: %v", err)
	}
	// eşleşen kenar (local_port = "Gi0/1") + eşleşmeyen kenar (local_port = "Gi9/9")
	for _, port := range []string{"Gi0/1", "Gi9/9"} {
		if err := st.UpsertTopologyLink(store.TopologyLink{
			Ts: now, Kind: "lldp", SourceType: "device", SourceID: id, SourceName: "core-sw",
			LocalPort: port, PeerType: "device", PeerName: "peer-" + port,
		}); err != nil {
			t.Fatalf("link %s: %v", port, err)
		}
	}

	resp := apiReq(t, http.MethodGet, ts.URL+"/api/v1/topology", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("200: %d", resp.StatusCode)
	}
	var g struct {
		Links []struct {
			LocalPort  string `json:"local_port"`
			Confidence string `json:"confidence"`
			Telemetry  *struct {
				IfName    string  `json:"if_name"`
				RxUtilPct float64 `json:"rx_util_pct"`
				Class     string  `json:"class"`
			} `json:"telemetry"`
		} `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var matched, unmatched int
	for _, l := range g.Links {
		if l.Confidence != "discovered" {
			t.Fatalf("kenar confidence 'discovered' olmalı: %+v", l)
		}
		switch l.LocalPort {
		case "Gi0/1":
			matched++
			if l.Telemetry == nil || l.Telemetry.Class != "ethernet" || l.Telemetry.RxUtilPct < 90 || l.Telemetry.RxUtilPct > 100 {
				t.Fatalf("Gi0/1 telemetri hatalı: %+v", l.Telemetry)
			}
		case "Gi9/9":
			unmatched++
			if l.Telemetry != nil {
				t.Fatalf("eşleşmeyen port telemetri taşımamalı: %+v", l.Telemetry)
			}
		}
	}
	if matched != 1 || unmatched != 1 {
		t.Fatalf("beklenen kenarlar bulunamadı: matched=%d unmatched=%d", matched, unmatched)
	}
}
