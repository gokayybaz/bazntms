package alert

// Anomali tespiti (Faz 6.2 · S22.1 materyalize · S22.2 mevsimsel · S22.3
// çok-boyutlu): AI'sız erken uyarı. Mevsimsel bir baseline (hafta içi/sonu ×
// saat) ile o anki verim boyut bazında (filo / saha / agent) karşılaştırılır;
// z-skoru eşiği aşılırsa "anomaly" uyarısı üretilir.
//
//   - Baseline lider-kapılı saatlik rebuildAnomalyBaseline ile materyalize
//     edilir (anomaly_baseline tablosu) — checkAnomaly değerlendirme başına
//     canlı LAG taraması yapmaz.
//   - Rebuild, (kova × gün-yaşı) alt-toplamlarını gün yaşına göre EWMA
//     ağırlığıyla birleştirir: yeni günler eskilerden ağır basar (yavaş drift).
//   - checkAnomaly her boyutu değerlendirir, adayları |z|'ye göre sıralar ve
//     en çok MaxSurfaced tanesini yüzeye çıkarır (5.000 agent'ta uyarı seli
//     olmasın). MinAbsDeltaBps altındaki sapmalar (sessiz-saat gürültüsü) elenir.
//   - std = sqrt(m2/n) Go tarafında hesaplanır (SQLite'ta SQL sqrt yok).

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type AnomalyConfig struct {
	Enabled     bool    `json:"enabled"`
	Sensitivity float64 `json:"sensitivity"` // z-skoru esigi (varsayilan 3.0)
	MinSamples  int     `json:"min_samples"` // baseline guvenilirlik esigi (varsayilan 120)
	WindowMin   int     `json:"window_min"`  // karsilastirma penceresi dk (varsayilan 5)

	// S22.2 — mevsimsel model
	Seasonality  string  `json:"seasonality"`         // "hourly" | "weekday" | "dow" (vars. "weekday")
	BaselineDays int     `json:"baseline_days"`       // baseline penceresi gun (vars. 21)
	EWMAHalfLife float64 `json:"ewma_half_life_days"` // gun yasi yari-omru; 0 = esit agirlik

	// S22.3 — cok-boyutlu
	PerSite        bool    `json:"per_site"`          // saha bazli baseline (vars. acik)
	PerAgent       bool    `json:"per_agent"`         // agent bazli baseline (vars. acik)
	MaxSurfaced    int     `json:"max_surfaced"`      // tek degerlendirmede en cok kac uyari (vars. 8)
	MinAbsDeltaBps float64 `json:"min_abs_delta_bps"` // gurultu tabani (vars. 500000 = 0.5 Mbit/sn)
}

// seasonalities, gecerli Seasonality degerleri (bilinmeyen → "weekday").
var seasonalities = map[string]bool{"hourly": true, "weekday": true, "dow": true}

func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		Enabled:        true,
		Sensitivity:    3.0,
		MinSamples:     120, // ~2 saat ornek (1/sn) / 2 saat filo 60sn kova
		WindowMin:      5,
		Seasonality:    "weekday",
		BaselineDays:   21,
		EWMAHalfLife:   10, // gun — 0 = esit agirlik
		PerSite:        true,
		PerAgent:       true,
		MaxSurfaced:    8,
		MinAbsDeltaBps: 500_000,
	}
}

// normalized, legacy configlerde (JSON'da alan yok / sifir) degerleri
// varsayilanlarla doldurur.
func (a AnomalyConfig) normalized() AnomalyConfig {
	d := DefaultAnomalyConfig()
	if a.Sensitivity <= 0 {
		a.Sensitivity = d.Sensitivity
	}
	if a.MinSamples <= 0 {
		a.MinSamples = d.MinSamples
	}
	if a.WindowMin <= 0 {
		a.WindowMin = d.WindowMin
	}
	if !seasonalities[a.Seasonality] {
		a.Seasonality = d.Seasonality
	}
	if a.BaselineDays <= 0 {
		a.BaselineDays = d.BaselineDays
	} else if a.BaselineDays > 90 {
		a.BaselineDays = 90
	}
	if a.EWMAHalfLife < 0 {
		a.EWMAHalfLife = 0
	}
	// MaxSurfaced == 0 → config S22.3 oncesi; yeni alanlar icin varsayilanlari
	// benimse (per-site/agent'ı acık getir — mevcut kurulumlarda da devrede olsun).
	if a.MaxSurfaced == 0 {
		a.PerSite, a.PerAgent = d.PerSite, d.PerAgent
		a.MaxSurfaced = d.MaxSurfaced
		if a.MinAbsDeltaBps == 0 {
			a.MinAbsDeltaBps = d.MinAbsDeltaBps
		}
	}
	if a.MaxSurfaced < 0 {
		a.MaxSurfaced = d.MaxSurfaced
	}
	if a.MinAbsDeltaBps < 0 {
		a.MinAbsDeltaBps = 0
	}
	return a
}

// NormalizeConfig, DB'den okunan eski JSON configlerini yeni alanlarla
// uyumlu hale getirir (anomali/IOC alani yoksa varsayilanla acilir).
func NormalizeConfig(cfg Config) Config {
	if cfg.Anomaly.Sensitivity <= 0 {
		cfg.Anomaly = DefaultAnomalyConfig()
	} else {
		cfg.Anomaly = cfg.Anomaly.normalized()
	}
	if cfg.IOC == (IOCConfig{}) { // eski config: "ioc" alani yok → varsayilan (acik)
		cfg.IOC = DefaultIOCConfig()
	}
	return cfg
}

// anomalyCand, degerlendirmede esigi asan tek bir (boyut, anahtar) sapmasi.
type anomalyCand struct {
	dim, key          string
	z, cur, mean, std float64
}

// checkAnomaly, periyodik cagirilir: her boyutta mevcut pencere verimini bu
// zaman diliminin (mevsimsel kova) baseline'i ile karsilastirir, adaylari
// buyuklukce siralar ve en cok MaxSurfaced tanesini atesler.
func (m *Manager) checkAnomaly(cfg Config) {
	ac := cfg.Anomaly.normalized()
	if !ac.Enabled {
		return
	}
	curBucket := store.SeasonalBucket(time.Now(), ac.Seasonality)
	winStart := time.Now().Add(-time.Duration(ac.WindowMin) * time.Minute)
	var cands []anomalyCand

	eval := func(dim string, baseline []store.AnomalyBaselineRow, cur map[string]float64) {
		for i := range baseline {
			b := baseline[i]
			if b.Bucket != curBucket || b.N < int64(ac.MinSamples) {
				continue
			}
			std := b.Std()
			if std <= 0 {
				continue
			}
			c, ok := cur[b.Key]
			if !ok {
				continue
			}
			if math.Abs(c-b.Mean) < ac.MinAbsDeltaBps {
				continue // sessiz-saat gurultu tabani
			}
			if z := (c - b.Mean) / std; math.Abs(z) >= ac.Sensitivity {
				cands = append(cands, anomalyCand{dim: dim, key: b.Key, z: z, cur: c, mean: b.Mean, std: std})
			}
		}
	}

	// filo (her zaman) — baseline yoksa hub-yerel'e dus
	if rows := m.loadBaseline("fleet"); len(rows) > 0 {
		if v, err := m.st.FleetAvgBpsSince(winStart); err == nil {
			eval("fleet", rows, map[string]float64{"": v})
		}
	} else if rows := m.loadBaseline("local"); len(rows) > 0 {
		if v, err := m.st.AvgBpsSince(winStart); err == nil {
			eval("local", rows, map[string]float64{"": v})
		}
	}
	if ac.PerSite {
		if rows := m.loadBaseline("site"); len(rows) > 0 {
			if cur, err := m.st.AvgBpsByDim("site", winStart); err == nil {
				eval("site", rows, cur)
			}
		}
	}
	if ac.PerAgent {
		if rows := m.loadBaseline("agent"); len(rows) > 0 {
			if cur, err := m.st.AvgBpsByDim("agent", winStart); err == nil {
				eval("agent", rows, cur)
			}
		}
	}

	if len(cands) == 0 {
		return
	}
	sort.Slice(cands, func(i, j int) bool { return math.Abs(cands[i].z) > math.Abs(cands[j].z) })
	limit := ac.MaxSurfaced
	if limit <= 0 || limit > len(cands) {
		limit = len(cands)
	}
	slog.Debug("anomali", "kova", curBucket, "aday", len(cands), "yuzeye", limit)
	for _, c := range cands[:limit] {
		direction := "yükseliş"
		if c.z < 0 {
			direction = "düşüş"
		}
		m.fire("anomaly", fmt.Sprintf("bps:%s:%s:%d", c.dim, c.key, curBucket),
			fmt.Sprintf("%s: alışılmadık trafik sapması (%s) — %.0f bps, bu zaman dilimi ortalaması %.0f ± %.0f (z=%.1f, son %d dk)",
				anomalyScope(c.dim, c.key), direction, c.cur, c.mean, c.std, c.z, ac.WindowMin))
	}
}

func (m *Manager) loadBaseline(dim string) []store.AnomalyBaselineRow {
	rows, err := m.st.LoadAnomalyBaseline(dim, "bps")
	if err != nil {
		return nil
	}
	return rows
}

// anomalyScope, uyari mesajindaki insan-okur boyut etiketi.
func anomalyScope(dim, key string) string {
	switch dim {
	case "fleet":
		return "Filo geneli"
	case "local":
		return "Hub yerel"
	case "site":
		return "Saha " + key
	case "agent":
		return "Agent #" + key
	}
	return dim
}

// rebuildAnomalyBaseline, lider-kapili saatlik: materyalize baseline tablosunu
// tum etkin boyutlarin (filo / yerel / saha / agent) mevsimsel
// alt-toplamlariyla gunceller.
func (m *Manager) rebuildAnomalyBaseline(cfg Config) {
	ac := cfg.Anomaly.normalized()
	dims := []string{"fleet", "local"}
	if ac.PerSite {
		dims = append(dims, "site")
	}
	if ac.PerAgent {
		dims = append(dims, "agent")
	}
	var rows []store.AnomalyBaselineRow
	for _, dim := range dims {
		bk, err := m.st.BaselineDayBuckets(dim, ac.BaselineDays, ac.Seasonality)
		if err != nil {
			slog.Debug("anomali baseline alt-toplamlari okunamadi", "dim", dim, "err", err)
			continue
		}
		rows = append(rows, combineBaseline(dim, "bps", bk, ac.EWMAHalfLife)...)
	}
	if len(rows) == 0 {
		return // taze kurulum / veri yok — tabloya dokunma
	}
	if err := m.st.SaveAnomalyBaseline(rows); err != nil {
		slog.Warn("anomali baseline yazilamadi", "err", err)
	}
}

// combineBaseline, (anahtar × kova × gun-yasi) alt-toplamlarini (anahtar × kova)
// basina tek bir baseline satirina indirger. halfLife > 0 ise her gun-yasi
// 2^(-yas/halfLife) ile agirliklanir (EWMA): ortalama/varyans agirlikli, n
// (MinSamples geridi) agirliksiz ham ornek sayisi.
func combineBaseline(dim, metric string, buckets []store.BaselineDayBucket, halfLife float64) []store.AnomalyBaselineRow {
	type kb struct {
		key    string
		bucket int
	}
	type acc struct {
		rawN             int64
		wN, wSum, wSumSq float64
	}
	byKB := map[kb]*acc{}
	for _, b := range buckets {
		k := kb{b.Key, b.Bucket}
		a := byKB[k]
		if a == nil {
			a = &acc{}
			byKB[k] = a
		}
		w := 1.0
		if halfLife > 0 {
			w = math.Exp2(-float64(b.DayAge) / halfLife)
		}
		a.rawN += b.N
		a.wN += w * float64(b.N)
		a.wSum += w * b.Sum
		a.wSumSq += w * b.SumSq
	}
	out := make([]store.AnomalyBaselineRow, 0, len(byKB))
	for k, a := range byKB {
		if a.wN <= 0 {
			continue
		}
		mean := a.wSum / a.wN
		variance := a.wSumSq/a.wN - mean*mean
		if variance < 0 {
			variance = 0 // kayan nokta artigi
		}
		out = append(out, store.AnomalyBaselineRow{
			Dim: dim, Metric: metric, Key: k.key, Bucket: k.bucket,
			N: a.rawN, Mean: mean, M2: float64(a.rawN) * variance,
		})
	}
	return out
}
