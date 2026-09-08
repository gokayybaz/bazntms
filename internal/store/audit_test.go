package store

import (
	"strings"
	"testing"
)

// TestAuditChainStableAcrossV2, v1 (yalnız temel alan) ve v2 (actor_type/
// result/before/after dolu) kayıtları aynı zincire eklenince VerifyAuditChain
// hâlâ sağlam demeli — yeni alanlar hash'e koşullu katıldığı için eski
// kayıtların hash'i değişmez.
func TestAuditChainStableAcrossV2(t *testing.T) {
	st := openTest(t)

	// v1 tarzı: yalnız temel alanlar (actor_type vs. boş)
	if _, err := st.InsertAuditEvent(AuditEvent{Username: "admin", Role: "admin", Action: "login", Target: "legacy"}); err != nil {
		t.Fatalf("v1 ekleme: %v", err)
	}
	if _, err := st.InsertAuditEvent(AuditEvent{Username: "admin", Role: "admin", Action: "device.add", Target: "device:1"}); err != nil {
		t.Fatalf("v1 ekleme 2: %v", err)
	}
	// v2: yeni alanlar dolu
	if _, err := st.InsertAuditEvent(AuditEvent{
		Username: "admin", Role: "admin", Action: "alerts.update", Target: "",
		ActorType: "user", Result: "ok", RequestID: "abc123",
		BeforeJSON: `{"x":1}`, AfterJSON: `{"x":2}`,
	}); err != nil {
		t.Fatalf("v2 ekleme: %v", err)
	}
	if _, err := st.InsertAuditEvent(AuditEvent{
		Username: "-", Action: "login.failed", Target: "user:bob",
		ActorType: "-", Result: "error",
	}); err != nil {
		t.Fatalf("v2 ekleme 2: %v", err)
	}

	ok, brokenAt, checked, err := st.VerifyAuditChain()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatalf("zincir bozuk raporlandı (kayıt #%d)", brokenAt)
	}
	if checked != 4 {
		t.Fatalf("4 kayıt beklenirdi, doğrulanan: %d", checked)
	}
}

// TestQueryAuditEventsFilters, süzgeçlerin (actor/action/resource/result/
// zaman) parametreli sorguda çalıştığını doğrular.
func TestQueryAuditEventsFilters(t *testing.T) {
	st := openTest(t)
	seed := []AuditEvent{
		{Username: "alice", Role: "admin", Action: "user.update", Target: "user:bob", ActorType: "user", Result: "ok"},
		{Username: "alice", Role: "admin", Action: "user.delete", Target: "user:carl", ActorType: "user", Result: "ok"},
		{Username: "bob", Role: "netops", Action: "device.add", Target: "device:9", ActorType: "user", Result: "ok"},
		{Username: "bob", Role: "netops", Action: "denied", Target: "GET /api/v1/users", ActorType: "user", Result: "denied"},
	}
	for _, e := range seed {
		if _, err := st.InsertAuditEvent(e); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// actor süzgeci
	got, err := st.QueryAuditEvents(AuditFilter{Actor: "alice"})
	if err != nil {
		t.Fatalf("actor sorgu: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("alice: 2 beklenirdi, gelen %d", len(got))
	}

	// action prefix süzgeci ("user.*")
	got, _ = st.QueryAuditEvents(AuditFilter{Action: "user.*"})
	if len(got) != 2 {
		t.Fatalf("user.*: 2 beklenirdi, gelen %d", len(got))
	}

	// action tam eşleşme
	got, _ = st.QueryAuditEvents(AuditFilter{Action: "device.add"})
	if len(got) != 1 || got[0].Username != "bob" {
		t.Fatalf("device.add: 1 (bob) beklenirdi, gelen %+v", got)
	}

	// resource (target LIKE)
	got, _ = st.QueryAuditEvents(AuditFilter{Resource: "user:bob"})
	if len(got) != 1 || got[0].Action != "user.update" {
		t.Fatalf("resource user:bob: user.update beklenirdi, gelen %+v", got)
	}

	// result süzgeci
	got, _ = st.QueryAuditEvents(AuditFilter{Result: "denied"})
	if len(got) != 1 || got[0].Action != "denied" {
		t.Fatalf("result denied: 1 beklenirdi, gelen %+v", got)
	}

	// RecentAuditEvents hâlâ çalışıyor (sarmalayıcı)
	all, _ := st.RecentAuditEvents(100, "")
	if len(all) != 4 {
		t.Fatalf("RecentAuditEvents: 4 beklenirdi, gelen %d", len(all))
	}
}

// TestAuditUserAgentTruncated, 256'dan uzun user-agent kırpılır (hash'e
// girdiği için sınır önemli).
func TestAuditUserAgentTruncated(t *testing.T) {
	st := openTest(t)
	long := strings.Repeat("x", 400)
	if _, err := st.InsertAuditEvent(AuditEvent{Username: "a", Action: "login", ActorType: "user", Result: "ok", UserAgent: long}); err != nil {
		t.Fatalf("ekleme: %v", err)
	}
	got, _ := st.QueryAuditEvents(AuditFilter{})
	if len(got) != 1 || len(got[0].UserAgent) != 256 {
		t.Fatalf("user-agent 256'ya kırpılmalıydı, uzunluk: %d", len(got[0].UserAgent))
	}
	if ok, at, _, _ := st.VerifyAuditChain(); !ok {
		t.Fatalf("kırpma sonrası zincir bozuk (#%d)", at)
	}
}
