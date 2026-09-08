package alert

// S22.6: fire() dedup — aynı (kind,key) açık olay varsa yeni satır yerine
// tekrar sayacı artar ve yeniden bildirim yapılmaz.

import (
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestFireDedup(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "dedup.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine(), 30)

	m.fireCtx("port", "10.0.0.5:4444", "ilk", fireOpts{Site: "dc1"})
	m.fireCtx("port", "10.0.0.5:4444", "tekrar", fireOpts{})
	m.fireCtx("port", "10.0.0.5:4444", "yine", fireOpts{})

	evs := m.RecentEvents(10)
	if len(evs) != 1 {
		t.Fatalf("tek olay bekleniyordu (dedup), %d: %+v", len(evs), evs)
	}
	e := evs[0]
	if e.Count != 3 {
		t.Fatalf("count 3 bekleniyordu, %d", e.Count)
	}
	if e.Message != "yine" {
		t.Fatalf("mesaj en son tekrarınki olmalı: %q", e.Message)
	}
	if e.Severity != "crit" { // kindSeverity["port"]
		t.Fatalf("port → crit bekleniyordu, %q", e.Severity)
	}
	if e.Site != "dc1" || e.State != "firing" {
		t.Fatalf("site/state: %+v", e)
	}

	// farklı key → ayrı olay
	m.fireCtx("port", "10.0.0.9:1337", "başka hedef", fireOpts{})
	if len(m.RecentEvents(10)) != 2 {
		t.Fatalf("farklı key ayrı olay olmalı")
	}
}
