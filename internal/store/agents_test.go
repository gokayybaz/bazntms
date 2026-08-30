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
	if err := st.TouchAgent(id, "v1", "10.0.0.5"); err != nil {
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
	if err := st.TouchAgent(id, "v1", "10.0.0.6"); err != nil {
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
