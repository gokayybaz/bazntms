package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// TestFleetBaseline, FleetHourlyBpsStats + FleetAvgBpsSince'in agent arayuz
// telemetrisinden makul bit/sn baseline urettigini ve fiziksel olarak
// imkansiz sayac artislarini (bozuk/sanal arayuz) eledigini dogrular.
func TestFleetBaseline(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "fb.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	aid, _ := st.RegisterAgent(Agent{Name: "a1", TokenHash: "h1"})

	now := time.Now()
	// saglikli arayuz: ~800 Kbit/sn (6 MB / 60 sn), son 3 saat
	var good uint64 = 10_000_000
	// bozuk arayuz: her ornekte +400 GB (~53 Gbit/sn — canli veride gorulen
	// "18 Tbit/sn" senaryosu) — maxIfaceBps ile elenmeli
	var bad uint64 = 0
	start := now.Add(-3 * time.Hour).Truncate(time.Minute)
	for i := 0; i < 180; i++ {
		ts := start.Add(time.Duration(i) * time.Minute).Unix()
		good += 6_000_000
		bad += 400_000_000_000
		if err := st.SaveIfaceSamples(aid, ts, []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: good},
			{Name: "tap-bogus", RxBytes: bad},
		}); err != nil {
			t.Fatalf("iface: %v", err)
		}
	}

	stats, err := st.FleetHourlyBpsStats()
	if err != nil {
		t.Fatalf("FleetHourlyBpsStats: %v", err)
	}
	var total int64
	for _, h := range stats {
		total += h.Count
		// bozuk arayuz elenmezse ortalama TB/sn mertebesine cikardi
		if h.Mean > 50_000_000 { // 50 Mbit/sn — saglikli senaryonun ~60x ustu
			t.Fatalf("saat %d: baseline bozuk arayuzden zehirlenmis (mean=%.0f bps)", h.Hour, h.Mean)
		}
	}
	if total < 120 {
		t.Fatalf("baseline kova sayisi dusuk: %d", total)
	}

	// pencere ortalamasi: yalniz saglikli arayuz → ~800 Kbit/sn civari
	avg, err := st.FleetAvgBpsSince(now.Add(-30 * time.Minute))
	if err != nil {
		t.Fatalf("FleetAvgBpsSince: %v", err)
	}
	if avg < 400_000 || avg > 1_600_000 {
		t.Fatalf("pencere ort beklenen ~800 Kbit/sn, gelen: %.0f bps", avg)
	}
}
