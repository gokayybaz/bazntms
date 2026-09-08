package store

import (
	"testing"
	"time"
)

// TestAlertEventLifecycle, S22.6: InsertAlertEvent varsayılanları +
// OpenAlertEventByKey (yalnız firing/ack) + BumpAlertEvent (yeni satır yok).
func TestAlertEventLifecycle(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.InsertAlertEvent(AlertEvent{
		Ts: now, Kind: "anomaly", Key: "bps:fleet::13", Message: "sapma",
		Severity: "warn", Site: "dc1",
	})
	if err != nil || id == 0 {
		t.Fatalf("insert: %v %d", err, id)
	}

	open, err := st.OpenAlertEventByKey("anomaly", "bps:fleet::13")
	if err != nil || open == nil {
		t.Fatalf("open sorgu: %v %+v", err, open)
	}
	if open.State != "firing" || open.Count != 1 || open.FirstTs != now || open.LastTs != now || open.Site != "dc1" {
		t.Fatalf("insert varsayılanları: %+v", open)
	}

	// aynı koşul tekrar → bump, yeni satır yok
	if err := st.BumpAlertEvent(open.ID, now+30, "sapma sürüyor"); err != nil {
		t.Fatalf("bump: %v", err)
	}
	open, _ = st.OpenAlertEventByKey("anomaly", "bps:fleet::13")
	if open.Count != 2 || open.LastTs != now+30 || open.Message != "sapma sürüyor" {
		t.Fatalf("bump sonrası: %+v", open)
	}
	if evs, _ := st.RecentAlertEvents(10); len(evs) != 1 {
		t.Fatalf("tek satır bekleniyordu, %d", len(evs))
	}

	// resolved olay artık "açık" değil
	if _, err := st.(*sqlStore).db.Exec(
		st.(*sqlStore).q(`UPDATE alert_events SET state = 'resolved' WHERE id = ?`), open.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if o, _ := st.OpenAlertEventByKey("anomaly", "bps:fleet::13"); o != nil {
		t.Fatalf("resolved olay açık dönmemeli: %+v", o)
	}
}
