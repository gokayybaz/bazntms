package alert

// Faz 8.7: FortiGate uyarı testleri — down tünel ve SLA ihlali üretimi.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newFortiManager(t *testing.T) (*Manager, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "forti.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(DefaultConfig(), st, capture.NewEngine(), 30), st
}

func TestFortiAlerts(t *testing.T) {
	m, st := newFortiManager(t)

	id, err := st.AddDevice(store.Device{
		Name: "fgt-a", Host: "10.0.0.1", Vendor: "fortigate", Enabled: true, PollSeconds: 60,
	})
	if err != nil {
		t.Fatalf("cihaz: %v", err)
	}
	now := time.Now().Unix()

	// down tünel + up tünel + SLA ihlali + yüksek oturum
	st.SaveFortiVPNStatus(id, now, []store.FortiVPNStatus{
		{DeviceID: id, VDOM: "root", Kind: "ipsec", Name: "hq", Status: "up", Ts: now},
		{DeviceID: id, VDOM: "root", Kind: "ipsec", Name: "branch", Status: "down", Ts: now},
	})
	st.SaveFortiSDWAN(id, now, []store.FortiSDWANSample{
		{DeviceID: id, VDOM: "root", Member: "wan2", HealthCheck: "sla1",
			LatencyMs: 400, JitterMs: 10, PacketLossPct: 1, State: "up", Ts: now},
	})
	st.SaveDeviceResources(store.DeviceResource{
		DeviceID: id, Ts: now, CPUPct: 40, MemPct: 60, Sessions: 5000,
	})

	cfg := DefaultConfig()
	cfg.Forti.MaxSessions = 2000
	m.checkForti(cfg)

	kinds := map[string]int{}
	for _, e := range m.RecentEvents(50) {
		kinds[e.Kind]++
	}
	if kinds["vpn_down"] != 1 {
		t.Fatalf("vpn_down: %d (hq up olmalı, branch down)", kinds["vpn_down"])
	}
	if kinds["sdwan_sla_breach"] != 1 {
		t.Fatalf("sdwan_sla_breach: %d", kinds["sdwan_sla_breach"])
	}
	if kinds["high_sessions"] != 1 {
		t.Fatalf("high_sessions: %d", kinds["high_sessions"])
	}

	// ikinci değerlendirme: cooldown aynı olayı tekrar üretmez
	m.checkForti(cfg)
	kinds2 := map[string]int{}
	for _, e := range m.RecentEvents(100) {
		kinds2[e.Kind]++
	}
	if kinds2["vpn_down"] != 1 {
		t.Fatalf("cooldown çalışmadı: %d", kinds2["vpn_down"])
	}
}

func TestFortiAlertsDisabled(t *testing.T) {
	m, st := newFortiManager(t)
	id, _ := st.AddDevice(store.Device{Name: "fgt-b", Host: "10.0.0.2", Vendor: "fortigate", Enabled: true, PollSeconds: 60})
	now := time.Now().Unix()
	st.SaveFortiVPNStatus(id, now, []store.FortiVPNStatus{
		{DeviceID: id, VDOM: "root", Kind: "ipsec", Name: "x", Status: "down", Ts: now},
	})

	cfg := DefaultConfig()
	cfg.Forti.VPNDown = false
	cfg.Forti.SDWANLatencyMs = 0
	cfg.Forti.SDWANJitterMs = 0
	cfg.Forti.SDWANLossPct = 0
	cfg.Forti.MaxSessions = 0
	m.checkForti(cfg)

	if n := len(m.RecentEvents(10)); n != 0 {
		t.Fatalf("kapalıyken uyarı üretildi: %d", n)
	}
}
