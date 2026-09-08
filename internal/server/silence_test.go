package server

// S22.10: /api/v1/alerts/silences — POST/GET/DELETE.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestSilenceEndpoints(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sil.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// oluştur
	body, _ := json.Marshal(map[string]any{"match_kind": "anomaly", "reason": "bakım", "duration_min": 60})
	resp, err := http.Post(ts.URL+"/api/v1/alerts/silences", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("post: %v %v", err, resp.StatusCode)
	}
	var pr struct{ ID int64 }
	json.NewDecoder(resp.Body).Decode(&pr)
	resp.Body.Close()
	if pr.ID == 0 {
		t.Fatalf("id boş")
	}

	// listele (active)
	code, out := getJSON(t, ts, "/api/v1/alerts/silences?active=1", "")
	if code != 200 {
		t.Fatalf("get: %d", code)
	}
	sl, _ := out["silences"].([]any)
	if len(sl) != 1 {
		t.Fatalf("1 susturma bekleniyordu: %+v", out)
	}

	// motor susturmayı görüyor mu (AddSilence RefreshSilences çağırdı)
	if act, _ := mgr.ListSilences(true); len(act) != 1 {
		t.Fatalf("motor aktif susturmayı listelemeli: %d", len(act))
	}

	// sil
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/alerts/silences/"+fmt.Sprint(pr.ID), nil)
	dresp, _ := http.DefaultClient.Do(req)
	if dresp.StatusCode != 200 {
		t.Fatalf("delete: %d", dresp.StatusCode)
	}
	dresp.Body.Close()
	code, out = getJSON(t, ts, "/api/v1/alerts/silences?active=1", "")
	if sl, _ := out["silences"].([]any); len(sl) != 0 {
		t.Fatalf("silme sonrası 0 bekleniyordu: %+v", out)
	}
}
