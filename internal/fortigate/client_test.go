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
