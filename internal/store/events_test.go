package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestQueryEvents(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	aid, _ := st.RegisterAgent(Agent{Name: "ev-agent", TokenHash: TokenHash("ev")})
	other, _ := st.RegisterAgent(Agent{Name: "ev-other", TokenHash: TokenHash("evo")})

	if err := st.SaveAgentDNS(aid, now-10, []telemetry.DNSSample{{PID: 5, Process: "curl", Domain: "example.com", Queries: 3, Responses: 3}}); err != nil {
		t.Fatalf("dns: %v", err)
	}
	if err := st.SaveL7(aid, now-8, []telemetry.L7Sample{
		{PID: 5, Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.2.3.4", Bytes: 100, Count: 2},
		{PID: 5, Process: "curl", Kind: "http", Host: "plain.example", RemoteIP: "1.2.3.4", Bytes: 50, Count: 1},
	}); err != nil {
		t.Fatalf("l7: %v", err)
	}
	if err := st.SaveAgentDNS(other, now-5, []telemetry.DNSSample{{Process: "x", Domain: "other.example", Queries: 1, Responses: 1}}); err != nil {
		t.Fatalf("dns2: %v", err)
	}
	if err := st.SaveFlows([]FlowRow{{Ts: now - 6, Device: "fw1", Src: "10.0.0.1", Dst: "8.8.8.8", SrcPort: 5000, DstPort: 53, Proto: "udp", Packets: 2, Octets: 120}}); err != nil {
		t.Fatalf("flows: %v", err)
	}
	if err := st.SaveSyslogEvent(SyslogEvent{Ts: now - 4, Host: "fw1", SourceIP: "192.168.1.1", Severity: 3, Message: "link down"}); err != nil {
		t.Fatalf("syslog: %v", err)
	}

	since := time.Now().Add(-time.Hour)

	// --- tüm türler ---
	all, next, err := st.QueryEvents(EventFilter{Since: since, Limit: 100})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if next != 0 {
		t.Fatalf("limit dolmadan next boş olmalı: %d", next)
	}
	// 2 dns + 2 l7 + 1 flow + 1 syslog = 6
	if len(all) != 6 {
		t.Fatalf("6 olay beklenirdi: %d (%+v)", len(all), all)
	}
	// ts azalan
	for i := 1; i < len(all); i++ {
		if all[i].Ts > all[i-1].Ts {
			t.Fatalf("ts azalan değil: %+v", all)
		}
	}
	byType := map[string]Event{}
	for _, e := range all {
		byType[e.Type] = e
	}
	if e := byType["dns.query"]; e.Domain != "example.com" && e.Domain != "other.example" {
		t.Fatalf("dns.query alanı: %+v", e)
	}
	if e := byType["tls.sni_observed"]; e.Domain != "example.com" || e.DstIP != "1.2.3.4" || e.Process != "curl" {
		t.Fatalf("tls.sni_observed: %+v", e)
	}
	if e := byType["http.host_observed"]; e.Domain != "plain.example" {
		t.Fatalf("http.host_observed: %+v", e)
	}
	if e := byType["netflow.flow"]; e.SrcIP != "10.0.0.1" || e.DstPort != 53 || e.Proto != "udp" || e.Count != 2 {
		t.Fatalf("netflow.flow: %+v", e)
	}
	if e := byType["syslog.received"]; e.Device != "fw1" || e.Severity != "3" {
		t.Fatalf("syslog.received: %+v", e)
	}

	// --- tür filtresi ---
	dns, _, _ := st.QueryEvents(EventFilter{Types: []string{"dns.query"}, Since: since, Limit: 100})
	if len(dns) != 2 {
		t.Fatalf("dns filtresi 2 beklenirdi: %d", len(dns))
	}

	// --- agent filtresi: device-kaynaklı türler düşer ---
	byAgent, _, _ := st.QueryEvents(EventFilter{AgentID: aid, Since: since, Limit: 100})
	for _, e := range byAgent {
		if e.AgentID != aid {
			t.Fatalf("agent filtresi sızdırdı: %+v", e)
		}
		if e.Type == "netflow.flow" || e.Type == "syslog.received" {
			t.Fatalf("agent filtresinde device-kaynaklı olay olmamalı: %+v", e)
		}
	}
	if len(byAgent) != 3 { // 1 dns + 2 l7
		t.Fatalf("aid için 3 olay: %d", len(byAgent))
	}

	// --- imleç (pagination) ---
	page1, next1, _ := st.QueryEvents(EventFilter{Since: since, Limit: 3})
	if len(page1) != 3 || next1 == 0 {
		t.Fatalf("sayfa 1: len=%d next=%d", len(page1), next1)
	}
	page2, _, _ := st.QueryEvents(EventFilter{Since: since, Before: next1, Limit: 100})
	if len(page2) == 0 || page2[0].Ts >= next1 {
		t.Fatalf("sayfa 2 imleçten eski olmalı: %+v", page2)
	}
}
