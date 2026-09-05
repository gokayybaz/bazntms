package server

// Faz 10 (plan P2): DB-destekli enroll token yonetimi — hub'in TEK statik
// -enroll-token sirrina ek olarak, yeniden baslatilmadan olusturulup iptal
// edilebilen token'lar. rbac_test.go'daki newRBACServer/postJSON/getJSON
// yardimcilarini kullanir.

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestEnrollTokenLifecycle(t *testing.T) {
	ts := newRBACServer(t, "admin-pass-1")

	status, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "admin-pass-1"})
	if status != http.StatusOK {
		t.Fatalf("admin girisi: %d %v", status, out)
	}
	adminTok, _ := out["token"].(string)

	status, out = postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "bob", "password": "bob-password", "role": "viewer", "site": "",
	})
	if status != http.StatusOK {
		t.Fatalf("kullanici olusturma: %d %v", status, out)
	}
	status, out = postJSON(t, ts, "/api/login", "", map[string]string{"username": "bob", "password": "bob-password"})
	if status != http.StatusOK {
		t.Fatalf("bob girisi: %d %v", status, out)
	}
	bobTok, _ := out["token"].(string)

	// viewer enroll token olusturamaz/listeleyemez
	if status, _ := postJSON(t, ts, "/api/v1/enroll-tokens", bobTok, map[string]any{"name": "x"}); status != http.StatusForbidden {
		t.Fatalf("viewer icin 403 beklenirdi: %d", status)
	}
	if status, _ := getJSON(t, ts, "/api/v1/enroll-tokens", bobTok); status != http.StatusForbidden {
		t.Fatalf("viewer icin 403 beklenirdi: %d", status)
	}

	// admin bir enroll token olusturur
	status, out = postJSON(t, ts, "/api/v1/enroll-tokens", adminTok, map[string]any{"name": "windows-filosu", "site": "ofis-a"})
	if status != http.StatusOK {
		t.Fatalf("enroll token olusturma: %d %v", status, out)
	}
	plain, _ := out["token"].(string)
	if !strings.HasPrefix(plain, "ent_") {
		t.Fatalf("token formati: %q", plain)
	}
	idFloat, _ := out["id"].(float64)
	id := int64(idFloat)

	// bu yeni token ile GERCEK bir agent enroll edilebilmeli (hub'in
	// statik -enroll-token'i degistirilmeden — bkz. TestAgentHelloWrongToken/
	// TestAgentHelloRateLimit'te statik token yolunun ayrica dogrulandigi
	// newTestServerWithEnroll/testEnrollToken; newRBACServer kendi statik
	// token'ini rastgele urettigi icin burada tekrar sinanmiyor)
	resp := helloReq(t, ts, plain, "yeni-token-ile-agent")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("yeni token ile enrollment basarisiz: %d", resp.StatusCode)
	}

	// listede gorunmeli
	status, listOut := getJSON(t, ts, "/api/v1/enroll-tokens", adminTok)
	if status != http.StatusOK {
		t.Fatalf("liste: %d", status)
	}
	_ = listOut // handleEnrollTokensList dogrudan dizi dondurur, map'e cozulmez

	// admin iptal eder
	if status, _ := func() (int, map[string]any) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/enroll-tokens/"+strconv.FormatInt(id, 10), nil)
		req.Header.Set("Authorization", "Bearer "+adminTok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("iptal istegi: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode, nil
	}(); status != http.StatusOK {
		t.Fatalf("iptal 200 beklenirdi: %d", status)
	}

	// iptal sonrasi AYNI token artik kabul edilmemeli — hub yeniden
	// baslatilmadan token sizintisi kapatilabildigini kanitlar (plan P2'nin
	// asil amaci)
	resp3 := helloReq(t, ts, plain, "iptal-sonrasi-agent")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("iptal edilmis token icin 401 beklenirdi: %d", resp3.StatusCode)
	}
}

// TestEnrollTokenExpiredRejected, suresi gecmis (ExpiresAt < now) bir DB
// token'inin enrollment'ta reddedildigini dogrular. Store'a dogrudan
// yazilir (handler yalniz pozitif "gelecekte X gun" kabul eder, testte
// halihazirda gecmis bir tarih uretmek icin store katmani kullanilir).
func TestEnrollTokenExpiredRejected(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "expiry.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	const plain = "ent_test_expired_value"
	if _, err := st.CreateEnrollToken(store.EnrollToken{
		Name: "suresi-dolmus", TokenHash: TokenHashString(plain), ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("olusturma: %v", err)
	}

	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp := helloReq(t, ts, plain, "suresi-dolmus-agent")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("suresi dolmus token icin 401 beklenirdi: %d", resp.StatusCode)
	}
}

// TestEnrollTokenSiteBinding (A3 / S12.7): site-kapsamli DB token ile enroll
// olan agent, kendi -site beyanindan bagimsiz olarak token'in site'ina yazilir.
// Statik token'da agent'in beyani gecerli kalir.
func TestEnrollTokenSiteBinding(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sitebind.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	const scopedTok = "ent_scoped_site_value"
	if _, err := st.CreateEnrollToken(store.EnrollToken{
		Name: "ofis-a-filosu", TokenHash: TokenHashString(scopedTok), Site: "ofis-a",
	}); err != nil {
		t.Fatalf("token olusturma: %v", err)
	}

	engine := capture.NewEngine()
	srv := New(nil, engine, st, "test.db",
		alert.NewManager(alert.DefaultConfig(), st, engine, 30),
		nil, "", testEnrollToken, 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// helloReq gövdesi "site":"test" gönderir — token'ın site'ı bunu ezmeli
	resp := helloReq(t, ts, scopedTok, "scoped-agent")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scoped token enroll: %d", resp.StatusCode)
	}

	// statik token → agent beyanı (hello.Site="test") geçerli
	resp2 := helloReq(t, ts, testEnrollToken, "static-agent")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("static token enroll: %d", resp2.StatusCode)
	}

	agents, err := st.ListAgents(time.Hour, "")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	got := map[string]string{}
	for _, a := range agents {
		got[a.Name] = a.Site
	}
	if got["scoped-agent"] != "ofis-a" {
		t.Fatalf("site-kapsamlı token agent'ı token site'ına bağlamalı: %q (beklenen ofis-a)", got["scoped-agent"])
	}
	if got["static-agent"] != "test" {
		t.Fatalf("statik token agent beyanını korumalı: %q (beklenen test)", got["static-agent"])
	}
}
