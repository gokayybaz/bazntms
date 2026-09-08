package alert

// S22.9: korelasyon — aynı sahada pencere içinde ateşlenen olaylar ortak
// group_id alır; farklı saha / pencere dışı ayrı kalır.

import (
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestCorrelateSameSite(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "corr.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine(), 30)

	m.fireCtx("bw", "agent-in:a1", "yüksek", fireOpts{Site: "dc1"})
	m.fireCtx("port", "a1:4444", "şüpheli port", fireOpts{Site: "dc1"})
	m.fireCtx("proc", "a9:nc", "yeni süreç", fireOpts{Site: "dc2"}) // farklı saha
	m.fireCtx("anomaly", "bps:local::9", "sapma", fireOpts{})       // saha yok

	byKey := map[string]store.AlertEvent{}
	for _, e := range m.RecentEvents(20) {
		byKey[e.Key] = e
	}
	g1 := byKey["agent-in:a1"].GroupID
	g2 := byKey["a1:4444"].GroupID
	if g1 == "" || g1 != g2 {
		t.Fatalf("dc1 olayları aynı grupta olmalı: %q vs %q", g1, g2)
	}
	if byKey["a9:nc"].GroupID == g1 {
		t.Fatalf("dc2 olayı dc1 grubuna girmemeli")
	}
	if byKey["bps:local::9"].GroupID != "" {
		t.Fatalf("sahasız olay gruplanmamalı: %q", byKey["bps:local::9"].GroupID)
	}

	// pencere kapalıyken gruplama yok
	m2 := NewManager(func() Config { c := DefaultConfig(); c.CorrelateWindowSec = -1; return c }(), st, capture.NewEngine(), 30)
	m2.fireCtx("bw", "agent-in:z1", "y", fireOpts{Site: "dc3"})
	m2.fireCtx("bw", "agent-out:z1", "y", fireOpts{Site: "dc3"})
	for _, e := range m2.RecentEvents(30) {
		if (e.Key == "agent-in:z1" || e.Key == "agent-out:z1") && e.GroupID != "" {
			t.Fatalf("pencere kapalıyken grup atanmamalı: %+v", e)
		}
	}
}
