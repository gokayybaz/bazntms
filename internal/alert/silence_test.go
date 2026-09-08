package alert

// S22.10: bakım penceresi — eşleşen yeni uyarı state='silenced' kaydedilir,
// bildirilmez, "açık" sayılmaz (koşul sürerse yeniden değerlendirilir).

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestFireSilenced(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sil.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(DefaultConfig(), st, capture.NewEngine(), 30)

	now := time.Now().Unix()
	if _, err := m.AddSilence(store.AlertSilence{
		MatchKind: "bw", MatchSite: "dc1",
		StartsTs: now - 60, EndsTs: now + 3600, Reason: "gece bakımı", CreatedTs: now,
	}); err != nil {
		t.Fatalf("silence ekle: %v", err)
	}

	// eşleşen → susturulur
	m.fireCtx("bw", "agent-in:a1", "yüksek", fireOpts{Site: "dc1"})
	// eşleşmeyen saha → normal ateşlenir
	m.fireCtx("bw", "agent-in:a2", "yüksek", fireOpts{Site: "dc2"})
	// eşleşmeyen kind → normal
	m.fireCtx("port", "a1:4444", "port", fireOpts{Site: "dc1"})

	states := map[string]string{}
	for _, e := range m.RecentEvents(20) {
		states[e.Key] = e.State
	}
	if states["agent-in:a1"] != "silenced" {
		t.Fatalf("dc1 bw susturulmalıydı: %q", states["agent-in:a1"])
	}
	if states["agent-in:a2"] != "firing" {
		t.Fatalf("dc2 bw normal ateşlenmeliydi: %q", states["agent-in:a2"])
	}
	if states["a1:4444"] != "firing" {
		t.Fatalf("port susturulmamalıydı: %q", states["a1:4444"])
	}
	// susturulan olay "açık" değil → koşul sürerse yeniden değerlendirilir
	if o, _ := st.OpenAlertEventByKey("bw", "agent-in:a1"); o != nil {
		t.Fatalf("susturulan olay açık dönmemeli")
	}
}
