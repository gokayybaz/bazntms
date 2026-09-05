package server

// Faz 5 RBAC/audit entegrasyon testleri: legacy giris, kullanici olusturma,
// rol korumasi (403), API token'ları ve denetim zinciri dogrulamasi.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	srv := New(nil, engine, st, "test.db", mgr, nil, password, "", 30, false, nil, nil, nil)
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

	// 4) viewer salt-okuma: agents GET ok, cihaz ekleme (PermManageDevices) 403
	if status, _ := getJSON(t, ts, "/api/v1/agents", bobTok); status != http.StatusOK {
		t.Fatalf("viewer GET agents: %d", status)
	}
	if status, _ := postJSON(t, ts, "/api/v1/devices", bobTok, map[string]string{"name": "sw1", "host": "10.0.0.1"}); status != http.StatusForbidden {
		t.Fatalf("viewer cihaz ekleme 403 beklenirdi: %d", status)
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

	// analyst: rapor ok (PermAnalyze) ama cihaz ekleme (PermManageDevices) 403
	if status, _ := getJSON(t, ts, "/api/report", apiTok); status != http.StatusOK {
		t.Fatalf("analyst report: %d", status)
	}
	if status, _ := postJSON(t, ts, "/api/v1/devices", apiTok, map[string]string{"name": "sw1", "host": "10.0.0.1"}); status != http.StatusForbidden {
		t.Fatalf("analyst cihaz ekleme 403 beklenirdi: %d", status)
	}

	// 6) audit: giris + kullanici + red olaylari zincirde.
	// audit listesi handleAuditList'ten dizi doner (getJSON map'e parse
	// edemez); icerik asagida /api/v1/audit/verify ile dogrulanir.
	status, _ = getJSON(t, ts, "/api/v1/audit?limit=100", adminTok)
	if status != http.StatusOK {
		t.Fatalf("audit: %d", status)
	}

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

// TestLegacyLoginDisabledWhenAdminExists (B6): users boşken legacy tek-şifre
// çalışır; etkin bir admin RBAC kullanıcısı eklenince legacy giriş reddedilir.
// Etkin admin kalmayınca legacy "break-glass" olarak geri döner.
func TestLegacyLoginDisabledWhenAdminExists(t *testing.T) {
	ts := newRBACServer(t, "legacy-pw-1")

	// 1) users yok → legacy çalışır
	status, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pw-1"})
	if status != http.StatusOK {
		t.Fatalf("users boşken legacy giriş: %d %v", status, out)
	}
	legacyTok, _ := out["token"].(string)

	// 2) analyst (admin DEĞİL) ekle → legacy hâlâ çalışmalı
	if code, o := postJSON(t, ts, "/api/v1/users", legacyTok, map[string]any{
		"username": "ana", "password": "ana-pass-1", "role": "analyst",
	}); code != http.StatusOK {
		t.Fatalf("analyst oluşturma: %d %v", code, o)
	}
	if code, _ := postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pw-1"}); code != http.StatusOK {
		t.Fatalf("analyst varken legacy hâlâ çalışmalı: %d", code)
	}

	// 3) admin RBAC kullanıcısı ekle → legacy reddedilir
	code, o := postJSON(t, ts, "/api/v1/users", legacyTok, map[string]any{
		"username": "adm", "password": "adm-pass-1", "role": "admin",
	})
	if code != http.StatusOK {
		t.Fatalf("admin oluşturma: %d %v", code, o)
	}
	admID := int64(o["id"].(float64))

	code, o = postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pw-1"})
	if code != http.StatusUnauthorized {
		t.Fatalf("admin RBAC varken legacy 401 beklenirdi: %d %v", code, o)
	}
	if msg, _ := o["error"].(string); !strings.Contains(msg, "RBAC") {
		t.Fatalf("mesaj RBAC'a işaret etmeli: %q", msg)
	}

	// 4) yeni admin kullanıcısı ile giriş → 200
	if code, _ := postJSON(t, ts, "/api/login", "", map[string]string{"username": "adm", "password": "adm-pass-1"}); code != http.StatusOK {
		t.Fatalf("yeni admin girişi: %d", code)
	}

	// 5) admin kullanıcısını pasifleştir → legacy tekrar çalışır (break-glass)
	if code, o := putJSON(t, ts, "/api/v1/users/"+strconv.FormatInt(admID, 10), legacyTok, map[string]any{"enabled": false}); code != http.StatusOK {
		t.Fatalf("admin pasifleştirme: %d %v", code, o)
	}
	if code, _ := postJSON(t, ts, "/api/login", "", map[string]string{"password": "legacy-pw-1"}); code != http.StatusOK {
		t.Fatalf("etkin admin kalmayınca legacy geri dönmeli: %d", code)
	}
}
