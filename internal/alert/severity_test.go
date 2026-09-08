package alert

// S22.7: severity — operatör geçersiz kılması + anomali z-büyüklüğü eskalasyonu.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func TestSeverityOverride(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sev.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := DefaultConfig()
	cfg.Severities = map[string]string{"proc": "crit", "bogus": "nonsense"}
	m := NewManager(cfg, st, capture.NewEngine(), 30)

	m.fire("proc", "chrome", "yeni süreç")    // varsayılan info → override crit
	m.fire("target", "1.2.3.4", "yeni hedef") // override yok → info

	byKey := map[string]store.AlertEvent{}
	for _, e := range m.RecentEvents(10) {
		byKey[e.Kind] = e
	}
	if byKey["proc"].Severity != "crit" {
		t.Fatalf("proc override crit olmalı: %q", byKey["proc"].Severity)
	}
	if byKey["target"].Severity != "info" {
		t.Fatalf("target varsayılan info olmalı: %q", byKey["target"].Severity)
	}
	// geçersiz override değeri yok sayılır
	if got := cfg.severityFor("bogus"); got != "warn" {
		t.Fatalf("geçersiz override → varsayılan warn, %q", got)
	}
}

func TestAnomalySeverityByZ(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()
	aid, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})

	// baseline: geçmiş 2 gün, düşük ve durgun. Üretimde baseline saatlik
	// rebuild'lenir; spike'tan ÖNCEki hale karşılık gelir → önce rebuild, sonra spike.
	var last uint64 = 1_000_000
	for day := 2; day >= 1; day-- {
		hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 1, 0, 0, now.Location())
		last = seedIfaceRun(t, st, aid, hs.Unix(), 40, 60, last, func(i int) uint64 { return 58_000 + uint64(i%5)*2_000 })
	}

	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	cfg.Anomaly.MinSamples = 20
	cfg.Anomaly.MinAbsDeltaBps = 100_000
	m.rebuildAnomalyBaseline(cfg)

	// çok büyük sıçrama → |z| ≫ CritZ(5) → crit
	seedIfaceRun(t, st, aid, now.Unix()-240, 8, 30, last, func(int) uint64 { return 20_000_000 })
	m.checkAnomaly(cfg)

	var crit bool
	for _, e := range m.RecentEvents(10) {
		if e.Kind == "anomaly" && strings.HasPrefix(e.Key, "bps:fleet:") {
			crit = e.Severity == "crit"
		}
	}
	if !crit {
		t.Fatalf("büyük z → crit bekleniyordu: %+v", m.RecentEvents(10))
	}
}
