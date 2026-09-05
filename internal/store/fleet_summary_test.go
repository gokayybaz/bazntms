package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestFleetSummary, WS tick'inde yayınlanan filo özetinin agent sayıları +
// bit/sn + NetFlow hızını doğru hesapladığını dogrular.
func TestFleetSummary(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "fsum.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now()

	// 3 agent: 2'si son 60 sn'de görüldü (online), 1'i eski
	on1, _ := st.RegisterAgent(Agent{Name: "a1", TokenHash: "h1"})
	on2, _ := st.RegisterAgent(Agent{Name: "a2", TokenHash: "h2"})
	off, _ := st.RegisterAgent(Agent{Name: "a3", TokenHash: "h3"})
	_ = off
	st.TouchAgent(on1, "v", 1, "1.1.1.1")
	st.TouchAgent(on2, "v", 1, "1.1.1.2")
	// a3'ü eski göster
	st.(*sqlStore).db.Exec(`UPDATE agents SET last_seen = ? WHERE id = ?`, now.Add(-1*time.Hour).Unix(), off)

	// on1: eth0 sayacı ~1 MB/60 sn artıyor (~133 kbit/sn)
	var rx uint64 = 5_000_000
	for i := 0; i < 3; i++ {
		ts := now.Add(time.Duration(-120+i*60) * time.Second).Unix()
		if err := st.SaveIfaceSamples(on1, ts, []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: rx, TxBytes: rx / 4, RxPackets: uint64(i * 100)},
		}); err != nil {
			t.Fatalf("iface: %v", err)
		}
		rx += 1_000_000
	}
	// birkaç NetFlow (son 60 sn)
	rows := make([]FlowRow, 5)
	for i := range rows {
		rows[i] = FlowRow{Ts: now.Add(-30 * time.Second).Unix(), Device: "r", Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Octets: 100}
	}
	if err := st.SaveFlows(rows); err != nil {
		t.Fatalf("flows: %v", err)
	}

	fs, err := st.FleetSummary(60 * time.Second)
	if err != nil {
		t.Fatalf("FleetSummary: %v", err)
	}
	if fs.AgentsTotal != 3 || fs.AgentsOnline != 2 {
		t.Fatalf("agent sayıları: total=%d online=%d", fs.AgentsTotal, fs.AgentsOnline)
	}
	// rx: 2M bayt delta / 120 sn * 8 ≈ 133 kbit/sn
	if fs.RxBps < 80_000 || fs.RxBps > 250_000 {
		t.Fatalf("rx_bps beklenen ~133k, gelen %.0f", fs.RxBps)
	}
	if fs.TxBps <= 0 || fs.TxBps >= fs.RxBps {
		t.Fatalf("tx_bps hatalı: %.0f (rx=%.0f)", fs.TxBps, fs.RxBps)
	}
	if fs.FlowsPerMin != 5 {
		t.Fatalf("flows_per_min: %d", fs.FlowsPerMin)
	}
}
