package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestProcessDetail(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	aid, err := st.RegisterAgent(Agent{Name: "det-agent", TokenHash: TokenHash("d")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	other, err := st.RegisterAgent(Agent{Name: "other", TokenHash: TokenHash("o")})
	if err != nil {
		t.Fatalf("register other: %v", err)
	}

	// iki farklı ts kovası (hız hesabı için) + iki uzak uç + iki PID.
	if err := st.SaveProcessTraffic(aid, now-30, []telemetry.ProcessTrafficSample{
		{PID: 10, Process: "curl", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 1000, BytesOut: 200},
		{PID: 10, Process: "curl", Proto: "tcp", RemoteIP: "8.8.8.8", Port: 53, BytesIn: 40, BytesOut: 60},
	}); err != nil {
		t.Fatalf("pt kova1: %v", err)
	}
	if err := st.SaveProcessTraffic(aid, now, []telemetry.ProcessTrafficSample{
		{PID: 11, Process: "curl", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 3000, BytesOut: 300},
	}); err != nil {
		t.Fatalf("pt kova2: %v", err)
	}
	// başka agent'ın aynı adlı süreci — kapsam dışı kalmalı.
	if err := st.SaveProcessTraffic(other, now, []telemetry.ProcessTrafficSample{
		{PID: 99, Process: "curl", Proto: "tcp", RemoteIP: "2.2.2.2", Port: 443, BytesIn: 9999, BytesOut: 9999},
	}); err != nil {
		t.Fatalf("pt other: %v", err)
	}

	if err := st.SaveAgentDNS(aid, now-30, []telemetry.DNSSample{
		{PID: 10, Process: "curl", Domain: "example.com", Queries: 2, Responses: 2},
	}); err != nil {
		t.Fatalf("dns: %v", err)
	}
	if err := st.SaveL7(aid, now, []telemetry.L7Sample{
		{PID: 11, Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.1.1.1", Bytes: 512, Count: 3},
	}); err != nil {
		t.Fatalf("l7: %v", err)
	}

	since := time.Now().Add(-time.Hour)

	// --- ProcessSummary ---
	sum, err := st.ProcessSummary(aid, "curl", since)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.BytesIn != 4040 || sum.BytesOut != 560 || sum.Total != 4600 {
		t.Fatalf("summary toplam hatalı: %+v", sum)
	}
	if len(sum.PIDs) != 2 || sum.PIDs[0] != 10 || sum.PIDs[1] != 11 {
		t.Fatalf("pids hatalı: %+v", sum.PIDs)
	}
	if sum.FirstSeen != now-30 || sum.LastSeen != now {
		t.Fatalf("first/last hatalı: %d/%d", sum.FirstSeen, sum.LastSeen)
	}
	// hız = son kova (3300 bayt) / 30 sn aralık
	if sum.RxBps < 99 || sum.RxBps > 101 {
		t.Fatalf("rx_bps ~100 beklenirdi: %v", sum.RxBps)
	}

	// bilinmeyen süreç → boş özet
	if empty, _ := st.ProcessSummary(aid, "yok", since); empty.FirstSeen != 0 || empty.Total != 0 {
		t.Fatalf("bilinmeyen süreç boş dönmeli: %+v", empty)
	}

	// --- ProcessRemotes ---
	rem, err := st.ProcessRemotes(aid, "curl", since, 10)
	if err != nil {
		t.Fatalf("remotes: %v", err)
	}
	if len(rem) != 2 {
		t.Fatalf("2 uzak uç beklenirdi: %+v", rem)
	}
	if rem[0].RemoteIP != "1.1.1.1" || rem[0].BytesIn != 4000 || rem[0].Port != 443 {
		t.Fatalf("ilk uzak uç hatalı: %+v", rem[0])
	}

	// --- ProcessAppVisibility ---
	app, err := st.ProcessAppVisibility(aid, "curl", since, 10)
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	if len(app) != 2 {
		t.Fatalf("2 gözlem beklenirdi (dns + tls): %+v", app)
	}
	var haveDNS, haveTLS bool
	for _, o := range app {
		if o.Type == "dns" && o.Host == "example.com" && o.Observations == 4 {
			haveDNS = true
		}
		if o.Type == "tls" && o.Host == "example.com" && o.Bytes == 512 {
			haveTLS = true
		}
	}
	if !haveDNS || !haveTLS {
		t.Fatalf("dns/tls gözlemi eksik: %+v", app)
	}

	// --- ProcessTimeline ---
	tl, err := st.ProcessTimeline(aid, "det-agent", "curl", since, 100)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if len(tl) < 3 {
		t.Fatalf("en az 3 olay (first_seen + dns + l7): %+v", tl)
	}
	if tl[0].Event != "process.first_seen" {
		t.Fatalf("ilk olay process.first_seen olmalı: %+v", tl[0])
	}
	for i := 1; i < len(tl); i++ {
		if tl[i].Ts < tl[i-1].Ts {
			t.Fatalf("timeline kronolojik değil: %+v", tl)
		}
	}
	var haveQuery bool
	for _, e := range tl {
		if e.Event == "dns.query" && e.Target == "example.com" {
			haveQuery = true
		}
	}
	if !haveQuery {
		t.Fatalf("dns.query olayı eksik: %+v", tl)
	}
}
