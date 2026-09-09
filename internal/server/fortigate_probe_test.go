package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// fakeFortiGate, probe testleri için FortiOS 7.2 benzeri mock.
func fakeFortiGate(t *testing.T) string {
	t.Helper()
	env := func(results string) string {
		return `{"http_method":"GET","results":` + results + `,"status":"success","version":"v7.2.11","build":1639}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/monitor/system/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env(`{"hostname":"fgt-lab","serial":"FG100","version":"v7.2.11","build":1639}`)))
	})
	mux.HandleFunc("/api/v2/cmdb/system/vdom", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env(`[{"name":"root"}]`)))
	})
	mux.HandleFunc("/api/v2/monitor/system/interface", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env(`{"port1":{"name":"port1","link":true,"speed":1000.0}}`)))
	})
	mux.HandleFunc("/api/v2/monitor/firewall/policy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env(`[]`)))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// vaultServer, kimlik kasası bağlı bir RBAC test sunucusu (fortigate token
// şifreleme/çözme yolu için).
func vaultServer(t *testing.T, password string) (*httptest.Server, store.Store) {
	t.Helper()
	ts, srv, st := newRBACServerEx(t, password, "", false)
	v, err := vault.Open(filepath.Join(t.TempDir(), "vault.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	srv.SetVault(v)
	return ts, st
}

func TestDeviceProbeNewAndExisting(t *testing.T) {
	ts, st := vaultServer(t, "admin-probe-pw")
	_, admin := login(t, ts, "admin-probe-pw")
	fgURL := fakeFortiGate(t)

	// 1. yeni cihaz sınaması (kayıt öncesi)
	code, out := postJSON(t, ts, "/api/v1/devices/probe", admin, map[string]any{
		"api_url": fgURL, "api_token": "x", "api_verify_tls": false, "vdom": "root",
	})
	if code != http.StatusOK {
		t.Fatalf("probe: %d %v", code, out)
	}
	if out["version"] != "v7.2.11" || out["profile_id"] != "7.2" || out["vdom_mode"] != "single" {
		t.Fatalf("probe raporu: %v", out)
	}
	caps, _ := out["caps"].(map[string]any)
	if caps["interface"] != "ok" || caps["policy_mon"] != "denied" {
		t.Fatalf("caps: %v", caps)
	}

	// 2. cihazı kaydet, sonra device_id + profil pini ile yeniden sına
	code, _ = postJSON(t, ts, "/api/v1/devices", admin, map[string]any{
		"name": "fgt-lab", "host": "10.0.0.1", "kind": "firewall", "vendor": "fortigate",
		"api_url": fgURL, "api_token": "x", "api_verify_tls": false, "vdom": "root",
	})
	if code != http.StatusOK {
		t.Fatalf("cihaz ekle: %d", code)
	}
	devs, _ := st.ListDevices("")
	id := devs[0].ID

	code, out = postJSON(t, ts, "/api/v1/devices/probe", admin, map[string]any{
		"device_id": id, "profile": "7.4",
	})
	if code != http.StatusOK {
		t.Fatalf("re-probe: %d %v", code, out)
	}
	d, _ := st.DeviceByID(id)
	if d.APIVersion != "v7.2.11" || d.APIProfile != "7.4" || d.APICaps == "" {
		t.Fatalf("probe sonrası cihaz meta: %+v", d)
	}

	// 3. PUT /fortigate ile profili auto'ya çevir
	code, _ = putJSON(t, ts, "/api/v1/devices/"+strconv.FormatInt(id, 10)+"/fortigate", admin, map[string]any{"profile": ""})
	if code != http.StatusOK {
		t.Fatalf("fortigate config: %d", code)
	}
	if d, _ := st.DeviceByID(id); d.APIProfile != "" {
		t.Fatalf("profil temizlenmedi: %q", d.APIProfile)
	}
}

func TestDeviceProbeRBAC(t *testing.T) {
	ts, _, _ := newRBACServerEx(t, "admin-probe-rbac-pw", "", false)
	_, admin := login(t, ts, "admin-probe-rbac-pw")
	postJSON(t, ts, "/api/v1/users", admin, map[string]any{"username": "ann", "password": "ann-password-1", "role": "analyst"})
	_, lo := postJSON(t, ts, "/api/login", "", map[string]string{"username": "ann", "password": "ann-password-1"})
	analyst, _ := lo["token"].(string)

	code, _ := postJSON(t, ts, "/api/v1/devices/probe", analyst, map[string]any{"api_url": "https://x", "api_token": "y"})
	if code != http.StatusForbidden {
		t.Fatalf("analyst probe reddedilmeliydi: %d", code)
	}
}
