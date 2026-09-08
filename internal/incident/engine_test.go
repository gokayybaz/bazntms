package incident

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func newEngine(t *testing.T) (*Engine, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "inc.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, func() Config { return DefaultConfig() }), st
}

func alert(t *testing.T, st store.Store, kind string, agentID int64, sev string, ts int64) int64 {
	t.Helper()
	id, err := st.InsertAlertEvent(store.AlertEvent{
		Ts: ts, Kind: kind, Key: kind + "-k", Message: kind + " oldu", Severity: sev,
		State: "firing", Site: "dc1", AgentID: agentID, FirstTs: ts, LastTs: ts, Count: 1,
	})
	if err != nil {
		t.Fatalf("alert %s: %v", kind, err)
	}
	return id
}

func TestRule1NewProcNewTarget(t *testing.T) {
	e, st := newEngine(t)
	now := time.Now()
	alert(t, st, "proc", 7, "info", now.Add(-3*time.Minute).Unix())
	alert(t, st, "target", 7, "info", now.Add(-1*time.Minute).Unix())

	if err := e.Evaluate(now); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	incs, _ := st.ListIncidents(store.IncidentFilter{})
	if len(incs) != 1 {
		t.Fatalf("1 incident beklenirdi: %+v", incs)
	}
	in := incs[0]
	if in.Severity != "crit" || in.AgentID != 7 || in.Status != "open" {
		t.Fatalf("incident hatalı: %+v", in)
	}
	if in.RiskScore < 55 {
		t.Fatalf("risk skoru düşük: %d", in.RiskScore)
	}
	// kanıt: 2 uyarı
	_, evs, _ := st.IncidentByID(in.ID)
	if len(evs) != 2 {
		t.Fatalf("2 kanıt beklenirdi: %+v", evs)
	}
	for i := 1; i < len(evs); i++ {
		if evs[i].Ts < evs[i-1].Ts {
			t.Fatalf("kanıt kronolojik değil")
		}
	}

	// idempotent: tekrar değerlendir → hâlâ 1 incident (dedup)
	if err := e.Evaluate(now); err != nil {
		t.Fatalf("evaluate2: %v", err)
	}
	if incs2, _ := st.ListIncidents(store.IncidentFilter{}); len(incs2) != 1 {
		t.Fatalf("dedup çalışmadı: %d", len(incs2))
	}
}

func TestRule2And4(t *testing.T) {
	e, st := newEngine(t)
	now := time.Now()
	// agent 1: proc + port (kural 2)
	alert(t, st, "proc", 1, "info", now.Add(-2*time.Minute).Unix())
	alert(t, st, "port", 1, "crit", now.Add(-1*time.Minute).Unix())
	// agent 2: anomaly + bw (kural 4)
	alert(t, st, "anomaly", 2, "warn", now.Add(-3*time.Minute).Unix())
	alert(t, st, "bw", 2, "warn", now.Add(-1*time.Minute).Unix())

	e.Evaluate(now)
	incs, _ := st.ListIncidents(store.IncidentFilter{})
	byAgent := map[int64]store.Incident{}
	for _, in := range incs {
		byAgent[in.AgentID] = in
	}
	if byAgent[1].Severity != "crit" {
		t.Fatalf("kural 2 crit olmalı (port crit): %+v", byAgent[1])
	}
	if byAgent[2].Severity != "warn" || byAgent[2].Title == "" {
		t.Fatalf("kural 4: %+v", byAgent[2])
	}
}

func TestRule5MultipleAlerts(t *testing.T) {
	e, st := newEngine(t)
	now := time.Now()
	for i, k := range []string{"proc", "bw", "anomaly", "iface_util"} {
		alert(t, st, k, 9, "warn", now.Add(-time.Duration(8-i)*time.Minute).Unix())
	}
	e.Evaluate(now)
	incs, _ := st.ListIncidents(store.IncidentFilter{AgentID: 9})
	var r5 *store.Incident
	for i := range incs {
		if incs[i].RiskScore > 0 && incs[i].Title != "" && incs[i].AgentID == 9 {
			// rule 5 başlığı "Çoklu şüpheli aktivite"
			if incs[i].CorrelationKey == "r5|agent9" {
				r5 = &incs[i]
			}
		}
	}
	if r5 == nil {
		t.Fatalf("kural 5 incident'ı yok: %+v", incs)
	}
	_, evs, _ := st.IncidentByID(r5.ID)
	if len(evs) < 3 {
		t.Fatalf("kural 5 en az 3 kanıt: %d", len(evs))
	}
}

func TestSeverityMonotonicAndNotify(t *testing.T) {
	e, st := newEngine(t)
	now := time.Now()
	var notified []struct {
		id    int64
		isNew bool
	}
	e.SetNotifier(func(in store.Incident, isNew bool) {
		notified = append(notified, struct {
			id    int64
			isNew bool
		}{in.ID, isNew})
	})

	// önce kural 4 (warn)
	alert(t, st, "anomaly", 3, "warn", now.Add(-4*time.Minute).Unix())
	alert(t, st, "bw", 3, "warn", now.Add(-3*time.Minute).Unix())
	e.Evaluate(now)
	incs, _ := st.ListIncidents(store.IncidentFilter{AgentID: 3})
	if len(incs) != 1 || incs[0].Severity != "warn" {
		t.Fatalf("ilk: %+v", incs)
	}
	first := incs[0]

	// sonra aynı agent'ta crit bw → severity yükselir, risk artar
	alert(t, st, "bw", 3, "crit", now.Add(-1*time.Minute).Unix())
	e.Evaluate(now)
	incs, _ = st.ListIncidents(store.IncidentFilter{AgentID: 3})
	var same *store.Incident
	for i := range incs {
		if incs[i].ID == first.ID {
			same = &incs[i]
		}
	}
	if same == nil || same.Severity != "crit" {
		t.Fatalf("severity yükselmeliydi: %+v", incs)
	}
	if same.RiskScore <= first.RiskScore {
		t.Fatalf("risk artmalıydı: %d → %d", first.RiskScore, same.RiskScore)
	}

	// notify: bir kez isNew=true, sonra raised
	if len(notified) < 2 || !notified[0].isNew || notified[1].isNew {
		t.Fatalf("bildirim akışı hatalı: %+v", notified)
	}
}

func TestDisabledAndAgentlessSkipped(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now()
	// agent_id=0 uyarılar → korelasyon dışı
	alert(t, st, "proc", 0, "info", now.Add(-1*time.Minute).Unix())
	alert(t, st, "target", 0, "info", now.Add(-1*time.Minute).Unix())

	off := New(st, func() Config { c := DefaultConfig(); c.Enabled = false; return c })
	off.Evaluate(now)
	if incs, _ := st.ListIncidents(store.IncidentFilter{}); len(incs) != 0 {
		t.Fatalf("devre dışı motor incident üretmemeli: %d", len(incs))
	}

	on := New(st, func() Config { return DefaultConfig() })
	on.Evaluate(now)
	if incs, _ := st.ListIncidents(store.IncidentFilter{}); len(incs) != 0 {
		t.Fatalf("agent_id=0 uyarılar korele olmamalı: %d", len(incs))
	}
}
