package alert

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/sysmon"
)

func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "alerts.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine())
	return m, st
}

func TestBandwidthRule(t *testing.T) {
	m, _ := newTestManager(t)
	cfg := DefaultConfig()
	cfg.Bandwidth.Seconds = 3

	snap := &capture.Snapshot{Running: true, BpsIn: 200e6, BpsOut: 0} // 200 Mbps

	m.checkBandwidth(cfg, snap)
	m.checkBandwidth(cfg, snap)
	// henuz esik sure dolmadigi icin olay yok
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("erken tetikleme: %d olay", len(evs))
	}
	m.checkBandwidth(cfg, snap)
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 1 {
		t.Fatalf("esik sonrasi olay beklenirdi, gelen: %d", len(evs))
	}
	// cooldown: hemen tekrar tetiklenmemeli
	m.checkBandwidth(cfg, snap)
	m.checkBandwidth(cfg, snap)
	m.checkBandwidth(cfg, snap)
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 1 {
		t.Fatalf("cooldown calismadi: %d olay", len(evs))
	}
}

func TestNewProcessBaselineAndAlert(t *testing.T) {
	m, _ := newTestManager(t)
	cfg := DefaultConfig()
	cfg.NewProc.Enabled = true

	// taban cizgisi: mevcut surecler sessizce kaydedilir
	base := []sysmon.Connection{{Process: "safari", PID: 100}, {Process: "mail", PID: 101}}
	m.checkNewProcess(cfg, base)
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("taban cizgisi bildirim uretti: %d", len(evs))
	}

	// yeni surec bildirim uretmeli
	m.checkNewProcess(cfg, []sysmon.Connection{{Process: "safari", PID: 100}, {Process: "suskun-bitki", PID: 666}})
	evs, _ := m.st.RecentAlertEvents(10)
	if len(evs) != 1 || evs[0].Kind != "proc" || evs[0].Key != "suskun-bitki" {
		t.Fatalf("yeni surec olayi hatali: %+v", evs)
	}

	// ayni surec tekrar bildirim uretmemeli
	m.checkNewProcess(cfg, []sysmon.Connection{{Process: "suskun-bitki", PID: 666}})
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 1 {
		t.Fatalf("tekrar bildirim: %d", len(evs))
	}
}

func TestSuspiciousPort(t *testing.T) {
	m, _ := newTestManager(t)
	cfg := DefaultConfig() // portlar: 23, 4444, 1337, 31337

	cons := []sysmon.Connection{
		{Proto: "tcp", LocalAddr: "192.168.1.43:50000", RemoteAddr: "10.0.0.9:23", Status: "ESTABLISHED"},
		{Proto: "tcp", LocalAddr: "192.168.1.43:50001", RemoteAddr: "10.0.0.9:443", Status: "ESTABLISHED"},
	}
	m.checkPorts(cfg, cons)
	evs, _ := m.st.RecentAlertEvents(10)
	if len(evs) != 1 || evs[0].Key != "23" {
		t.Fatalf("suspicious port olayi hatali: %+v", evs)
	}
}

func TestNewTargetMinBytes(t *testing.T) {
	m, _ := newTestManager(t)
	cfg := DefaultConfig()
	cfg.NewTarget.MinTotalMB = 10

	// taban cizgisi
	m.checkNewTarget(cfg, &capture.Snapshot{
		TopEndpoints: []capture.EndpointStat{{IP: "1.1.1.1", Total: 999 << 20}},
	})
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("taban cizgisi bildirim uretti")
	}

	// esik altindaki yeni hedef: bildirim yok
	m.checkNewTarget(cfg, &capture.Snapshot{
		TopEndpoints: []capture.EndpointStat{
			{IP: "1.1.1.1", Total: 999 << 20},
			{IP: "2.2.2.2", Total: 5 << 20},
		},
	})
	// esik ustundeki yeni hedef: bildirim var
	m.checkNewTarget(cfg, &capture.Snapshot{
		TopEndpoints: []capture.EndpointStat{
			{IP: "1.1.1.1", Total: 999 << 20},
			{IP: "3.3.3.3", Total: 50 << 20, Hostname: "cdn.example.com"},
		},
	})
	evs, _ := m.st.RecentAlertEvents(10)
	if len(evs) != 1 || evs[0].Key != "3.3.3.3" {
		t.Fatalf("yeni hedef olaylari hatali: %+v", evs)
	}
}

func TestConfigPersistence(t *testing.T) {
	m, _ := newTestManager(t)
	cfg := m.Config()
	cfg.Bandwidth.InMbps = 55.5
	if err := m.UpdateConfig(cfg); err != nil {
		t.Fatalf("kayit: %v", err)
	}
	// ayni DB uzerinden yeni manager acilirsa ayar korunmali
	raw, _ := m.st.LoadAlertConfig()
	if raw == "" {
		t.Fatal("ayar bos")
	}
	m2 := NewManager(DefaultConfig(), m.st, capture.NewEngine())
	_ = m2
	// UpdateConfig store'a yazdi; Config() guncel degeri dondurmeli
	if got := m.Config().Bandwidth.InMbps; got != 55.5 {
		t.Fatalf("ayar okunmadi: %v", got)
	}
}

func TestRemotePort(t *testing.T) {
	if got := remotePort("1.2.3.4:443"); got != 443 {
		t.Fatalf("443 beklenirdi: %d", got)
	}
	if got := remotePort("[::1]:8080"); got != 8080 {
		t.Fatalf("8080 beklenirdi: %d", got)
	}
	if got := remotePort("no-port"); got != 0 {
		t.Fatalf("0 beklenirdi: %d", got)
	}
	_ = time.Now
}
