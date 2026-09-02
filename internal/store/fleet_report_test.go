package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// fleetSeed, tek agent + 2 saatlik 1 dk aralikli kumulatif arayuz ornekleri
// (araya bir sayac reset'i konur) + NetFlow + cihaz arayuz ornekleri yazar.
func fleetSeed(t *testing.T) (Store, int64) {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	aid, err := st.RegisterAgent(Agent{Name: "a1", TokenHash: "h1"})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}

	base := time.Now().Add(-2 * time.Hour).Truncate(time.Minute)
	var rx, tx uint64
	for i := 0; i < 120; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		if i == 60 {
			rx, tx = 0, 0 // sayac reset (arayuz/agent yeniden basladi)
		}
		rx += 1_000_000 // 1 MB/dk in  → ~133 Kbit/s
		tx += 250_000
		if err := st.SaveIfaceSamples(aid, ts.Unix(), []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: rx, TxBytes: tx, RxPackets: rx / 1000, TxPackets: tx / 1000},
		}); err != nil {
			t.Fatalf("iface: %v", err)
		}
	}

	if err := st.SaveFlows([]FlowRow{
		{Ts: base.Add(time.Hour).Unix(), Src: "10.0.0.1", Dst: "8.8.8.8", Proto: "udp", Packets: 10, Octets: 5000},
		{Ts: base.Add(time.Hour).Unix(), Src: "10.0.0.1", Dst: "1.1.1.1", Proto: "tcp", Packets: 100, Octets: 900000},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}
	return st, aid
}

func TestFleetTrafficBuckets(t *testing.T) {
	st, _ := fleetSeed(t)
	since := time.Now().Add(-3 * time.Hour)

	buckets, err := st.FleetTrafficBuckets(since, 300)
	if err != nil {
		t.Fatalf("buckets: %v", err)
	}
	if len(buckets) == 0 {
		t.Fatalf("kova yok")
	}
	var totalIn float64
	for _, b := range buckets {
		if b.In < 0 || b.Out < 0 {
			t.Fatalf("negatif hiz: %+v", b)
		}
		totalIn += b.In * 300
	}
	// ~2 saat × 1 MB/dk ≈ 120 MB in (reset noktasi bir kovanin bir kismini
	// dusurebilir; genis bir alt sinir yeterli)
	if totalIn < 80e6 || totalIn > 130e6 {
		t.Fatalf("toplam gelen bayt beklenen aralikta degil: %.0f", totalIn)
	}
}

func TestFleetTopEndpointsAndProtocols(t *testing.T) {
	st, _ := fleetSeed(t)
	since := time.Now().Add(-3 * time.Hour)

	eps, err := st.FleetTopEndpoints(since, 10)
	if err != nil {
		t.Fatalf("endpoints: %v", err)
	}
	if len(eps) == 0 {
		t.Fatalf("endpoint yok")
	}
	// 10.0.0.1 iki akisin da kaynagi → toplam hacim en yuksek (905000);
	// gelen/giden yonler ayri toplanir.
	if eps[0].IP != "10.0.0.1" || eps[0].BytesOut != 905000 {
		t.Fatalf("siralama hatali: %+v", eps)
	}
	var oneone EndpointDelta
	for _, e := range eps {
		if e.IP == "1.1.1.1" {
			oneone = e
		}
	}
	if oneone.BytesIn != 900000 {
		t.Fatalf("1.1.1.1 gelen hacmi hatali: %+v", oneone)
	}

	protos, err := st.FleetProtocolTotals(since)
	if err != nil {
		t.Fatalf("protos: %v", err)
	}
	if protos["TCP"] != 900000 || protos["UDP"] != 5000 {
		t.Fatalf("protokol toplamlari hatali: %v", protos)
	}
}

func TestFleetTopEndpointsFallback(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "fb.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	aid, _ := st.RegisterAgent(Agent{Name: "a1", TokenHash: "h1"})
	// NetFlow yok — process_traffic'e dusmeli
	if err := st.SaveProcessTraffic(aid, time.Now().Add(-10*time.Minute).Unix(), []telemetry.ProcessTrafficSample{
		{Process: "p", Proto: "tcp", RemoteIP: "9.9.9.9", BytesIn: 1000, BytesOut: 200},
	}); err != nil {
		t.Fatalf("proctraffic: %v", err)
	}
	eps, err := st.FleetTopEndpoints(time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("endpoints: %v", err)
	}
	if len(eps) != 1 || eps[0].IP != "9.9.9.9" || eps[0].BytesIn != 1000 {
		t.Fatalf("fallback hatali: %+v", eps)
	}
}

func TestFleetIfaceHealthEmpty(t *testing.T) {
	st, _ := fleetSeed(t)
	disc, errs, err := st.FleetIfaceHealth(time.Now().Add(-3 * time.Hour))
	if err != nil {
		t.Fatalf("iface health: %v", err)
	}
	if disc != 0 || errs != 0 {
		t.Fatalf("cihaz arayuzu yokken 0/0 bekleniyordu: %d/%d", disc, errs)
	}
}
