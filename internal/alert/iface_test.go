package alert

// Faz 23-C: arayüz kullanım uyarısı — sürekli-aşım sayacı, önem (warn/crit),
// loopback/tünel atlaması, eşik altına düşünce sayaç sıfırlama.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newIfaceManager(t *testing.T) (*Manager, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "iface.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(DefaultConfig(), st, capture.NewEngine(), 30), st
}

// seedIface, bir cihaza iki örnekle bir arayüz yazar (rate hesabı için).
// deltaRxBytes 30 sn'de aktarılan bayt.
func seedIface(t *testing.T, st store.Store, devName string, iface store.DeviceIface, deltaRx uint64) int64 {
	t.Helper()
	id, err := st.AddDevice(store.Device{Name: devName, Host: devName + ".local", Kind: "switch", Vendor: "snmp", Enabled: true, PollSeconds: 60})
	if err != nil {
		t.Fatalf("cihaz: %v", err)
	}
	now := time.Now().Unix()
	base := iface
	base.RxBytes, base.TxBytes = 0, 0
	if err := st.SaveDeviceIfaceSamples(id, now-30, []store.DeviceIface{base}); err != nil {
		t.Fatalf("s1: %v", err)
	}
	iface.RxBytes, iface.TxBytes = deltaRx, 0
	if err := st.SaveDeviceIfaceSamples(id, now, []store.DeviceIface{iface}); err != nil {
		t.Fatalf("s2: %v", err)
	}
	return id
}

func kindCount(m *Manager, kind string) (n int, sev string) {
	for _, e := range m.RecentEvents(100) {
		if e.Kind == kind {
			n++
			sev = e.Severity
		}
	}
	return
}

func TestIfaceUtilSustainedAndSeverity(t *testing.T) {
	m, st := newIfaceManager(t)
	// 1 Gbps ethernet, 30 sn'de ~3.7 GB rx → ~%98 util (crit eşiği %90 üstü)
	seedIface(t, st, "sw-hot", store.DeviceIface{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6}, 3_675_000_000)

	cfg := DefaultConfig()
	cfg.Iface.SustainSec = 120 // need = 2 ardışık kontrol

	m.checkIfaceUtil(cfg)
	if n, _ := kindCount(m, "iface_util"); n != 0 {
		t.Fatalf("ilk kontrolde uyarı olmamalı (sürekli-aşım): %d", n)
	}
	m.checkIfaceUtil(cfg)
	n, sev := kindCount(m, "iface_util")
	if n != 1 {
		t.Fatalf("ikinci kontrolde 1 uyarı beklenirdi: %d", n)
	}
	if sev != "crit" {
		t.Fatalf("%%98 util → crit önem beklenirdi: %q", sev)
	}

	// üçüncü kontrol: açık olay bump'lanır, yeni satır yok
	m.checkIfaceUtil(cfg)
	if n, _ := kindCount(m, "iface_util"); n != 1 {
		t.Fatalf("açık olay tekrar üretilmemeli: %d", n)
	}
}

func TestIfaceUtilSkipsLoopbackAndTunnel(t *testing.T) {
	m, st := newIfaceManager(t)
	// tünel arayüzü %100'de → atlanmalı (yanıltıcı)
	seedIface(t, st, "sw-tun", store.DeviceIface{IfIndex: 5, Name: "Tunnel1", Speed: 1_000_000_000, OperStatus: 1, IfType: 131}, 4_000_000_000)
	// loopback %100'de → atlanmalı
	seedIface(t, st, "sw-lo", store.DeviceIface{IfIndex: 6, Name: "lo0", Speed: 1_000_000_000, OperStatus: 1, IfType: 24}, 4_000_000_000)

	cfg := DefaultConfig()
	cfg.Iface.SustainSec = 60 // need = 1
	m.checkIfaceUtil(cfg)
	m.checkIfaceUtil(cfg)
	if n, _ := kindCount(m, "iface_util"); n != 0 {
		t.Fatalf("loopback/tünel uyarı üretmemeli: %d", n)
	}
}

func TestIfaceUtilResetsBelowThreshold(t *testing.T) {
	m, st := newIfaceManager(t)
	// %30 util — eşik altında
	id := seedIface(t, st, "sw-cool", store.DeviceIface{IfIndex: 1, Name: "Gi0/1", Speed: 1_000_000_000, OperStatus: 1, IfType: 6}, 1_125_000_000)
	_ = id

	cfg := DefaultConfig()
	cfg.Iface.SustainSec = 60
	m.checkIfaceUtil(cfg)
	m.checkIfaceUtil(cfg)
	if n, _ := kindCount(m, "iface_util"); n != 0 {
		t.Fatalf("eşik altı util uyarı üretmemeli: %d", n)
	}

	// devre dışı → sayaç temizlenir, uyarı yok
	cfg.Iface.Enabled = false
	m.checkIfaceUtil(cfg)
	if n, _ := kindCount(m, "iface_util"); n != 0 {
		t.Fatalf("devre dışı: %d", n)
	}
}
