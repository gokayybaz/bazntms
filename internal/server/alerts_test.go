package server

// S11.3 + S11.4 (B1): GET /api/alerts admin-korumalı olmalı ve yanıttaki
// bildirim sırları maskeli dönmeli; maskeli değer PUT edilince saklı sır
// bozulmamalı.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newAlertsTestServer(t *testing.T, password string) (*httptest.Server, *alert.Manager) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "alerts.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, password, "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, mgr
}

func putJSON(t *testing.T, ts *httptest.Server, path, token string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+path, bytes.NewReader(b))
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
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func userToken(t *testing.T, ts *httptest.Server, username, password string) string {
	t.Helper()
	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"username": username, "password": password})
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatalf("%s girişi başarısız: %v", username, out)
	}
	return tok
}

func TestAlertsGetRequiresAdmin(t *testing.T) {
	ts, _ := newAlertsTestServer(t, "admin-pass-1")

	_, adminTok := login(t, ts, "admin-pass-1") // legacy şifre → admin
	if adminTok == "" {
		t.Fatal("admin girişi başarısız")
	}

	if st, _ := getJSON(t, ts, "/api/alerts", adminTok); st != http.StatusOK {
		t.Fatalf("admin GET /api/alerts: %d beklendi 200", st)
	}

	// viewer + analyst → 403 (PermAdmin gerek)
	for _, u := range []struct{ name, role string }{{"vera", "viewer"}, {"andy", "analyst"}} {
		if st, out := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
			"username": u.name, "password": u.name + "-pass-1", "role": u.role,
		}); st != http.StatusOK {
			t.Fatalf("%s oluşturma: %d %v", u.role, st, out)
		}
		tok := userToken(t, ts, u.name, u.name+"-pass-1")
		if st, _ := getJSON(t, ts, "/api/alerts", tok); st != http.StatusForbidden {
			t.Fatalf("%s GET /api/alerts: %d beklendi 403", u.role, st)
		}
	}
}

func TestAlertsSecretMaskRoundTrip(t *testing.T) {
	ts, mgr := newAlertsTestServer(t, "admin-pass-2")
	_, adminTok := login(t, ts, "admin-pass-2")

	// 1) gerçek sırları içeren config'i PUT et
	cfg := alert.DefaultConfig()
	cfg.Notifiers.TelegramToken = "123456:REAL-telegram-token"
	cfg.Notifiers.TelegramChatID = "-100999"
	cfg.Notifiers.EmailPass = "REAL-smtp-password"
	cfg.Notifiers.WebhookV2Secret = "REAL-hmac-secret"
	cfg.Notifiers.SIEM.Token = "Splunk REAL-hec-token"
	cfg.Notifiers.SIEM.Target = "siem.local:514"
	if st, out := putJSON(t, ts, "/api/alerts", adminTok, cfg); st != http.StatusOK || out["ok"] != true {
		t.Fatalf("ilk PUT: %d %v", st, out)
	}

	// 2) GET → sırlar maskeli, sır olmayan alanlar açık
	_, got := getJSON(t, ts, "/api/alerts", adminTok)
	nf, _ := got["notifiers"].(map[string]any)
	if nf == nil {
		t.Fatalf("notifiers yok: %v", got)
	}
	for _, k := range []string{"telegram_token", "email_pass", "webhook_v2_secret"} {
		if nf[k] != secretMask {
			t.Fatalf("%s maskeli değil: %v", k, nf[k])
		}
	}
	if siem, _ := nf["siem"].(map[string]any); siem["token"] != secretMask {
		t.Fatalf("siem.token maskeli değil: %v", nf["siem"])
	}
	if nf["telegram_chat_id"] != "-100999" {
		t.Fatalf("sır olmayan telegram_chat_id bozuldu: %v", nf["telegram_chat_id"])
	}

	// 3) maskeli config'i geri PUT et → saklı sırlar bozulmamalı
	maskedBack := alert.DefaultConfig()
	maskedBack.Notifiers.TelegramToken = secretMask
	maskedBack.Notifiers.EmailPass = secretMask
	maskedBack.Notifiers.WebhookV2Secret = secretMask
	maskedBack.Notifiers.SIEM.Token = secretMask
	maskedBack.Notifiers.TelegramChatID = "-100111" // sır olmayan alanı değiştir
	if st, _ := putJSON(t, ts, "/api/alerts", adminTok, maskedBack); st != http.StatusOK {
		t.Fatalf("maskeli PUT: %d", st)
	}
	stored := mgr.Config()
	if stored.Notifiers.TelegramToken != "123456:REAL-telegram-token" {
		t.Fatalf("telegram_token maskeli PUT ile bozuldu: %q", stored.Notifiers.TelegramToken)
	}
	if stored.Notifiers.EmailPass != "REAL-smtp-password" {
		t.Fatalf("email_pass bozuldu: %q", stored.Notifiers.EmailPass)
	}
	if stored.Notifiers.WebhookV2Secret != "REAL-hmac-secret" {
		t.Fatalf("webhook_v2_secret bozuldu: %q", stored.Notifiers.WebhookV2Secret)
	}
	if stored.Notifiers.SIEM.Token != "Splunk REAL-hec-token" {
		t.Fatalf("siem.token bozuldu: %q", stored.Notifiers.SIEM.Token)
	}
	if stored.Notifiers.TelegramChatID != "-100111" {
		t.Fatalf("sır olmayan alan güncellenmedi: %q", stored.Notifiers.TelegramChatID)
	}

	// 4) yeni gerçek değer PUT edilince güncellenir; maskeli alan korunur
	upd := alert.DefaultConfig()
	upd.Notifiers.TelegramToken = "999999:NEW-telegram-token"
	upd.Notifiers.EmailPass = secretMask
	if st, _ := putJSON(t, ts, "/api/alerts", adminTok, upd); st != http.StatusOK {
		t.Fatalf("güncelleme PUT: %d", st)
	}
	stored = mgr.Config()
	if stored.Notifiers.TelegramToken != "999999:NEW-telegram-token" {
		t.Fatalf("yeni telegram_token yazılmadı: %q", stored.Notifiers.TelegramToken)
	}
	if stored.Notifiers.EmailPass != "REAL-smtp-password" {
		t.Fatalf("maskeli email_pass yine korunmalıydı: %q", stored.Notifiers.EmailPass)
	}
}
