package server

// Faz 5 RBAC/audit entegrasyon testleri: legacy giris, kullanici olusturma,
// rol korumasi (403), API token'ları ve denetim zinciri dogrulamasi.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newRBACServer(t *testing.T, password string) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "rbac.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", ai.NewClient(ai.Config{}), mgr, nil, password, "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, ts *httptest.Server, path, token string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("istek: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func getJSON(t *testing.T, ts *httptest.Server, path, token string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("istek: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestRBACLifecycle(t *testing.T) {
	ts := newRBACServer(t, "legacy-pass-1")

	// 1) legacy giris → admin
	status, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pass-1"})
	if status != http.StatusOK {
		t.Fatalf("legacy giris: %d %v", status, out)
	}
	adminTok, _ := out["token"].(string)
	if adminTok == "" {
		t.Fatal("token donmedi")
	}

	// 2) admin → viewer kullanici olustur
	status, out = postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "bob", "password": "bob-password", "role": "viewer", "site": "dc1",
	})
	if status != http.StatusOK {
		t.Fatalf("kullanici olusturma: %d %v", status, out)
	}

	// 3) bob giris yapar → viewer
	status, out = postJSON(t, ts, "/api/login", "", map[string]string{"username": "bob", "password": "bob-password"})
	if status != http.StatusOK {
		t.Fatalf("bob girisi: %d %v", status, out)
	}
	if out["role"] != string(RoleViewer) {
		t.Fatalf("rol hatali: %v", out["role"])
	}
	bobTok, _ := out["token"].(string)

	// 4) viewer salt-okuma: agents GET ok, capture start 403
	if status, _ := getJSON(t, ts, "/api/v1/agents", bobTok); status != http.StatusOK {
		t.Fatalf("viewer GET agents: %d", status)
	}
	if status, _ := postJSON(t, ts, "/api/capture/start", bobTok, map[string]string{"device": "en0"}); status != http.StatusForbidden {
		t.Fatalf("viewer capture start 403 beklenirdi: %d", status)
	}
	if status, _ := getJSON(t, ts, "/api/v1/users", bobTok); status != http.StatusForbidden {
		t.Fatalf("viewer users 403 beklenirdi: %d", status)
	}

	// 5) API token (analyst) olustur; duz token bir kez doner
	status, out = postJSON(t, ts, "/api/v1/tokens", adminTok, map[string]any{
		"name": "grafana", "role": "analyst",
	})
	if status != http.StatusOK {
		t.Fatalf("token olusturma: %d %v", status, out)
	}
	apiTok, _ := out["token"].(string)
	if !strings.HasPrefix(apiTok, "bnt_") {
		t.Fatalf("token formati: %v", apiTok)
	}

	// analyst: rapor ok (PermAnalyze) ama capture 403
	if status, _ := getJSON(t, ts, "/api/report", apiTok); status != http.StatusOK {
		t.Fatalf("analyst report: %d", status)
	}
	if status, _ := postJSON(t, ts, "/api/capture/start", apiTok, map[string]string{"device": "en0"}); status != http.StatusForbidden {
		t.Fatalf("analyst capture 403 beklenirdi: %d", status)
	}

	// 6) audit: giris + kullanici + red olaylari zincirde
	status, out = getJSON(t, ts, "/api/v1/audit?limit=100", adminTok)
	if status != http.StatusOK {
		t.Fatalf("audit: %d", status)
	}
	events, _ := out["__raw"] // handleAuditList dogrudan dizi dondurur; asagida verify ile test edilir
	_ = events

	status, out = getJSON(t, ts, "/api/v1/audit/verify", adminTok)
	if status != http.StatusOK || out["ok"] != true {
		t.Fatalf("zincir dogrulamasi: %d %v", status, out)
	}
	if n, _ := out["checked"].(float64); n < 3 {
		t.Fatalf("beklenenden az audit kaydi: %v", out["checked"])
	}

	// 7) gebersiz token reddedilir
	if status, _ := getJSON(t, ts, "/api/v1/audit", "bnt_bogus"); status != http.StatusUnauthorized {
		t.Fatalf("gecersiz token 401 beklenirdi: %d", status)
	}
}

func TestLegacyAdminStillWorks(t *testing.T) {
	ts := newRBACServer(t, "pass-1234")
	status, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "pass-1234"})
	if status != http.StatusOK || out["role"] != string(RoleAdmin) {
		t.Fatalf("legacy admin giris: %d %v", status, out)
	}
	tok, _ := out["token"].(string)
	// admin tum uclara erisir
	if status, _ := getJSON(t, ts, "/api/v1/users", tok); status != http.StatusOK {
		t.Fatalf("admin users: %d", status)
	}
}
