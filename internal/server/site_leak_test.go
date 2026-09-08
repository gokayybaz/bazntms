package server

// Faz 14B S14.B4: tek parametrize çapraz-saha sızıntı testi.
//
// İki saha (dc1, dc2) her tabloda veri taşır. Bir site-admin@dc1 kimliğiyle
// TÜM liste/detay uçları taranır; hiçbirinden dc2 verisi görünmemeli
// (agent_id/device_id/user_id tahmini dâhil). Saha-üstü uçlar (ISMS, uyarı
// config, audit-verify) site-admin'e 403.
//
// Yeni bir liste ucu eklerken bu tabloya bir satır ekle.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// seedSite, bir sahaya agent + cihaz + filo verisi ekler; agent ve cihaz
// id'lerini döndürür (çapraz-saha IDOR denemeleri için).
func seedSite(t *testing.T, st store.Store, site string) (agentID, deviceID int64) {
	t.Helper()
	now := time.Now().Unix()
	agentID, err := st.RegisterAgent(store.Agent{Name: site + "-agent", Site: site, TokenHash: store.TokenHash(site + "-atok")})
	if err != nil {
		t.Fatalf("agent %s: %v", site, err)
	}
	if err := st.TouchAgent(agentID, "1.0.0", 1, "10.0.0.1"); err != nil {
		t.Fatalf("touch %s: %v", site, err)
	}
	_ = st.SaveIfaceSamples(agentID, now-30, []telemetry.InterfaceSample{{Name: "eth0", RxBytes: 1000, TxBytes: 500}})
	_ = st.SaveIfaceSamples(agentID, now, []telemetry.InterfaceSample{{Name: "eth0", RxBytes: 5000, TxBytes: 2500}})
	_ = st.SaveL7(agentID, now, []telemetry.L7Sample{{Process: "curl", Kind: "tls", Host: site + ".example.com", Bytes: 100, Count: 1}})
	_ = st.SaveAgentDNS(agentID, now, []telemetry.DNSSample{{Process: "curl", Domain: site + ".example.net", Queries: 2, Responses: 2}})
	_ = st.SaveProcessTraffic(agentID, now, []telemetry.ProcessTrafficSample{{PID: 9, Process: site + "-proc", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 10, BytesOut: 5}})
	_ = st.ReplaceConnLatest(agentID, []telemetry.ConnectionSample{{Proto: "tcp", LocalAddr: "10.0.0.1:22", RemoteAddr: "2.2.2.2:5000", Status: "ESTABLISHED", Process: site + "-sshd"}})
	_ = st.SaveAgentSubnets(agentID, site+"-agent", []string{"10." + site[len(site)-1:] + ".0.0/24"})
	_, _ = st.CreateIncident(store.Incident{
		Title: site + " şüpheli aktivite", Severity: "crit", Status: "open", Site: site,
		AgentID: agentID, CorrelationKey: "r1|" + site, CorrelationReason: site + "-neden",
		FirstSeen: now, LastSeen: now,
	})

	deviceID, err = st.AddDevice(store.Device{Name: site + "-fw", Host: "192.168." + site[len(site)-1:] + ".1", Kind: "firewall", Site: site, Vendor: "snmp"})
	if err != nil {
		t.Fatalf("device %s: %v", site, err)
	}
	_ = st.SaveFlows([]store.FlowRow{{Ts: now, Device: "192.168." + site[len(site)-1:] + ".1", Src: "10.0.0.5", Dst: "8.8.8.8", Proto: "udp", Octets: 100}})
	_ = st.SaveSyslogEvent(store.SyslogEvent{Ts: now, Host: site + "-fw", SourceIP: "192.168." + site[len(site)-1:] + ".1", Message: site + " syslog satırı"})
	return agentID, deviceID
}

func TestSiteLeak(t *testing.T) {
	ts, _, st := newRBACServerEx(t, "admin-pass-1", "static-boot", true)
	_, out := postJSON(t, ts, "/api/login", "", map[string]string{"password": "admin-pass-1"})
	adminTok, _ := out["token"].(string)

	a1, d1 := seedSite(t, st, "dc1")
	a2, d2 := seedSite(t, st, "dc2")
	_ = a1
	_ = d1

	// her sahaya bir kullanıcı + token + enroll token
	for _, s := range []string{"dc1", "dc2"} {
		postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{"username": "u-" + s, "password": "password-123", "role": "viewer", "site": s})
		postJSON(t, ts, "/api/v1/tokens", adminTok, map[string]any{"name": "tok-" + s, "role": "viewer", "site": s})
		postJSON(t, ts, "/api/v1/enroll-tokens", adminTok, map[string]any{"name": "ent-" + s, "site": s})
	}
	// dc1 + dc2 site-admin (audit olayları için)
	postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{"username": "sa-dc1", "password": "password-123", "role": "site-admin", "site": "dc1"})
	postJSON(t, ts, "/api/v1/users", adminTok, map[string]any{"username": "sa-dc2", "password": "password-123", "role": "site-admin", "site": "dc2"})
	saTok := userToken(t, ts, "sa-dc1", "password-123")
	// sa-dc2 bir işlem yapsın (kendi sahasında audit olayı üretir)
	sa2 := userToken(t, ts, "sa-dc2", "password-123")
	postJSON(t, ts, "/api/v1/enroll-tokens", sa2, map[string]any{"name": "ent-dc2-by-sa", "site": "dc2"})

	// --- saha-üstü uçlar: site-admin'e 403 ---
	for _, p := range []string{
		"/api/alerts", "/api/alerts/status", "/api/v1/audit/verify",
		"/api/v1/isms/summary", "/api/v1/isms/assets", "/api/v1/isms/risks",
		"/api/v1/compliance/evidence",
	} {
		if code, _ := getJSON(t, ts, p, saTok); code != http.StatusForbidden {
			t.Errorf("saha-üstü uç %s: site-admin 403 beklenirdi, %d", p, code)
		}
	}

	// --- çapraz-saha IDOR: dc2 kaynağı, site-admin@dc1 → 404 ---
	for _, p := range []string{
		"/api/v1/agents/" + i64(a2),
		"/api/v1/agents/" + i64(a2) + "/history",
		"/api/v1/agents/" + i64(a2) + "/processes/dc2-proc",
		"/api/v1/l7?agent_id=" + i64(a2),
		"/api/v1/dns?agent_id=" + i64(a2),
		"/api/v1/processes?agent_id=" + i64(a2),
		"/api/v1/devices/" + i64(d2) + "/interfaces",
		"/api/v1/devices/" + i64(d2) + "/vpn",
		"/api/v1/incidents?agent_id=" + i64(a2),
	} {
		if code, _ := getJSON(t, ts, p, saTok); code != http.StatusNotFound {
			t.Errorf("çapraz-saha IDOR %s: 404 beklenirdi, %d", p, code)
		}
	}
	// dc2 agent silme / adlandırma → 404
	if c := delReq(t, ts, "/api/v1/agents/"+i64(a2), saTok); c != http.StatusNotFound {
		t.Errorf("dc2 agent silme: 404 beklenirdi, %d", c)
	}

	// --- liste uçları: yanıtta dc2 verisinin AYIRT EDİCİ izi olmamalı ---
	// (bare "dc2" alt-dizesi rastgele hex hash'lerde de geçebilir; seedSite'in
	// koyduğu belirgin işaretçileri arıyoruz.)
	dc2Markers := []string{
		`"site":"dc2"`, "dc2-agent", "dc2-fw", "dc2.example.com", "dc2.example.net",
		"dc2-proc", "dc2-sshd", "dc2 syslog", "192.168.2.1", "10.2.0.0/24", "dc2-neden", "dc2 şüpheli",
	}
	bodyChecks := []string{
		"/api/v1/agents", "/api/v1/devices", "/api/v1/flows", "/api/v1/flows/conversations", "/api/v1/syslog",
		"/api/v1/topology", "/api/v1/geo", "/api/v1/l7", "/api/v1/dns", "/api/v1/processes", "/api/v1/events",
		"/api/v1/incidents",
		"/api/v1/users", "/api/v1/tokens", "/api/v1/enroll-tokens", "/api/v1/audit",
	}
	for _, p := range bodyChecks {
		body := rawGet(t, ts, p, saTok)
		for _, m := range dc2Markers {
			if strings.Contains(body, m) {
				t.Errorf("liste ucu %s site-admin@dc1 yanıtında dc2 izi (%q) geçiyor:\n%s", p, m, trunc(body))
			}
		}
		if p == "/api/v1/agents" && !strings.Contains(body, "dc1-agent") {
			t.Errorf("%s: site-admin kendi sahasının agent'ını görmeli", p)
		}
	}

	// --- rapor: trafik raporu site-kısıtlıya 403; kurumsal/uyumluluk açık (S22.22) ---
	if code, _ := getJSON(t, ts, "/api/report?days=1", saTok); code != http.StatusForbidden {
		t.Errorf("/api/report (trafik) site-admin: 403 beklenirdi, %d", code)
	}
	if code, _ := getJSON(t, ts, "/api/report?type=enterprise&days=1", saTok); code != http.StatusOK {
		t.Errorf("/api/report?type=enterprise site-admin: 200 beklenirdi (kendi sahasına kırpılı), %d", code)
	}
	if code, _ := getJSON(t, ts, "/api/report?type=compliance", saTok); code != http.StatusOK {
		t.Errorf("/api/report?type=compliance site-admin: 200 beklenirdi, %d", code)
	}
}

func i64(v int64) string { return strconv.FormatInt(v, 10) }

func rawGet(t *testing.T, ts *httptest.Server, path, token string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: 200 beklendi, %d", path, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func trunc(s string) string {
	if len(s) > 800 {
		return s[:800] + "…"
	}
	return s
}
