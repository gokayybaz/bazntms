package alert

// S22.21: SLA hedef ihlali → "sla_breach" uyarısı (severity crit).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestSLABreach(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sla.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine(), 30)

	// dc1'de 1 agent (online) + 2 cihaz (hiç poll edilmedi → sağlıksız)
	st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1", Site: "dc1"})
	st.AddDevice(store.Device{Name: "r1", Host: "10.0.0.1", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60, Site: "dc1"})
	st.AddDevice(store.Device{Name: "r2", Host: "10.0.0.2", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60, Site: "dc1"})

	st.UpsertSLATarget(store.SLATarget{Scope: "site", Site: "dc1", DeviceHealthPct: 90})

	m.checkSLA()

	var breach *store.AlertEvent
	for _, e := range m.RecentEvents(10) {
		if e.Kind == "sla_breach" && strings.Contains(e.Key, "dc1") {
			ec := e
			breach = &ec
		}
	}
	if breach == nil {
		t.Fatalf("sla_breach uyarısı bekleniyordu: %+v", m.RecentEvents(10))
	}
	if breach.Severity != "crit" || breach.Site != "dc1" {
		t.Fatalf("severity/site: %+v", breach)
	}
	if !strings.Contains(breach.Message, "cihaz sağlığı") {
		t.Fatalf("mesaj: %q", breach.Message)
	}

	// hedefsiz saha → çökme yok, yeni ihlal yok
	before := len(m.RecentEvents(50))
	st.DeleteSLATarget("site", "dc1")
	m.checkSLA()
	if after := len(m.RecentEvents(50)); after != before {
		t.Fatalf("hedef silinince yeni olay olmamalı: %d → %d", before, after)
	}
}
