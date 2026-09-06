package server

// Faz 14B S14.B2: site-admin rolü + kapsam denetimi.
// Kapsamlı çapraz-saha sızıntı testi: site_leak_test.go (S14.B4).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

func delReq(t *testing.T, ts *httptest.Server, path, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+path, bytes.NewReader(nil))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete istegi: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// getList, bir JSON dizisi döndüren ucu []map[string]any olarak okur.
func getList(t *testing.T, ts *httptest.Server, path, token string) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("liste istegi: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: %d beklendi 200", path, resp.StatusCode)
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("%s çözülemedi: %v", path, err)
	}
	return out
}

func storeAgent(name, site string) store.Agent {
	return store.Agent{Name: name, Site: site, TokenHash: store.TokenHash(name + "-tok")}
}

// TestRoleSiteConsistency, S14.B2: admin rolü siteye bağlanamaz, site-admin
// rolü site zorunlu.
func TestRoleSiteConsistency(t *testing.T) {
	ts := newRBACServer(t, "admin-pass-1")
	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "admin-pass-1"})
	adminTok, _ := out["token"].(string)

	if st, _ := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "bad-admin", "password": "password-123", "role": "admin", "site": "dc1",
	}); st != http.StatusBadRequest {
		t.Fatalf("siteye bağlı admin 400 beklenirdi: %d", st)
	}
	if st, _ := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "bad-siteadmin", "password": "password-123", "role": "site-admin", "site": "",
	}); st != http.StatusBadRequest {
		t.Fatalf("site'siz site-admin 400 beklenirdi: %d", st)
	}
	if st, _ := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "ok-siteadmin", "password": "password-123", "role": "site-admin", "site": "dc1",
	}); st != http.StatusOK {
		t.Fatalf("geçerli site-admin oluşturulmalı: %d", st)
	}
}

// TestSiteAdminScope, S14.B2: site-admin yalnız kendi sahasının kullanıcı /
// token / enroll-token / agent'ını yönetir; global admin / ISMS / alert-config
// erişemez.
func TestSiteAdminScope(t *testing.T) {
	ts, _, st := newRBACServerEx(t, "admin-pass-1", "", false)
	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "admin-pass-1"})
	adminTok, _ := out["token"].(string)

	mkUser := func(name, role, site string) {
		t.Helper()
		if s, o := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
			"username": name, "password": "password-123", "role": role, "site": site,
		}); s != http.StatusOK {
			t.Fatalf("kullanıcı %s: %d %v", name, s, o)
		}
	}
	mkUser("sa-dc1", "site-admin", "dc1")
	mkUser("viewer-dc2", "viewer", "dc2")

	saTok := userToken(t, ts, "sa-dc1", "password-123")

	// 1) kullanıcı listesi yalnız dc1
	users := getList(t, ts, "/api/v1/users", saTok)
	if len(users) == 0 {
		t.Fatal("site-admin en az kendini görmeli")
	}
	for _, u := range users {
		if u["site"] != "dc1" {
			t.Fatalf("site-admin başka sahanın kullanıcısını gördü: %v", u)
		}
	}

	// 2) kendi sahasına kullanıcı açar (site zorlanır)
	if s, _ := postJSON(t, ts, "/api/v1/users", saTok, map[string]any{
		"username": "new-dc1", "password": "password-123", "role": "viewer", "site": "dc2",
	}); s != http.StatusOK {
		t.Fatalf("site-admin kendi sahasına kullanıcı açabilmeli: %d", s)
	}
	for _, u := range getList(t, ts, "/api/v1/users", adminTok) {
		if u["username"] == "new-dc1" && u["site"] != "dc1" {
			t.Fatalf("site-admin'in açtığı kullanıcı kendi sahasına sabitlenmeli: %v", u)
		}
	}

	// 3) global admin oluşturamaz
	if s, _ := postJSON(t, ts, "/api/v1/users", saTok, map[string]any{
		"username": "sneaky", "password": "password-123", "role": "admin",
	}); s != http.StatusForbidden {
		t.Fatalf("site-admin global admin oluşturamamalı: %d", s)
	}

	// 4) başka sahanın kullanıcısını düzenleyemez / silemez
	var viewerID int64
	for _, u := range getList(t, ts, "/api/v1/users", adminTok) {
		if u["username"] == "viewer-dc2" {
			viewerID = int64(u["id"].(float64))
		}
	}
	if s, _ := putJSON(t, ts, "/api/v1/users/"+strconv.FormatInt(viewerID, 10), saTok, map[string]any{"enabled": false}); s != http.StatusForbidden {
		t.Fatalf("site-admin başka sahanın kullanıcısını düzenleyememeli: %d", s)
	}
	if s := delReq(t, ts, "/api/v1/users/"+strconv.FormatInt(viewerID, 10), saTok); s != http.StatusForbidden {
		t.Fatalf("site-admin başka sahanın kullanıcısını silememeli: %d", s)
	}

	// 5) ISMS + alert config → global-admin (403)
	if s, _ := getJSON(t, ts, "/api/v1/isms/summary", saTok); s != http.StatusForbidden {
		t.Fatalf("site-admin ISMS görememeli: %d", s)
	}
	if s, _ := getJSON(t, ts, "/api/alerts", saTok); s != http.StatusForbidden {
		t.Fatalf("site-admin alert config görememeli: %d", s)
	}

	// 6) başka sahanın agent'ını silemez (IDOR)
	otherID, _ := st.RegisterAgent(storeAgent("dc2-agent", "dc2"))
	if s := delReq(t, ts, "/api/v1/agents/"+strconv.FormatInt(otherID, 10), saTok); s != http.StatusNotFound {
		t.Fatalf("site-admin başka sahanın agent'ını silememeli: %d", s)
	}
}
