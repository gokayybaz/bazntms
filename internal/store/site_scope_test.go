package store

// S11.7 (B3): TopL7 / TopAgentDNS / TopProcessTraffic / FleetTopEndpoints
// site parametresi — site-kısıtlı sorgu yalnızca o site'taki agent'ların
// verisini görmeli; site="" tüm filoyu görmeli.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestTopQueriesSiteScope(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Now().Unix()

	idA, err := st.RegisterAgent(Agent{Name: "a-dc1", Site: "dc1", TokenHash: TokenHash("ta"), ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register A: %v", err)
	}
	idB, err := st.RegisterAgent(Agent{Name: "b-dc2", Site: "dc2", TokenHash: TokenHash("tb"), ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register B: %v", err)
	}

	for _, id := range []int64{idA, idB} {
		suffix := "-dc1"
		if id == idB {
			suffix = "-dc2"
		}
		if err := st.SaveL7(id, now, []telemetry.L7Sample{
			{PID: 1, Process: "curl", Kind: "tls", Host: "site" + suffix + ".example", RemoteIP: "9.9.9.9", Bytes: 100, Count: 2},
		}); err != nil {
			t.Fatalf("SaveL7 %d: %v", id, err)
		}
		if err := st.SaveAgentDNS(id, now, []telemetry.DNSSample{
			{PID: 1, Process: "curl", Domain: "site" + suffix + ".example", Queries: 2, Responses: 2},
		}); err != nil {
			t.Fatalf("SaveAgentDNS %d: %v", id, err)
		}
		if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
			{PID: 1, Process: "curl" + suffix, Proto: "tcp", RemoteIP: "9.9.9.9", Port: 443, BytesIn: 900, BytesOut: 100},
		}); err != nil {
			t.Fatalf("SaveProcessTraffic %d: %v", id, err)
		}
	}

	since := time.Unix(now-60, 0)

	// site="" → iki site de görünür
	if l7, _ := st.TopL7(since, 0, 50, ""); len(l7) != 2 {
		t.Fatalf("site'sız TopL7: 2 bekleniyor, %d", len(l7))
	}
	if dns, _ := st.TopAgentDNS(since, 0, 50, ""); len(dns) != 2 {
		t.Fatalf("site'sız TopAgentDNS: 2 bekleniyor, %d", len(dns))
	}
	if pt, _ := st.TopProcessTraffic(since, 0, 50, ""); len(pt) != 2 {
		t.Fatalf("site'sız TopProcessTraffic: 2 bekleniyor, %d", len(pt))
	}

	// site="dc1" → yalnız dc1 verisi
	l7, _ := st.TopL7(since, 0, 50, "dc1")
	if len(l7) != 1 || l7[0].Host != "site-dc1.example" {
		t.Fatalf("dc1 TopL7 sızıntı: %+v", l7)
	}
	dns, _ := st.TopAgentDNS(since, 0, 50, "dc1")
	if len(dns) != 1 || dns[0].Domain != "site-dc1.example" {
		t.Fatalf("dc1 TopAgentDNS sızıntı: %+v", dns)
	}
	pt, _ := st.TopProcessTraffic(since, 0, 50, "dc1")
	if len(pt) != 1 || pt[0].Process != "curl-dc1" {
		t.Fatalf("dc1 TopProcessTraffic sızıntı: %+v", pt)
	}

	// agent_id verilse bile başka site'ın agent'ı → boş
	if l7x, _ := st.TopL7(since, idB, 50, "dc1"); len(l7x) != 0 {
		t.Fatalf("dc1 kimliği dc2 agent_id'sini görmemeli: %+v", l7x)
	}

	// FleetTopEndpoints process_traffic fallback'i (flows yok) site'a göre
	if eps, _ := st.FleetTopEndpoints(since, 50, ""); len(eps) != 1 || eps[0].IP != "9.9.9.9" {
		t.Fatalf("site'sız FleetTopEndpoints: %+v", eps)
	}
	epsA, _ := st.FleetTopEndpoints(since, 50, "dc1")
	if len(epsA) != 1 || epsA[0].IP != "9.9.9.9" || epsA[0].BytesIn != 900 {
		t.Fatalf("dc1 FleetTopEndpoints: %+v", epsA)
	}
	if epsNone, _ := st.FleetTopEndpoints(since, 50, "dc3"); len(epsNone) != 0 {
		t.Fatalf("dc3 (veri yok) boş dönmeli: %+v", epsNone)
	}
}

func TestFleetTopEndpointsFlowsSiteScope(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "f.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Now().Unix()

	if _, err := st.AddDevice(Device{Name: "rt-dc1", Host: "10.0.1.1", Site: "dc1", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60}); err != nil {
		t.Fatalf("device dc1: %v", err)
	}
	if _, err := st.AddDevice(Device{Name: "rt-dc2", Host: "10.0.2.1", Site: "dc2", Kind: "router", SNMPVersion: 2, Enabled: true, PollSeconds: 60}); err != nil {
		t.Fatalf("device dc2: %v", err)
	}
	if err := st.SaveFlows([]FlowRow{
		{Ts: now, Device: "10.0.1.1", Src: "1.1.1.1", Dst: "8.8.8.8", Proto: "udp", Packets: 5, Octets: 5000},
		{Ts: now, Device: "10.0.2.1", Src: "2.2.2.2", Dst: "9.9.9.9", Proto: "udp", Packets: 5, Octets: 7000},
	}); err != nil {
		t.Fatalf("SaveFlows: %v", err)
	}

	since := time.Unix(now-60, 0)
	all, _ := st.FleetTopEndpoints(since, 50, "")
	if len(all) != 4 { // 2 flow × (src+dst)
		t.Fatalf("site'sız: 4 uç bekleniyor, %d (%+v)", len(all), all)
	}
	dc1, _ := st.FleetTopEndpoints(since, 50, "dc1")
	for _, e := range dc1 {
		if e.IP == "9.9.9.9" || e.IP == "2.2.2.2" {
			t.Fatalf("dc1 sorgusu dc2 flow ucunu gördü: %+v", dc1)
		}
	}
	if len(dc1) != 2 {
		t.Fatalf("dc1: 2 uç bekleniyor (1.1.1.1, 8.8.8.8), %+v", dc1)
	}
}
