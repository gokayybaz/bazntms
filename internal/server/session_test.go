package server

// Faz 15 S15.3 (A4): -session-store=db ile oturumlar paylaşımlı; iki hub
// replikası aynı DB'yi paylaşır → birinde giriş, diğerinde oturum geçerli.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newHubOnStore(t *testing.T, st store.Store, dbSessions bool) *httptest.Server {
	t.Helper()
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "shared-pass", "", 30, false, nil, nil, nil)
	if dbSessions {
		srv.UseDBSessions(context.Background())
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestDBSessionsSharedAcrossReplicas(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "shared.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	repA := newHubOnStore(t, st, true)
	repB := newHubOnStore(t, st, true)

	// A'da giriş
	status, out := postJSON(t, repA, "/api/login", "", map[string]string{"password": "shared-pass"})
	if status != http.StatusOK {
		t.Fatalf("A girişi: %d", status)
	}
	tok, _ := out["token"].(string)

	// B'de aynı token ile korumalı uç → 200
	if code, _ := getJSON(t, repB, "/api/v1/users", tok); code != http.StatusOK {
		t.Fatalf("B'de paylaşımlı oturum geçerli olmalıydı: %d", code)
	}

	// A'da logout → B'de de geçersiz (replikalar arası)
	req, _ := http.NewRequest(http.MethodPost, repA.URL+"/api/logout", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	http.DefaultClient.Do(req)

	if code, _ := getJSON(t, repB, "/api/v1/users", tok); code != http.StatusUnauthorized {
		t.Fatalf("logout sonrası B'de oturum düşmeliydi: %d", code)
	}
}

// TestDBSessionsRestartSurvives, controller yeniden başlasa bile (yeni Server
// aynı DB üzerinde) oturum korunur.
func TestDBSessionsRestartSurvives(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "restart.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	rep1 := newHubOnStore(t, st, true)
	_, out := postJSON(t, rep1, "/api/login", "", map[string]string{"password": "shared-pass"})
	tok, _ := out["token"].(string)

	// "yeniden başlatma" — yeni Server, aynı store
	rep2 := newHubOnStore(t, st, true)
	if code, _ := getJSON(t, rep2, "/api/v1/users", tok); code != http.StatusOK {
		t.Fatalf("restart sonrası oturum korunmalıydı: %d", code)
	}
}

// TestMemSessionsNotShared, varsayılan bellek modunda oturumlar paylaşılmaz
// (regresyon: db modu istemeden global olmasın).
func TestMemSessionsNotShared(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	repA := newHubOnStore(t, st, false)
	repB := newHubOnStore(t, st, false)
	_, out := postJSON(t, repA, "/api/login", "", map[string]string{"password": "shared-pass"})
	tok, _ := out["token"].(string)

	if code, _ := getJSON(t, repA, "/api/v1/users", tok); code != http.StatusOK {
		t.Fatalf("A kendi oturumunu görmeli: %d", code)
	}
	if code, _ := getJSON(t, repB, "/api/v1/users", tok); code != http.StatusUnauthorized {
		t.Fatalf("bellek modunda B oturumu görmemeli: %d", code)
	}
}
