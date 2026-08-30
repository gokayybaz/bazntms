package alert

// Faz 6.2: anomali motoru birim testi — guvenilir baseline uzerinde
// ani sapma "anomaly" uyarisi uretmeli; duzgun trafik uretmemeli.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newAnomalyManager(t *testing.T, cfg Config) (*Manager, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "anomaly.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewManager(cfg, st, capture.NewEngine()), st
}

func TestAnomalyFires(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now().Unix()

	// guvenilir baseline: degerler ~1000 bps (az dalgalanma). Ornekler son
	// ~2 saate 2 sn aralikla serpistirilir; boylece test gunun hangi
	// dakikasinda kosarsa kosun guncel saat kovasinda >=120 ornek olur
	// (saat sinirina duyarli degil). Tum baseline, 5 dk'lik karsilastirma
	// penceresinin disinda kalir (>= 310 sn once).
	for i := 0; i < 2000; i++ {
		val := float64(1000 + (i%20)*10) // 1000-1190
		if err := st.InsertSample(store.Sample{
			Ts: now - int64(310+i*2), Device: "en0", BpsIn: val, BpsOut: 0, Pps: 1,
		}); err != nil {
			t.Fatalf("baseline ornek: %v", err)
		}
	}
	// mevcut pencere: ani yukselis (son 5 dk icinde 300K bps)
	for i := 0; i < 10; i++ {
		if err := st.InsertSample(store.Sample{Ts: now - int64(i), Device: "en0", BpsIn: 300_000, Pps: 500}); err != nil {
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
