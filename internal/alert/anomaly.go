package alert

// Anomali tespiti (Faz 6.2 · S22.1 materyalize · S22.2 mevsimsel + EWMA):
// AI'sız erken uyarı. Mevsimsel bir baseline (hafta içi/sonu × saat) ile o
// anki verim karşılaştırılır; z-skoru eşiği aşılırsa "anomaly" uyarısı üretilir.
//
//   - Baseline lider-kapılı saatlik rebuildAnomalyBaseline ile materyalize
//     edilir (anomaly_baseline tablosu) — checkAnomaly değerlendirme başına
//     canlı LAG taraması yapmaz.
//   - Rebuild, (kova × gün-yaşı) alt-toplamlarını gün yaşına göre EWMA
//     ağırlığıyla birleştirir: yeni günler eskilerden ağır basar (yavaş drift).
//   - std = sqrt(m2/n) Go tarafında hesaplanır (SQLite'ta SQL sqrt yok).

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type AnomalyConfig struct {
	Enabled     bool    `json:"enabled"`
	Sensitivity float64 `json:"sensitivity"` // z-skoru esigi (varsayilan 3.0)
	MinSamples  int     `json:"min_samples"` // baseline guvenilirlik esigi (varsayilan 120)
	WindowMin   int     `json:"window_min"`  // karsilastirma penceresi dk (varsayilan 5)

	// S22.2 — mevsimsel model
	Seasonality  string  `json:"seasonality"`   // "hourly" | "weekday" | "dow" (varsayilan "weekday")
	BaselineDays int     `json:"baseline_days"` // baseline penceresi gun (varsayilan 21)
	EWMAHalfLife float64 `json:"ewma_half_life_days"`
}

// seasonalities, gecerli Seasonality degerleri (bilinmeyen → "weekday").
var seasonalities = map[string]bool{"hourly": true, "weekday": true, "dow": true}

func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		Enabled:      true,
		Sensitivity:  3.0,
		MinSamples:   120, // ~2 saat ornek (1/sn) / 2 saat filo 60sn kova
		WindowMin:    5,
		Seasonality:  "weekday",
		BaselineDays: 21,
		EWMAHalfLife: 10, // gun — 0 = esit agirlik
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

// checkAnomaly, periyodik cagirilir: mevcut pencere verimini bu zaman
// diliminin (mevsimsel kova) baseline'i ile karsilastirir. Yalnizca baseline
// guvenilir (>= MinSamples) ve std > 0 iken degerlendirir. Baseline once filo
// (dim="fleet"), yetersizse hub yerel yakalamasi (dim="local").
func (m *Manager) checkAnomaly(cfg Config) {
	ac := cfg.Anomaly.normalized()
	if !ac.Enabled {
		return
	}
	curBucket := store.SeasonalBucket(time.Now(), ac.Seasonality)
	base, fleet := m.anomalyBaseline(curBucket, int64(ac.MinSamples))
	if base == nil {
		slog.Debug("anomali baseline isiniyor — yeterli ornek yok", "kova", curBucket, "min", ac.MinSamples)
		return
	}
	std := base.Std()
	if std <= 0 {
		return
	}
	window := time.Duration(ac.WindowMin) * time.Minute
	var cur float64
	var err error
	if fleet {
		cur, err = m.st.FleetAvgBpsSince(time.Now().Add(-window))
	} else {
		cur, err = m.st.AvgBpsSince(time.Now().Add(-window))
	}
	if err != nil {
		return
	}
	z := (cur - base.Mean) / std
	src := "yerel"
	if fleet {
		src = "filo"
	}
	slog.Debug("anomali baseline", "kaynak", src, "kova", curBucket, "n", base.N,
		"ort_bps", int64(base.Mean), "std_bps", int64(std), "son_bps", int64(cur), "z", math.Round(z*10)/10)
	direction := "yükseliş"
	if z < 0 {
		direction = "düşüş"
	}
	if math.Abs(z) >= ac.Sensitivity {
		m.fire("anomaly", fmt.Sprintf("bps:%d", curBucket),
			fmt.Sprintf("Trafiğe alışılmadık sapma (%s): %.0f bps — bu zaman dilimi ortalaması %.0f ± %.0f (z=%.1f, son %d dk)",
				direction, cur, base.Mean, std, z, ac.WindowMin))
	}
}

// anomalyBaseline, mevcut kovada >= minSamples ornek iceren ilk baseline
// satirini dondurur: once filo (dim="fleet"), sonra hub yerel (dim="local").
// fleet=true ise current-window karsilastirmasi da FleetAvgBpsSince ile
// yapilmali. Iki boyutta da yeterli veri yoksa (base=nil) motor sessiz kalir.
//
// S22.1: kaynak materyalize anomaly_baseline tablosu (canli LAG taramasi yok).
func (m *Manager) anomalyBaseline(bucket int, minSamples int64) (base *store.AnomalyBaselineRow, fleet bool) {
	if rows, err := m.st.LoadAnomalyBaseline("fleet", "bps"); err == nil {
		if b := baselineBucket(rows, bucket); b != nil && b.N >= minSamples {
			return b, true
		}
	}
	if rows, err := m.st.LoadAnomalyBaseline("local", "bps"); err == nil {
		if b := baselineBucket(rows, bucket); b != nil && b.N >= minSamples {
			return b, false
		}
	}
	return nil, false
}

func baselineBucket(rows []store.AnomalyBaselineRow, bucket int) *store.AnomalyBaselineRow {
	for i := range rows {
		if rows[i].Bucket == bucket {
			return &rows[i]
		}
	}
	return nil
}

// rebuildAnomalyBaseline, lider-kapili saatlik: materyalize baseline tablosunu
// filo (agent_iface_samples) ve hub yerel (samples) mevsimsel alt-toplamlariyla
// gunceller.
func (m *Manager) rebuildAnomalyBaseline(cfg Config) {
	ac := cfg.Anomaly.normalized()
	var rows []store.AnomalyBaselineRow
	if fb, err := m.st.FleetBaselineDayBuckets(ac.BaselineDays, ac.Seasonality); err == nil {
		rows = append(rows, combineBaseline("fleet", "bps", fb, ac.EWMAHalfLife)...)
	} else {
		slog.Debug("anomali baseline: filo alt-toplamlari okunamadi", "err", err)
	}
	if lb, err := m.st.BaselineDayBuckets(ac.BaselineDays, ac.Seasonality); err == nil {
		rows = append(rows, combineBaseline("local", "bps", lb, ac.EWMAHalfLife)...)
	}
	if len(rows) == 0 {
		return // taze kurulum / veri yok — tabloya dokunma
	}
	if err := m.st.SaveAnomalyBaseline(rows); err != nil {
		slog.Warn("anomali baseline yazilamadi", "err", err)
	}
}

// combineBaseline, (kova × gun-yasi) alt-toplamlarini kova basina tek bir
// baseline satirina indirger. halfLife > 0 ise her gun-yasi 2^(-yas/halfLife)
// ile agirliklanir (EWMA): ortalama ve varyans agirlikli, n (MinSamples
// geridi) agirliksiz ham ornek sayisi.
func combineBaseline(dim, metric string, buckets []store.BaselineDayBucket, halfLife float64) []store.AnomalyBaselineRow {
	type acc struct {
		rawN             int64
		wN, wSum, wSumSq float64
	}
	byBucket := map[int]*acc{}
	for _, b := range buckets {
		a := byBucket[b.Bucket]
		if a == nil {
			a = &acc{}
			byBucket[b.Bucket] = a
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
	out := make([]store.AnomalyBaselineRow, 0, len(byBucket))
	for bucket, a := range byBucket {
		if a.wN <= 0 {
			continue
		}
		mean := a.wSum / a.wN
		variance := a.wSumSq/a.wN - mean*mean
		if variance < 0 {
			variance = 0 // kayan nokta artigi
		}
		out = append(out, store.AnomalyBaselineRow{
			Dim: dim, Metric: metric, Key: "", Bucket: bucket,
			N: a.rawN, Mean: mean, M2: float64(a.rawN) * variance,
		})
	}
	return out
}
