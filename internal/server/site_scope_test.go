package server

import (
	"encoding/json"
	"io"
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
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestSiteScopeIsolation, site-sinirli bir kimligin (API token, site="ofis-a")
// yalnizca kendi sitesine ait cihazlari, NetFlow'u ve syslog'u gordugunu;
// baska sitenin cihaz detayina 404 aldigini dogrular.
func TestSiteScopeIsolation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "pw", "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// iki sitede birer cihaz + her sitenin exporter'indan NetFlow + syslog
	idA, _ := st.AddDevice(store.Device{Name: "rtr-a", Host: "10.1.0.1", Kind: "router", Site: "ofis-a", Vendor: "snmp"})
	idB, _ := st.AddDevice(store.Device{Name: "rtr-b", Host: "10.2.0.1", Kind: "router", Site: "ofis-b", Vendor: "snmp"})
	now := time.Now()
	if err := st.SaveFlows([]store.FlowRow{
		{Ts: now.Unix(), Device: "10.1.0.1", Src: "10.1.0.5", Dst: "8.8.8.8", Proto: "udp", Octets: 500},
		{Ts: now.Unix(), Device: "10.2.0.1", Src: "10.2.0.5", Dst: "1.1.1.1", Proto: "udp", Octets: 999},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}
	_ = st.SaveSyslogEvent(store.SyslogEvent{Ts: now.Unix(), Host: "rtr-a", SourceIP: "10.1.0.1", Severity: 5, Message: "a-event"})
	_ = st.SaveSyslogEvent(store.SyslogEvent{Ts: now.Unix(), Host: "rtr-b", SourceIP: "10.2.0.1", Severity: 5, Message: "b-event"})

	// site-a'ya sinirli API token
	tok := "site-a-token-value"
	if _, err := st.CreateAPIToken(store.APIToken{Name: "sube-a", TokenHash: store.TokenHash(tok), Role: "netops", Site: "ofis-a"}); err != nil {
		t.Fatalf("token: %v", err)
	}

	get := func(path string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	// cihaz listesi: yalnizca rtr-a
	code, body := get("/api/v1/devices")
	if code != 200 || !strings.Contains(body, "rtr-a") || strings.Contains(body, "rtr-b") {
		t.Fatalf("devices site sızıntısı: %d %s", code, body)
	}
	// NetFlow: yalnizca site-a exporter'i (10.1.0.5), site-b (10.2.0.5) görünmez
	code, body = get("/api/v1/flows?minutes=60")
	if code != 200 || !strings.Contains(body, "10.1.0.5") || strings.Contains(body, "10.2.0.5") {
		t.Fatalf("flows site sızıntısı: %d %s", code, body)
	}
	// syslog: yalnizca a-event
	code, body = get("/api/v1/syslog")
	if code != 200 || !strings.Contains(body, "a-event") || strings.Contains(body, "b-event") {
		t.Fatalf("syslog site sızıntısı: %d %s", code, body)
	}
	// baska sitenin cihaz detayi → 404
	if code, _ := get("/api/v1/devices/" + strconv.FormatInt(idB, 10) + "/interfaces"); code != http.StatusNotFound {
		t.Fatalf("site-b cihaz detayi 404 olmaliydi, gelen: %d", code)
	}
	// kendi sitesinin cihaz detayi → 200
	if code, _ := get("/api/v1/devices/" + strconv.FormatInt(idA, 10) + "/interfaces"); code != 200 {
		t.Fatalf("site-a cihaz detayi 200 olmaliydi, gelen: %d", code)
	}
}

// --- S11.8 (B3): L7 / DNS / Processes / Geo / Report site scope ---

func newSiteScopeServer(t *testing.T, password string) (*httptest.Server, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ss.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, password, "", 30, false, nil, nil, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st
}

func getArray(t *testing.T, ts *httptest.Server, path, token string) (int, []map[string]any) {
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
	var out []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func reportStatus(t *testing.T, ts *httptest.Server, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/report", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("report isteği: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestSiteScopeLeakL7DNSProcessesReport(t *testing.T) {
	ts, st := newSiteScopeServer(t, "admin-pass-ss")
	_, adminTok := login(t, ts, "admin-pass-ss")

	// iki site'lı fixture
	now := time.Now().Unix()
	idA, err := st.RegisterAgent(store.Agent{Name: "a-dc1", Site: "dc1", TokenHash: store.TokenHash("ta"), ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register A: %v", err)
	}
	idB, err := st.RegisterAgent(store.Agent{Name: "b-dc2", Site: "dc2", TokenHash: store.TokenHash("tb"), ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register B: %v", err)
	}
	for id, host := range map[int64]string{idA: "dc1.example", idB: "dc2.example"} {
		if err := st.SaveL7(id, now, []telemetry.L7Sample{{PID: 1, Process: "curl", Kind: "tls", Host: host, RemoteIP: "9.9.9.9", Bytes: 100, Count: 2}}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveAgentDNS(id, now, []telemetry.DNSSample{{PID: 1, Process: "curl", Domain: host, Queries: 2, Responses: 2}}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{{PID: 1, Process: "curl-" + host, Proto: "tcp", RemoteIP: "9.9.9.9", Port: 443, BytesIn: 900, BytesOut: 100}}); err != nil {
			t.Fatal(err)
		}
	}

	// dc1'e kısıtlı viewer
	if code, out := postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{
		"username": "sam", "password": "sam-pass-1", "role": "viewer", "site": "dc1",
	}); code != http.StatusOK {
		t.Fatalf("kullanıcı: %d %v", code, out)
	}
	samTok := userToken(t, ts, "sam", "sam-pass-1")

	// admin (site'sız): L7/DNS/processes = 2 kayıt
	for _, p := range []string{"/api/v1/l7", "/api/v1/dns", "/api/v1/processes"} {
		if code, arr := getArray(t, ts, p, adminTok); code != 200 || len(arr) != 2 {
			t.Fatalf("admin %s: code=%d len=%d", p, code, len(arr))
		}
	}

	// dc1-viewer: yalnız kendi site'ı (1 kayıt), dc2 host'u sızmıyor
	for _, tc := range []struct{ path, field, want string }{
		{"/api/v1/l7", "host", "dc1.example"},
		{"/api/v1/dns", "domain", "dc1.example"},
		{"/api/v1/processes", "process", "curl-dc1.example"},
	} {
		code, arr := getArray(t, ts, tc.path, samTok)
		if code != 200 || len(arr) != 1 {
			t.Fatalf("dc1-viewer %s: code=%d len=%d (%v)", tc.path, code, len(arr), arr)
		}
		if arr[0][tc.field] != tc.want {
			t.Fatalf("dc1-viewer %s sızıntı: %v (beklenen %s=%s)", tc.path, arr[0], tc.field, tc.want)
		}
	}

	// dc1-viewer, dc2 agent_id'sini açıkça isterse → 404; kendi agent'ı → 200
	if code, _ := getArray(t, ts, "/api/v1/l7?agent_id="+strconv.FormatInt(idB, 10), samTok); code != http.StatusNotFound {
		t.Fatalf("dc1-viewer agent_id=B: %d beklendi 404", code)
	}
	if code, arr := getArray(t, ts, "/api/v1/l7?agent_id="+strconv.FormatInt(idA, 10), samTok); code != 200 || len(arr) != 1 {
		t.Fatalf("dc1-viewer agent_id=A: code=%d len=%d", code, len(arr))
	}

	// /api/report: site-kısıtlı → 403, admin → 200
	if code := reportStatus(t, ts, samTok); code != http.StatusForbidden {
		t.Fatalf("dc1-viewer /api/report: %d beklendi 403", code)
	}
	if code := reportStatus(t, ts, adminTok); code != http.StatusOK {
		t.Fatalf("admin /api/report: %d beklendi 200", code)
	}
}
