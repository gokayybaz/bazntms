package alert

// Faz 6.2: anomali motoru birim testi — guvenilir baseline uzerinde
// ani sapma "anomaly" uyarisi uretmeli; duzgun trafik uretmemeli.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func newAnomalyManager(t *testing.T, cfg Config) (*Manager, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "anomaly.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(cfg, st, capture.NewEngine(), 30), st
}

// seedIfaceRun, tek agent/arayuz icin kumulatif rx sayacli bir ornek dizisi
// yazar; bir sonraki dizinin devam edebilmesi icin son sayac degerini doner.
func seedIfaceRun(t *testing.T, st store.Store, agentID, startTs int64, n int, stepSecs int64, startBytes uint64, perStepBytes func(i int) uint64) uint64 {
	t.Helper()
	b := startBytes
	for i := 0; i < n; i++ {
		if err := st.SaveIfaceSamples(agentID, startTs+int64(i)*stepSecs, []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: b, RxPackets: uint64(i)},
		}); err != nil {
			t.Fatalf("iface ornek: %v", err)
		}
		b += perStepBytes(i)
	}
	return b
}

func TestAnomalyFires(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()

	// guvenilir baseline: degerler ~1000 bps (az dalgalanma). HourlyBpsStats
	// saat-of-day kovalarini SON 7 GUNDEN toplar (bkz. topology.go); bu
	// yuzden ornekleri "simdiden geriye N saniye" seklinde degil, GECMIS
	// GUNLERİN AYNI saatine (now.Hour()) sabitleyerek ureterek testin
	// hangi DAKIKADA kosarsa kosun mevcut saat kovasinda >=120 ornek
	// bulmasini garanti ediyoruz (onceki surum saatin ilk birkac
	// dakikasinda calisirsa basarisiz oluyordu — bkz. commit mesaji).
	for day := 1; day <= 5; day++ {
		hourStart := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 0, 0, 0, now.Location())
		for i := 0; i < 40; i++ {
			val := float64(1000 + (i%20)*10)                       // 1000-1190
			ts := hourStart.Add(time.Duration(i*90) * time.Second) // saat icinde ~60 dk'ya yayilir
			if err := st.InsertSample(store.Sample{
				Ts: ts.Unix(), Device: "en0", BpsIn: val, BpsOut: 0, Pps: 1,
			}); err != nil {
				t.Fatalf("baseline ornek: %v", err)
			}
		}
	}
	// mevcut pencere: ani yukselis (son 5 dk icinde 300K bps)
	for i := 0; i < 10; i++ {
		if err := st.InsertSample(store.Sample{Ts: now.Unix() - int64(i), Device: "en0", BpsIn: 300_000, Pps: 500}); err != nil {
			t.Fatalf("spike ornek: %v", err)
		}
	}

	cfg := DefaultConfig()
	m.checkAnomaly(cfg)

	events := m.RecentEvents(10)
	found := false
	for _, e := range events {
		if e.Kind == "anomaly" {
			found = true
		}
	}
	if !found {
		t.Fatal("anomali uyarisi uretilmedi")
	}
}

// TestAnomalyFiresFromFleet, `samples` bos (coklu-hub) iken baseline'in agent
// arayuz telemetrisinden (`agent_iface_samples`) hesaplandigini ve ani filo
// verim sicramasinin "anomaly" uyarisi urettigini dogrular.
func TestAnomalyFiresFromFleet(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()

	// baseline: son 5 gunun ayni saati, ~8 kbit/sn (±dalgalanma). Kumulatif
	// sayac ts ile MONOTON artmali → en eski gun once yazilir.
	jitter := func(i int) uint64 { return uint64(55_000 + (i%9)*2_500) } // 55-75 KB / 60 sn
	var last uint64 = 5_000_000
	for day := 5; day >= 1; day-- {
		hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 2, 0, 0, now.Location())
		last = seedIfaceRun(t, st, 1, hs.Unix(), 50, 60, last, jitter)
	}
	// mevcut pencere: son ~5 dk ani yukselis (~800 kbit/sn — baseline'in ~100x)
	seedIfaceRun(t, st, 1, now.Unix()-300, 12, 30, last, func(int) uint64 { return 3_000_000 })

	m.checkAnomaly(DefaultConfig())

	found := false
	for _, e := range m.RecentEvents(10) {
		if e.Kind == "anomaly" {
			found = true
		}
	}
	if !found {
		t.Fatal("filo baseline'inden anomali uyarisi uretilmedi")
	}
}

func TestAnomalyQuietOnNormalTraffic(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now().Unix()

	// baseline ve mevcut pencere ayni aralikta: z-skoru esigi altinda
	for i := int64(0); i < 200; i++ {
		val := float64(1000 + (i%20)*10)
		ts := now - int64(i) // hepsi son dakikalar icinde
		if i >= 150 {
			ts = now - 2000 // bir kismi biraz geride, pencere harici
		}
		if err := st.InsertSample(store.Sample{Ts: ts, Device: "en0", BpsIn: val, Pps: 1}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}

	m.checkAnomaly(DefaultConfig())
	for _, e := range m.RecentEvents(10) {
		if e.Kind == "anomaly" {
			t.Fatalf("normal trafikte anomali uretildi: %s", e.Message)
		}
	}
}

func TestAnomalyDisabledAndLegacy(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now().Unix()
	for i := int64(0); i < 200; i++ {
		if err := st.InsertSample(store.Sample{Ts: now - int64(i), Device: "en0", BpsIn: 1000, Pps: 1}); err != nil {
			t.Fatalf("ornek: %v", err)
		}
	}

	// kapatilmis anomali: buyuk sapma olsa bile uyari yok
	cfg := DefaultConfig()
	cfg.Anomaly.Enabled = false
	for i := 0; i < 10; i++ {
		st.InsertSample(store.Sample{Ts: now - int64(i), Device: "en0", BpsIn: 500_000, Pps: 500})
	}
	m.checkAnomaly(cfg)
	for _, e := range m.RecentEvents(10) {
		if e.Kind == "anomaly" {
			t.Fatal("kapatilmis anomali uyarisi uretildi")
		}
	}
}
