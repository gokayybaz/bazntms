package store

import "testing"

func TestSLATargets(t *testing.T) {
	st := openTest(t)

	// global
	if err := st.UpsertSLATarget(SLATarget{Scope: "global", AgentUptimePct: 95, DeviceHealthPct: 90}); err != nil {
		t.Fatalf("upsert global: %v", err)
	}
	// dc1 özel
	if err := st.UpsertSLATarget(SLATarget{Scope: "site", Site: "dc1", AgentUptimePct: 99}); err != nil {
		t.Fatalf("upsert site: %v", err)
	}

	// dc1 → site-özel
	t1, _ := st.SLATargetFor("dc1")
	if t1.AgentUptimePct != 99 {
		t.Fatalf("dc1 site hedefi: %+v", t1)
	}
	// dc2 → global'e düşer
	t2, _ := st.SLATargetFor("dc2")
	if t2.AgentUptimePct != 95 || t2.DeviceHealthPct != 90 {
		t.Fatalf("dc2 global fallback: %+v", t2)
	}
	// "" → global
	tg, _ := st.SLATargetFor("")
	if tg.Scope != "global" {
		t.Fatalf("global: %+v", tg)
	}
	if !tg.Set() {
		t.Fatal("Set() true olmalı")
	}

	// upsert güncelle
	st.UpsertSLATarget(SLATarget{Scope: "global", AgentUptimePct: 97})
	tg, _ = st.SLATargetFor("")
	if tg.AgentUptimePct != 97 || tg.DeviceHealthPct != 0 {
		t.Fatalf("upsert replace: %+v", tg)
	}

	if all, _ := st.ListSLATargets(); len(all) != 2 {
		t.Fatalf("liste: %d", len(all))
	}
	st.DeleteSLATarget("site", "dc1")
	if all, _ := st.ListSLATargets(); len(all) != 1 {
		t.Fatalf("silme sonrası: %d", len(all))
	}
}
