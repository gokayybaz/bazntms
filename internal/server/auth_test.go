package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newTestServer(t *testing.T, password string) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine)
	srv := New(nil, engine, st, "test.db", ai.NewClient(ai.Config{}), mgr, nil, password)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func login(t *testing.T, ts *httptest.Server, password string) (*http.Cookie, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": password})
	resp, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login istegi: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.Cookies()[0], out.Token
}

func TestAuthDisabled(t *testing.T) {
	ts := newTestServer(t, "")
	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth kapaliyken 200 beklenirdi: %d", resp.StatusCode)
	}
}

func TestLoginFlow(t *testing.T) {
	ts := newTestServer(t, "gizli123")

	// sifresiz istek -> 401
	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("401 beklenirdi: %d", resp.StatusCode)
	}

	// yanlis sifre -> 401
	body, _ := json.Marshal(map[string]string{"password": "yanlis"})
	resp2, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	// dogru sifre -> cookie + token
	cookie, token := login(t, ts, "gizli123")
	if cookie == nil || cookie.Value == "" || token == "" {
		t.Fatal("oturum acilmadi")
	}

	// cookie ile erisim
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/status", nil)
	req.AddCookie(cookie)
	resp3, _ := http.DefaultClient.Do(req)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("cookie ile 200 beklenirdi: %d", resp3.StatusCode)
	}

	// bearer ile erisim
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/status", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp4, _ := http.DefaultClient.Do(req2)
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("bearer ile 200 beklenirdi: %d", resp4.StatusCode)
	}

	// logout sonrasi erisim kesilir
	req3, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/logout", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	resp5, _ := http.DefaultClient.Do(req3)
	resp5.Body.Close()
	req4, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/status", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	resp6, _ := http.DefaultClient.Do(req4)
	resp6.Body.Close()
	if resp6.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logout sonrasi 401 beklenirdi: %d", resp6.StatusCode)
	}
}

func TestAuthStatusEndpoint(t *testing.T) {
	ts := newTestServer(t, "gizli123")
	resp, err := http.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Required      bool `json:"required"`
		Authenticated bool `json:"authenticated"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if !out.Required || out.Authenticated {
		t.Fatalf("durum hatali: %+v", out)
	}
}

func TestStaticNotBlocked(t *testing.T) {
	ts := newTestServer(t, "gizli123")
	// staticFS nil: kok 404 doner AMA 401 donmemeli (auth middleware'i statige girmez)
	resp, err := http.Get(ts.URL + "/bir-sey")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("statik yollar auth'a takilmamali")
	}
}

func TestRateLimit(t *testing.T) {
	ts := newTestServer(t, "gizli123")
	body, _ := json.Marshal(map[string]string{"password": "yanlis"})
	blocked := false
	for i := 0; i < 8; i++ {
		resp, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			blocked = true
		} else if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("beklenmedik durum: %d", resp.StatusCode)
		}
	}
	if !blocked {
		t.Fatal("5 denemeden sonra blok beklenirdi")
	}
}
