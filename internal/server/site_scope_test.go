package server

import (
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
)

// TestSiteScopeIsolation, site-sinirli bir kimligin (API token, site="ofis-a")
// yalnizca kendi sitesine ait cihazlari, NetFlow'u ve syslog'u gordugunu;
// baska sitenin cihaz detayina 404 aldigini dogrular.
func TestSiteScopeIsolation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
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
	st.SaveSyslogEvent(store.SyslogEvent{Ts: now.Unix(), Host: "rtr-a", SourceIP: "10.1.0.1", Severity: 5, Message: "a-event"})
	st.SaveSyslogEvent(store.SyslogEvent{Ts: now.Unix(), Host: "rtr-b", SourceIP: "10.2.0.1", Severity: 5, Message: "b-event"})

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
