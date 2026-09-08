package store

import (
	"testing"
	"time"
)

func TestQueryAlertEvents(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	seed := []AlertEvent{
		{Ts: now - 50, Kind: "bw", Key: "a", Message: "1", Severity: "warn", Site: "dc1"},
		{Ts: now - 40, Kind: "ioc", Key: "b", Message: "2", Severity: "crit", Site: "dc1"},
		{Ts: now - 30, Kind: "anomaly", Key: "c", Message: "3", Severity: "warn", Site: "dc2"},
		{Ts: now - 20, Kind: "bw", Key: "d", Message: "4", Severity: "warn", Site: "dc2"},
		{Ts: now - 10, Kind: "port", Key: "e", Message: "5", Severity: "crit", Site: ""},
	}
	for _, e := range seed {
		if _, err := st.InsertAlertEvent(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// site filtresi
	dc1, _, _ := st.QueryAlertEvents(AlertEventFilter{Site: "dc1"})
	if len(dc1) != 2 {
		t.Fatalf("dc1: %d", len(dc1))
	}
	// severity filtresi
	crit, _, _ := st.QueryAlertEvents(AlertEventFilter{Severity: "crit"})
	if len(crit) != 2 {
		t.Fatalf("crit: %d", len(crit))
	}
	// kind + state
	bwFiring, _, _ := st.QueryAlertEvents(AlertEventFilter{Kind: "bw", State: "firing"})
	if len(bwFiring) != 2 {
		t.Fatalf("bw/firing: %d", len(bwFiring))
	}

	// cursor sayfalama: limit 2 → 3 sayfa
	var seen int
	var cursor int64
	for i := 0; i < 5; i++ {
		page, next, err := st.QueryAlertEvents(AlertEventFilter{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("sayfa: %v", err)
		}
		seen += len(page)
		if next == 0 {
			break
		}
		cursor = next
	}
	if seen != 5 {
		t.Fatalf("sayfalama toplamı 5 beklenirdi, %d", seen)
	}

	// ack + note
	id, _ := st.InsertAlertEvent(AlertEvent{Ts: now, Kind: "bw", Key: "z", Message: "ack testi", Severity: "warn"})
	if err := st.AckAlertEvent(id, "ops", now, "inceleniyor"); err != nil {
		t.Fatalf("ack: %v", err)
	}
	e, _ := st.AlertEventByID(id)
	if e.State != "ack" || e.AckBy != "ops" || e.Note != "inceleniyor" {
		t.Fatalf("ack alanları: %+v", e)
	}
	// resolved olay ack'lenemez
	st.ResolveAlertEvent(id, now)
	st.AckAlertEvent(id, "x", now, "y")
	e, _ = st.AlertEventByID(id)
	if e.State != "resolved" {
		t.Fatalf("resolved olay ack ile değişmemeli: %+v", e)
	}
	if err := st.SetAlertEventNote(id, "kapatıldı"); err != nil || func() bool { x, _ := st.AlertEventByID(id); return x.Note != "kapatıldı" }() {
		t.Fatalf("not güncellenmedi")
	}
}
