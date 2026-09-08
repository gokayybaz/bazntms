package store

import (
	"testing"
	"time"
)

func TestAlertSilenceCRUD(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.AddAlertSilence(AlertSilence{
		MatchKind: "anomaly", MatchSite: "dc1",
		StartsTs: now - 60, EndsTs: now + 3600, Reason: "planlı bakım", CreatedBy: "ops", CreatedTs: now,
	})
	if err != nil || id == 0 {
		t.Fatalf("add: %v %d", err, id)
	}
	// geçmiş (aktif değil)
	st.AddAlertSilence(AlertSilence{MatchKind: "bw", StartsTs: now - 7200, EndsTs: now - 3600, CreatedTs: now})

	all, _ := st.ListAlertSilences(false, now)
	if len(all) != 2 {
		t.Fatalf("tüm liste 2 beklenirdi, %d", len(all))
	}
	active, _ := st.ListAlertSilences(true, now)
	if len(active) != 1 || active[0].ID != id {
		t.Fatalf("aktif liste: %+v", active)
	}

	sl := active[0]
	if !sl.Active(now) || sl.Active(now+4000) {
		t.Fatalf("Active penceresi yanlış")
	}
	if !sl.Matches("anomaly", "dc1", "bps:site:dc1:9") {
		t.Fatalf("eşleşmeliydi")
	}
	if sl.Matches("bw", "dc1", "x") || sl.Matches("anomaly", "dc2", "x") {
		t.Fatalf("kind/site jokerları çok geniş eşleşiyor")
	}

	if err := st.DeleteAlertSilence(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if a, _ := st.ListAlertSilences(true, now); len(a) != 0 {
		t.Fatalf("silme sonrası aktif 0 beklenirdi")
	}
}
