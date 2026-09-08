package server

// S22.11: GET /api/v1/alerts/events (filtre) + POST :id/{ack,resolve,note}
// + site-kapsam.

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAlertEventsV1(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "aev.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	// admin şifresi ile korumalı → RBAC devrede
	srv := New(nil, engine, st, "test.db", mgr, nil, "gizli", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	now := time.Now().Unix()
	id1, _ := st.InsertAlertEvent(store.AlertEvent{Ts: now, Kind: "ioc", Key: "evil.com", Message: "tehdit", Severity: "crit", Site: "dc1"})
	st.InsertAlertEvent(store.AlertEvent{Ts: now, Kind: "bw", Key: "x", Message: "b", Severity: "warn", Site: "dc2"})

	adminTok := userToken(t, ts, "", "gizli")

	// filtre: severity=crit → 1
	code, out := getJSON(t, ts, "/api/v1/alerts/events?severity=crit", adminTok)
	if code != 200 {
		t.Fatalf("list: %d", code)
	}
	evs, _ := out["events"].([]any)
	if len(evs) != 1 {
		t.Fatalf("crit filtresi 1 beklenirdi: %+v", out)
	}

	// ack
	code, _ = postJSON(t, ts, "/api/v1/alerts/events/"+i64(id1)+"/ack", adminTok, map[string]any{"note": "bakılıyor"})
	if code != 200 {
		t.Fatalf("ack: %d", code)
	}
	e, _ := st.AlertEventByID(id1)
	if e.State != "ack" || e.Note != "bakılıyor" {
		t.Fatalf("ack sonrası: %+v", e)
	}

	// resolve
	code, _ = postJSON(t, ts, "/api/v1/alerts/events/"+i64(id1)+"/resolve", adminTok, nil)
	if code != 200 {
		t.Fatalf("resolve: %d", code)
	}
	if e, _ := st.AlertEventByID(id1); e.State != "resolved" {
		t.Fatalf("resolve sonrası: %+v", e)
	}

	// site-admin: yalnız kendi sahası
	postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{"username": "sa2", "password": "password-123", "role": "site-admin", "site": "dc2"})
	saTok := userToken(t, ts, "sa2", "password-123")
	code, out = getJSON(t, ts, "/api/v1/alerts/events", saTok)
	if code != 200 {
		t.Fatalf("sa list: %d", code)
	}
	for _, ev := range out["events"].([]any) {
		if ev.(map[string]any)["site"] != "dc2" {
			t.Fatalf("site-admin dc2 dışı olay gördü: %+v", ev)
		}
	}
	// site-admin dc1 olayına dokunamaz
	code, _ = postJSON(t, ts, "/api/v1/alerts/events/"+i64(id1)+"/note", saTok, map[string]any{"note": "x"})
	if code != http.StatusNotFound {
		t.Fatalf("site-admin başka saha olayı: 404 beklenirdi, %d", code)
	}
}
