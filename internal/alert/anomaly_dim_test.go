package alert

// S22.3: çok-boyutlu anomali — bir agent sıçrarsa yalnız o agent'ın kovası
// (ve filo toplamı) uyarı üretmeli; normal seyreden agent üretmemeli.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func TestAnomalyPerAgentDimension(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()
	a1, _ := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: "h1"})
	a2, _ := st.RegisterAgent(store.Agent{Name: "a2", TokenHash: "h2"})

	base := func(i int) uint64 { return uint64(55_000 + (i%7)*3_000) } // ~8 kbit/sn / 60 sn
	for _, aid := range []int64{a1, a2} {
		var last uint64 = 1_000_000
		for day := 2; day >= 1; day-- {
			hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 1, 0, 0, now.Location())
			last = seedIfaceRun(t, st, aid, hs.Unix(), 40, 60, last, base)
		}
		if aid == a1 {
			// mevcut pencerede ani yükseliş (~1.3 Mbit/sn — baseline'in ~160x)
			seedIfaceRun(t, st, aid, now.Unix()-240, 8, 30, last, func(int) uint64 { return 5_000_000 })
		} else {
			seedIfaceRun(t, st, aid, now.Unix()-240, 8, 30, last, base)
		}
	}

	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	cfg.Anomaly.MinSamples = 20
	cfg.Anomaly.MinAbsDeltaBps = 100_000
	m.rebuildAnomalyBaseline(cfg)
	m.checkAnomaly(cfg)

	var agentAnoms []store.AlertEvent
	for _, e := range m.RecentEvents(20) {
		if e.Kind == "anomaly" && strings.HasPrefix(e.Key, "bps:agent:") {
			agentAnoms = append(agentAnoms, e)
		}
	}
	if len(agentAnoms) != 1 {
		t.Fatalf("tam 1 agent anomalisi bekleniyordu (a1 sıçradı, a2 normal), gelen %d: %+v", len(agentAnoms), agentAnoms)
	}
	if !strings.Contains(agentAnoms[0].Key, fmt.Sprintf(":%d:", a1)) {
		t.Fatalf("anomali a1 (#%d) için olmalı: %s", a1, agentAnoms[0].Key)
	}
}

// TestAnomalyMaxSurfaced, çok sayıda eşzamanlı sapmada yüzeye çıkan uyarı
// sayısının MaxSurfaced ile sınırlandığını doğrular.
func TestAnomalyMaxSurfaced(t *testing.T) {
	m, st := newAnomalyManager(t, DefaultConfig())
	now := time.Now()

	base := func(i int) uint64 { return uint64(55_000 + (i%7)*3_000) }
	var ids []int64
	for a := 0; a < 5; a++ {
		aid, _ := st.RegisterAgent(store.Agent{Name: fmt.Sprintf("a%d", a), TokenHash: fmt.Sprintf("h%d", a)})
		ids = append(ids, aid)
		var last uint64 = 1_000_000
		for day := 2; day >= 1; day-- {
			hs := time.Date(now.Year(), now.Month(), now.Day()-day, now.Hour(), 1, 0, 0, now.Location())
			last = seedIfaceRun(t, st, aid, hs.Unix(), 40, 60, last, base)
		}
		// hepsi birden sıçrasın
		seedIfaceRun(t, st, aid, now.Unix()-240, 8, 30, last, func(int) uint64 { return 5_000_000 })
	}

	cfg := DefaultConfig()
	cfg.Anomaly.Seasonality = "hourly"
	cfg.Anomaly.MinSamples = 20
	cfg.Anomaly.MinAbsDeltaBps = 100_000
	cfg.Anomaly.MaxSurfaced = 3
	m.rebuildAnomalyBaseline(cfg)
	m.checkAnomaly(cfg)

	var n int
	for _, e := range m.RecentEvents(50) {
		if e.Kind == "anomaly" {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("MaxSurfaced=3 → 3 uyarı bekleniyordu (5 agent + filo sıçradı), gelen %d", n)
	}
	_ = ids
}
