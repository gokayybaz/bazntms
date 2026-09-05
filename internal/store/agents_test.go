package store

import (
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestListAgentsRateCounterReset, arayuz sayaci geriledigi (arayuz/agent
// resetlendi) durumda ListAgents'in uint64 alt tasmasiyla dev bir sayi
// uretmedigini, oran yerine 0 dondurdugunu dogrular.
func TestListAgentsRateCounterReset(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "laptop-01", Site: "ofis", TokenHash: TokenHash("t")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := st.TouchAgent(id, "v1", 1, "10.0.0.5"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// onceki ornek: yuksek kumulatif sayac (ornegin uzun sure calismis wifi arayuzu)
	if err := st.SaveIfaceSamples(id, now-30, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 5_000_000, TxBytes: 2_000_000, RxPackets: 5000, TxPackets: 2000},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	// sonraki ornek: arayuz resetlendi (uyku/uyanma, surucu yeniden yuklendi)
	// — sayac kucuk bir degere dustu
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
	}); err != nil {
		t.Fatalf("ornek2: %v", err)
	}

	agents, err := st.ListAgents(time.Hour, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || len(agents[0].Rates) != 1 {
		t.Fatalf("beklenmedik filo: %+v", agents)
	}
	r := agents[0].Rates[0]
	// duzeltmeden once: RxBps = float64(uint64(1000)-uint64(5_000_000)) → devasa
	// (yaklasik 1.8e19) bir sayi olurdu. Duzeltmeden sonra: 0 olmali.
	if r.RxBps != 0 || r.TxBps != 0 || r.Pps != 0 {
		t.Fatalf("sayac gerilemesinde oran 0 olmali, uint64 alt tasmasi supheli: %+v", r)
	}
	// kumulatif degerler yine de son ornegi yansitmali (goruntu icin)
	if r.RxBytes != 1000 || r.TxBytes != 500 {
		t.Fatalf("kumulatif degerler son ornekten gelmeli: %+v", r)
	}
}

// TestListAgentsRateNormal, normal (sayac ilerleyen) durumda oranin dogru
// hesaplandigini dogrular — regresyon testinin "saglikli yol"u.
func TestListAgentsRateNormal(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "sunucu-01", Site: "dc1", TokenHash: TokenHash("t2")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := st.TouchAgent(id, "v1", 1, "10.0.0.6"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	if err := st.SaveIfaceSamples(id, now-10, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "eth0", RxBytes: 11000, TxBytes: 5500, RxPackets: 110, TxPackets: 55},
	}); err != nil {
		t.Fatalf("ornek2: %v", err)
	}

	agents, err := st.ListAgents(time.Hour, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 || len(agents[0].Rates) != 1 {
		t.Fatalf("beklenmedik filo: %+v", agents)
	}
	r := agents[0].Rates[0]
	// (11000-1000)/10sn = 1000 bayt/sn
	if r.RxBps != 1000 {
		t.Fatalf("rx_bps beklenen 1000, gelen %v", r.RxBps)
	}
	if r.TxBps != 500 {
		t.Fatalf("tx_bps beklenen 500, gelen %v", r.TxBps)
	}
	// (100+50)/10sn = 15 pps
	if r.Pps != 15 {
		t.Fatalf("pps beklenen 15, gelen %v", r.Pps)
	}
}

// TestTouchAgentVersionGuard, TouchAgent'in dolu surum/protokol degerini
// yazdigini, bos "" / 0 gelince mevcut degeri KORUDUGUNU dogrular (surum
// tasimayan eski agent hub'daki bilgiyi silmemeli).
func TestTouchAgentVersionGuard(t *testing.T) {
	st := openTest(t)
	id, err := st.RegisterAgent(Agent{Name: "a", TokenHash: TokenHash("t"), Version: "0.1.0", ProtocolVersion: 1})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := st.TouchAgent(id, "0.2.0", 1, "10.0.0.1"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if a, _ := st.AgentByTokenHash(TokenHash("t")); a.Version != "0.2.0" {
		t.Fatalf("dolu surum yazilmaliydi, gelen: %q", a.Version)
	}

	// bos surum + 0 protokol → degistirme
	if err := st.TouchAgent(id, "", 0, "10.0.0.2"); err != nil {
		t.Fatalf("touch2: %v", err)
	}
	a, _ := st.AgentByTokenHash(TokenHash("t"))
	if a.Version != "0.2.0" || a.ProtocolVersion != 1 {
		t.Fatalf("bos degerler mevcut surumu korumaliydi: %q pv=%d", a.Version, a.ProtocolVersion)
	}
	if a.RemoteIP != "10.0.0.2" {
		t.Fatalf("remote_ip yine de guncellenmeliydi, gelen: %q", a.RemoteIP)
	}
}

// TestDeleteAgentCascade, agent silinince ona bagli TUM zaman-serisi
// tablolarinin (iface ornekleri + surec trafigi + L7 + DNS) temizlendigini,
// diger agent'in verisine dokunulmadigini dogrular. Regresyon: eskiden
// DeleteAgent yalnizca agents/agent_iface_samples/agent_conn_latest siliyordu,
// process_traffic/l7_endpoints/agent_dns oksuz kaliyordu.
func TestDeleteAgentCascade(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	victim, err := st.RegisterAgent(Agent{Name: "kurban", Site: "ofis", TokenHash: TokenHash("v")})
	if err != nil {
		t.Fatalf("register kurban: %v", err)
	}
	keep, err := st.RegisterAgent(Agent{Name: "kalan", Site: "ofis", TokenHash: TokenHash("k")})
	if err != nil {
		t.Fatalf("register kalan: %v", err)
	}

	for _, id := range []int64{victim, keep} {
		if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: 1000, TxBytes: 500, RxPackets: 10, TxPackets: 5},
		}); err != nil {
			t.Fatalf("iface ornek (agent %d): %v", id, err)
		}
		if err := st.SaveProcessTraffic(id, now, []telemetry.ProcessTrafficSample{
			{PID: 42, Process: "curl", Proto: "tcp", RemoteIP: "1.1.1.1", Port: 443, BytesIn: 900, BytesOut: 100},
		}); err != nil {
			t.Fatalf("process_traffic (agent %d): %v", id, err)
		}
		if err := st.SaveL7(id, now, []telemetry.L7Sample{
			{PID: 42, Process: "curl", Kind: "tls", Host: "example.com", RemoteIP: "1.1.1.1", Bytes: 500, Count: 3},
		}); err != nil {
			t.Fatalf("l7 (agent %d): %v", id, err)
		}
		if err := st.SaveAgentDNS(id, now, []telemetry.DNSSample{
			{PID: 42, Process: "curl", Domain: "example.com", Queries: 2, Responses: 2},
		}); err != nil {
			t.Fatalf("agent_dns (agent %d): %v", id, err)
		}
	}

	if err := st.DeleteAgent(victim); err != nil {
		t.Fatalf("delete: %v", err)
	}

	since := time.Unix(now-60, 0)
	// kurban: her tabloda 0 satir
	if pt, _ := st.TopProcessTraffic(since, victim, 10); len(pt) != 0 {
		t.Errorf("silinen agent'in process_traffic satirlari kaldi: %+v", pt)
	}
	if l7, _ := st.TopL7(since, victim, 10); len(l7) != 0 {
		t.Errorf("silinen agent'in l7_endpoints satirlari kaldi: %+v", l7)
	}
	if dns, _ := st.TopAgentDNS(since, victim, 10); len(dns) != 0 {
		t.Errorf("silinen agent'in agent_dns satirlari kaldi: %+v", dns)
	}
	if ag, _ := st.ListAgents(time.Hour, ""); len(ag) != 1 || ag[0].ID != keep {
		t.Errorf("yalnizca 'kalan' agent durmali: %+v", ag)
	}
	// kalan agent: verisi bozulmadi
	if pt, _ := st.TopProcessTraffic(since, keep, 10); len(pt) != 1 {
		t.Errorf("kalan agent'in process_traffic'i silinmis: %+v", pt)
	}
	if l7, _ := st.TopL7(since, keep, 10); len(l7) != 1 {
		t.Errorf("kalan agent'in l7'si silinmis: %+v", l7)
	}
	if dns, _ := st.TopAgentDNS(since, keep, 10); len(dns) != 1 {
		t.Errorf("kalan agent'in dns'i silinmis: %+v", dns)
	}
}

// TestAgentHistoryCounterReset, AgentHistory'nin de ayni alt tasma korumasina
// sahip oldugunu dogrular.
func TestAgentHistoryCounterReset(t *testing.T) {
	st := openTest(t)
	now := time.Now().Unix()

	id, err := st.RegisterAgent(Agent{Name: "laptop-02", Site: "ofis", TokenHash: TokenHash("t3")})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := st.SaveIfaceSamples(id, now-60, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 9_000_000, TxBytes: 4_000_000, RxPackets: 9000, TxPackets: 4000},
	}); err != nil {
		t.Fatalf("ornek1: %v", err)
	}
	if err := st.SaveIfaceSamples(id, now, []telemetry.InterfaceSample{
		{Name: "Wi-Fi", RxBytes: 200, TxBytes: 100, RxPackets: 2, TxPackets: 1},
	}); err != nil {
		t.Fatalf("ornek2 (reset sonrasi): %v", err)
	}

	buckets, err := st.AgentHistory(id, time.Unix(now-120, 0))
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	for _, b := range buckets {
		if b.In < 0 || b.Out < 0 || b.Pps < 0 {
			t.Fatalf("negatif deger olmamali: %+v", b)
		}
		// duzeltmeden once burada ~1.5e17 bayt/sn gibi devasa bir deger olurdu
		if b.In > 1e6 || b.Out > 1e6 {
			t.Fatalf("sayac gerilemesinde makul olmayan yuksek deger: %+v", b)
		}
	}
}
