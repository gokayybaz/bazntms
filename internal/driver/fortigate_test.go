package driver

// Faz 8.7: FortiDriver uçtan uca testi — mock REST API + gerçek vault
// (token şifreleme round-trip) ile tam Snapshot üretimi.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

const mockFortiResponses = `
{"http_method":"GET","results":{"version":"v7.4.4","build":2698,"serial":"FGTEST1","hostname":"fgt-x","uptime":7200},"status":"success"}
`

func TestFortiDriverPoll(t *testing.T) {
	// mock API: tüm uçları cevaplar
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/monitor/system/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mockFortiResponses))
	})
	mux.HandleFunc("/api/v2/monitor/system/resource/usage", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":[{"time":` + nowStr() + `,"cpu":23.5,"mem":61.2,"disk":12.0,"session":1500}],"status":"success"}`))
	})
	mux.HandleFunc("/api/v2/monitor/system/interface", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":[
			{"id":1,"name":"wan1","alias":"uplink","ip":"10.0.0.2","status":"up","link":{"speed":"1000FDX"},
			 "rx_bytes":9000,"tx_bytes":4000,"rx_errors":1,"tx_errors":0,"rx_drops":0,"tx_drops":2}
		],"status":"success"}`))
	})
	mux.HandleFunc("/api/v2/monitor/vpn/ipsec", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":[{"name":"hq","status":"up","rx_bytes":111,"tx_bytes":222,"peer":"1.1.1.1"},{"name":"branch","status":"down"}],"status":"success"}`))
	})
	mux.HandleFunc("/api/v2/monitor/vpn/ssl", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":{"users":[{"user":"veli","remote_host":"8.8.8.8","uptime":60,"rx":5,"tx":6}]},"status":"success"}`))
	})
	mux.HandleFunc("/api/v2/monitor/virtual-wan/health-check", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":{"sla1":{"members":[{"name":"wan1","state":"up","latency":14.2,"jitter":1.5,"packet_loss":0}]}},"status":"success"}`))
	})
	policyReqs := 0
	mux.HandleFunc("/api/v2/cmdb/firewall/policy", func(w http.ResponseWriter, r *http.Request) {
		policyReqs++
		if r.URL.Query().Get("start") == "0" {
			w.Write([]byte(`{"http_method":"GET","results":[{"policyid":7,"name":"web-out","action":"accept","hit_count":100,"bytes":5000}],"status":"success"}`))
			return
		}
		w.Write([]byte(`{"http_method":"GET","results":[],"status":"success"}`))
	})
	mux.HandleFunc("/api/v2/cmdb/system/vdom", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"http_method":"GET","results":[{"name":"root"}],"status":"success"}`))
	})

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	// gerçek vault: token şifrele → driver decrypt edebilmeli
	v, err := vault.Open(filepath.Join(t.TempDir(), "vault.key"))
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	encToken, err := v.Encrypt("super-secret-token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	d := store.Device{
		ID: 3, Name: "fgt-x", Host: "10.9.9.9", Vendor: "fortigate",
		APIURL: srv.URL, APIToken: encToken, APIVerifyTLS: false, VDOM: "root",
	}

	var f FortiDriver
	snap, err := f.Poll(context.Background(), d, v)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	if snap.SysName != "fgt-x" || snap.SysDescr == "" {
		t.Fatalf("sys: %+v", snap)
	}
	if len(snap.Ifaces) != 1 || snap.Ifaces[0].Name != "wan1" ||
		snap.Ifaces[0].Speed != 1_000_000_000 || snap.Ifaces[0].OperStatus != 1 {
		t.Fatalf("ifaces: %+v", snap.Ifaces)
	}
	if len(snap.Resources) != 1 || snap.Resources[0].Sessions != 1500 || snap.Resources[0].CPUPct != 23.5 {
		t.Fatalf("resources: %+v", snap.Resources)
	}
	if len(snap.VPN) != 3 { // 2 ipsec + 1 ssl
		t.Fatalf("vpn: %+v", snap.VPN)
	}
	// vpn normalize: up/down + ssl user
	var hq, branch, ssl *store.FortiVPNStatus
	for i := range snap.VPN {
		switch {
		case snap.VPN[i].Kind == "ipsec" && snap.VPN[i].Name == "hq":
			hq = &snap.VPN[i]
		case snap.VPN[i].Kind == "ipsec" && snap.VPN[i].Name == "branch":
			branch = &snap.VPN[i]
		case snap.VPN[i].Kind == "ssl":
			ssl = &snap.VPN[i]
		}
	}
	if hq == nil || hq.Status != "up" || hq.RxBytes != 111 {
		t.Fatalf("hq tüneli: %+v", hq)
	}
	if branch == nil || branch.Status != "down" {
		t.Fatalf("branch tüneli: %+v", branch)
	}
	if ssl == nil || ssl.Name != "veli" || ssl.Status != "up" {
		t.Fatalf("ssl: %+v", ssl)
	}
	if len(snap.SDWAN) != 1 || snap.SDWAN[0].LatencyMs != 14.2 || snap.SDWAN[0].HealthCheck != "sla1" {
		t.Fatalf("sdwan: %+v", snap.SDWAN)
	}
	if len(snap.Policies) != 1 || snap.Policies[0].Hits != 100 || snap.Policies[0].Action != "accept" {
		t.Fatalf("policies: %+v", snap.Policies)
	}

	// driver registry: fortigate → FortiDriver, snmp → SNMPDriver
	if _, ok := For(d).(*FortiDriver); !ok {
		t.Fatal("registry fortigate driver döndürmedi")
	}
	if _, ok := For(store.Device{Vendor: "snmp"}).(*SNMPDriver); !ok {
		t.Fatal("registry snmp driver döndürmedi")
	}
}

func TestFortiDriverMissingToken(t *testing.T) {
	v, err := vault.Open(filepath.Join(t.TempDir(), "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	var f FortiDriver
	_, err = f.Poll(context.Background(), store.Device{Vendor: "fortigate"}, v)
	if err == nil {
		t.Fatal("token'sız poll hata vermeli")
	}
}

func nowStr() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
