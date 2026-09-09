package fortigate

// Faz 8.7: FortiGate istemci testleri — httptest mock'u ile gerçekçi
// fixture'lar. results dizi/map varyantları, sayfalama, auth ve retry
// davranışları doğrulanır.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// newMockServer, fortigate REST API mock'u.
func newMockServer(t *testing.T, handler http.HandlerFunc) (*Client, string) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	c := New(Options{
		BaseURL:     srv.URL,
		Token:       "test-token",
		VerifyTLS:   false, // self-signed mock
		MinInterval: 0,
	})
	return c, srv.URL
}

func okEnvelope(results string) string {
	return `{"http_method":"GET","results":` + results + `,"vdom":"root","status":"success","version":"v7.4.4","build":2698}`
}

func TestSystemStatus(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/monitor/system/status" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(okEnvelope(`{
			"version":"v7.4.4","build":2698,"serial":"FGT60FTK2xxxxxxx",
			"hostname":"fgt-ofis","model_name":"FortiGate 60F","uptime":86400
		}`)))
	})

	st, err := c.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Hostname != "fgt-ofis" || st.Serial != "FGT60FTK2xxxxxxx" || st.Uptime != 86400 {
		t.Fatalf("status parse: %+v", st)
	}
}

func TestInterfacesAndSpeed(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okEnvelope(`[
			{"id":1,"name":"port1","alias":"WAN","ip":"10.0.0.2","mask":"255.255.255.0","status":"up",
			 "link":{"speed":"1000FDX"},"rx_bytes":1000000,"tx_bytes":500000,
			 "rx_packets":1000,"tx_packets":800,"rx_errors":2,"tx_errors":0,"rx_drops":1,"tx_drops":3},
			{"id":3,"name":"vlan100","status":"down"}
		]`)))
	})

	ifaces, err := c.Interfaces(context.Background(), "root")
	if err != nil || len(ifaces) != 2 {
		t.Fatalf("interfaces: %v %d", err, len(ifaces))
	}
	if ifaces[0].SpeedBps() != 1_000_000_000 {
		t.Fatalf("speed parse: %d", ifaces[0].SpeedBps())
	}
	if ifaces[1].SpeedBps() != 0 || ifaces[1].Status != "down" {
		t.Fatalf("ikinci arayüz: %+v", ifaces[1])
	}
}

func TestIPsecTunnelsArrayAndMap(t *testing.T) {
	// dizi biçimi
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okEnvelope(`[{"name":"branch-1","status":"up","rx_bytes":99,"tx_bytes":44,"peer":"1.2.3.4"}]`)))
	})
	tunnels, err := c.IPsecTunnels(context.Background(), "root")
	if err != nil || len(tunnels) != 1 || tunnels[0].Name != "branch-1" || tunnels[0].Status != "up" {
		t.Fatalf("ipsec dizi: %v %+v", err, tunnels)
	}

	// map biçimi (bazı FortiOS sürümleri)
	c2, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okEnvelope(`{"hq-tunnel":{"status":"down","rx_bytes":10}}`)))
	})
	tunnels2, err := c2.IPsecTunnels(context.Background(), "")
	if err != nil || len(tunnels2) != 1 || tunnels2[0].Name != "hq-tunnel" || tunnels2[0].Status != "down" {
		t.Fatalf("ipsec map: %v %+v", err, tunnels2)
	}
}

func TestSSLVPNWrappedShape(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okEnvelope(`{"users":[{"user":"ayse.k","remote_host":"88.99.1.2","uptime":3600,"rx":500,"tx":300}]}`)))
	})
	sessions, err := c.SSLVPNSessions(context.Background(), "root")
	if err != nil || len(sessions) != 1 || sessions[0].User != "ayse.k" {
		t.Fatalf("ssl: %v %+v", err, sessions)
	}
}

func TestSDWANHealthMapShape(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okEnvelope(`{"hc1":{"members":[
			{"name":"wan1","state":"up","latency":12.5,"jitter":2.1,"packet_loss":0},
			{"name":"wan2","state":"down","latency":0,"jitter":0,"packet_loss":100}
		]}}`)))
	})
	health, err := c.SDWANHealth(context.Background(), "root")
	if err != nil || len(health["hc1"]) != 2 {
		t.Fatalf("sdwan: %v %+v", err, health)
	}
	if health["hc1"][0].LatencyMs != 12.5 || health["hc1"][1].State != "down" {
		t.Fatalf("sdwan member: %+v", health)
	}
}

func TestPoliciesPagination(t *testing.T) {
	requests := 0
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		requests++
		switch start {
		case "0":
			// tam sayfa: 500 kayıt → sayfalama devam eder
			var sb strings.Builder
			sb.WriteString(`[`)
			for i := 0; i < 500; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`{"policyid":` + strconv.Itoa(i+1) + `,"name":"pol","action":"accept","hit_count":10,"bytes":100}`)
			}
			sb.WriteString(`]`)
			w.Write([]byte(okEnvelope(sb.String())))
		default:
			// kısa sayfa → dur
			w.Write([]byte(okEnvelope(`[{"policyid":501,"name":"son","action":"deny","hit_count":5,"bytes":50}]`)))
		}
	})

	policies, err := c.Policies(context.Background(), "root")
	if err != nil {
		t.Fatalf("policies: %v", err)
	}
	if len(policies) != 501 || policies[500].Name != "son" {
		t.Fatalf("sayfalama: %d kayıt", len(policies))
	}
	if requests < 2 {
		t.Fatalf("tek istekte bitti: %d", requests)
	}
}

// --- Faz 27: gerçek FortiOS 7.2 şekilleri + sürüm tespiti ---

func env72(results string) string {
	return `{"http_method":"GET","results":` + results + `,"vdom":"root","status":"success","version":"v7.2.11","build":1639}`
}

func TestInterfaces72MapKeyed(t *testing.T) {
	// 7.2: results ada-göre anahtarlı obje; link bool; speed top-level Mbps
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env72(`{
			"port1":{"id":"port1","name":"port1","alias":"wan","link":true,"speed":1000.0,
			         "ip":"10.0.0.2 255.255.255.0","rx_bytes":9000,"tx_bytes":4000,"rx_errors":1},
			"port2":{"id":"port2","name":"port2","link":false,"speed":0}
		}`)))
	})
	ifaces, err := c.Interfaces(context.Background(), "root")
	if err != nil || len(ifaces) != 2 {
		t.Fatalf("interfaces: %v %d", err, len(ifaces))
	}
	byName := map[string]Interface{}
	for _, i := range ifaces {
		byName[i.Name] = i
	}
	p1 := byName["port1"]
	if p1.Status != "up" || p1.SpeedBps() != 1_000_000_000 || p1.IP != "10.0.0.2" || p1.RxBytes != 9000 {
		t.Fatalf("port1: %+v", p1)
	}
	if byName["port2"].Status != "down" {
		t.Fatalf("port2 down bekleniyordu: %+v", byName["port2"])
	}
	if c.Version() != "v7.2.11" || c.ProfileID() != "7.2" {
		t.Fatalf("sürüm auto-tespit: version=%q profile=%q", c.Version(), c.ProfileID())
	}
}

func TestResourceUsageMapOfArrays(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("interval") != "" {
			t.Errorf("interval parametresi gönderilmemeli: %s", r.URL.RawQuery)
		}
		w.Write([]byte(env72(`{"cpu":[{"current":23}],"mem":[{"current":61}],"disk":[{"current":12}],"session":[{"current":1500}]}`)))
	})
	s, err := c.ResourceUsage(context.Background(), "root")
	if err != nil || len(s) != 1 {
		t.Fatalf("resource: %v %+v", err, s)
	}
	if s[0].CPU != 23 || s[0].Mem != 61 || s[0].Session != 1500 {
		t.Fatalf("resource örnek: %+v", s[0])
	}
}

func TestIPsecProxyIDShape(t *testing.T) {
	// 7.2: peer=rgwy; tünel status yok → proxyid[].status; baytlar proxyid altında
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env72(`[{"name":"hq","rgwy":"203.0.113.9","proxyid":[
			{"status":"up","incoming_bytes":500,"outgoing_bytes":300}]}]`)))
	})
	tuns, err := c.IPsecTunnels(context.Background(), "root")
	if err != nil || len(tuns) != 1 {
		t.Fatalf("ipsec: %v %+v", err, tuns)
	}
	if tuns[0].Peer != "203.0.113.9" || tuns[0].Status != "up" || tuns[0].RxBytes != 500 || tuns[0].TxBytes != 300 {
		t.Fatalf("ipsec tünel: %+v", tuns[0])
	}
}

func TestSSLVPN72UserName(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env72(`[{"user_name":"mehmet","remote_host":"88.1.2.3","uptime":120}]`)))
	})
	s, err := c.SSLVPNSessions(context.Background(), "root")
	if err != nil || len(s) != 1 || s[0].User != "mehmet" || s[0].RemoteHost != "88.1.2.3" {
		t.Fatalf("ssl: %v %+v", err, s)
	}
}

func TestSDWANIfnameKeyed(t *testing.T) {
	// 7.2: {hc:{ifname:{...}}} — members dizisi yok
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env72(`{"SLA":{"port1":{"status":"up","latency":8.1,"jitter":0.7,"packet_loss":0},
			"port2":{"status":"down","latency":0,"jitter":0,"packet_loss":100}}}`)))
	})
	h, err := c.SDWANHealth(context.Background(), "root")
	if err != nil || len(h["SLA"]) != 2 {
		t.Fatalf("sdwan: %v %+v", err, h)
	}
	byIf := map[string]SDWANMember{}
	for _, m := range h["SLA"] {
		byIf[m.Member] = m
	}
	if byIf["port1"].LatencyMs != 8.1 || byIf["port2"].State != "down" {
		t.Fatalf("sdwan üye: %+v", h["SLA"])
	}
}

func TestPoliciesMonitorCmdbJoin(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/cmdb/firewall/policy":
			w.Write([]byte(env72(`[{"policyid":1,"name":"lan-wan","action":"accept"},{"policyid":2,"name":"deny-all","action":"deny"}]`)))
		case r.URL.Path == "/api/v2/monitor/firewall/policy":
			w.Write([]byte(env72(`[{"policyid":1,"bytes":123456,"hit_count":42},{"policyid":2,"bytes":0,"hit_count":0}]`)))
		default:
			http.NotFound(w, r)
		}
	})
	pols, err := c.Policies(context.Background(), "root")
	if err != nil || len(pols) != 2 {
		t.Fatalf("policies: %v %+v", err, pols)
	}
	if pols[0].Name != "lan-wan" || pols[0].Hits != 42 || pols[0].Bytes != 123456 {
		t.Fatalf("policy join: %+v", pols[0])
	}
}

func TestProfilePinnedOverridesVersion(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(env72(`{"hostname":"x"}`)))
	})
	c.pinnedProfile = true
	c.profile, _ = ProfileByID("7.4")
	if _, err := c.SystemStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.ProfileID() != "7.4" {
		t.Fatalf("pinlenmiş profil sürümle ezildi: %s", c.ProfileID())
	}
}

func TestAuthFailure(t *testing.T) {
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.SystemStatus(context.Background()); err == nil {
		t.Fatal("401 hatasız geçti")
	}
}

func TestRetryOn500(t *testing.T) {
	attempts := 0
	c, _ := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(okEnvelope(`{"hostname":"ok"}`)))
	})
	c.opts.MaxRetries = 4
	st, err := c.SystemStatus(context.Background())
	if err != nil || st.Hostname != "ok" {
		t.Fatalf("retry: %v %+v (deneme: %d)", err, st, attempts)
	}
}
