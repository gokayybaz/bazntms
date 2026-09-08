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
	MinAbsDeltaBps float64 `json:"min_abs_delta_bps"` // bps/proc_bps gurultu tabani (vars. 500000)

	// S22.4 — bps disi metrikler
	Metrics        []string `json:"metrics"`           // "bps" | "dns_qps" | "proc_bps" (vars. ucu)
	MinAbsDeltaQps float64  `json:"min_abs_delta_qps"` // dns_qps gurultu tabani (vars. 5)
}

// knownMetrics, gecerli Metrics degerleri.
var knownMetrics = map[string]bool{"bps": true, "dns_qps": true, "proc_bps": true}

// minAbsDelta, bir metrigin mutlak-delta gurultu tabani (birimi metrige gore).
func (a AnomalyConfig) minAbsDelta(metric string) float64 {
	if metric == "dns_qps" {
		return a.MinAbsDeltaQps
	}
	return a.MinAbsDeltaBps
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
		Metrics:        []string{"bps", "dns_qps", "proc_bps"},
		MinAbsDeltaQps: 5,
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
	// S22.4 — metrik listesi: bilinmeyenleri ele, boşsa varsayılan üçlü.
	filtered := a.Metrics[:0]
	for _, mt := range a.Metrics {
		if knownMetrics[mt] {
			filtered = append(filtered, mt)
		}
	}
	a.Metrics = filtered
	if len(a.Metrics) == 0 {
		a.Metrics = d.Metrics
	}
	if a.MinAbsDeltaQps <= 0 {
		a.MinAbsDeltaQps = d.MinAbsDeltaQps
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

// anomalyCand, degerlendirmede esigi asan tek bir (metrik, boyut, anahtar) sapmasi.
type anomalyCand struct {
	metric, dim, key  string
	z, cur, mean, std float64
	n                 int64
}

// AnomalyDeviation, o an gözlenen bir sapma (UI "aktif sapmalar" listesi ve
// GET /api/v1/anomaly/active için). Yüzeye çıkan uyarıların aksine MaxSurfaced
// sınırı uygulanmaz — panelde hepsi görülür.
type AnomalyDeviation struct {
	Metric string  `json:"metric"`
	Dim    string  `json:"dim"`
	Key    string  `json:"key"`
	Scope  string  `json:"scope"` // insan-okur: "Filo geneli" / "Saha X" / "Agent #N"
	Bucket int     `json:"bucket"`
	Mean   float64 `json:"mean"`
	Std    float64 `json:"std"`
	Cur    float64 `json:"cur"`
	Z      float64 `json:"z"`
	N      int64   `json:"n"`
}

// evalAnomalyCands, mevcut kova için tüm metrik × boyut kombinasyonlarını
// değerlendirir ve z-skoru eşiğini (+ gürültü tabanını) aşan adayları döndürür.
// checkAnomaly (ateşleme, MaxSurfaced'lı) ve AnomalyActive (panel, sınırsız)
// ortak kaynağı.
func (m *Manager) evalAnomalyCands(ac AnomalyConfig) (curBucket int, cands []anomalyCand) {
	curBucket = store.SeasonalBucket(time.Now(), ac.Seasonality)
	winStart := time.Now().Add(-time.Duration(ac.WindowMin) * time.Minute)

	eval := func(metric, dim string, baseline []store.AnomalyBaselineRow, cur map[string]float64) {
		floor := ac.minAbsDelta(metric)
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
			if math.Abs(c-b.Mean) < floor {
				continue // sessiz-saat gurultu tabani
			}
			if z := (c - b.Mean) / std; math.Abs(z) >= ac.Sensitivity {
				cands = append(cands, anomalyCand{metric: metric, dim: dim, key: b.Key, z: z, cur: c, mean: b.Mean, std: std, n: b.N})
			}
		}
	}

	for _, metric := range ac.Metrics {
		dims := []string{"fleet"}
		if ac.PerSite {
			dims = append(dims, "site")
		}
		if ac.PerAgent {
			dims = append(dims, "agent")
		}
		fleetRows := m.loadBaseline("fleet", metric)
		for _, dim := range dims {
			if dim == "fleet" {
				if len(fleetRows) > 0 {
					if cur, err := m.st.AvgMetricByDim("fleet", metric, winStart); err == nil {
						eval(metric, "fleet", fleetRows, cur)
					}
					continue
				}
				// filo baseline yok → yalnız bps için hub-yerel'e düş
				if metric == "bps" {
					if rows := m.loadBaseline("local", "bps"); len(rows) > 0 {
						if v, err := m.st.AvgBpsSince(winStart); err == nil {
							eval("bps", "local", rows, map[string]float64{"": v})
						}
					}
				}
				continue
			}
			rows := m.loadBaseline(dim, metric)
			if len(rows) == 0 {
				continue
			}
			if cur, err := m.st.AvgMetricByDim(dim, metric, winStart); err == nil {
				eval(metric, dim, rows, cur)
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return math.Abs(cands[i].z) > math.Abs(cands[j].z) })
	return curBucket, cands
}

// checkAnomaly, periyodik cagirilir: adaylari buyuklukce siralar ve en cok
// MaxSurfaced tanesini atesler (bildirim seli kontrolu — panel hepsini gorur).
func (m *Manager) checkAnomaly(cfg Config) {
	ac := cfg.Anomaly.normalized()
	if !ac.Enabled {
		return
	}
	curBucket, cands := m.evalAnomalyCands(ac)
	if len(cands) == 0 {
		return
	}
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
		unit := metricUnit(c.metric)
		m.fire("anomaly", fmt.Sprintf("%s:%s:%s:%d", c.metric, c.dim, c.key, curBucket),
			fmt.Sprintf("%s: alışılmadık %s sapması (%s) — %.0f %s, bu zaman dilimi ortalaması %.0f ± %.0f %s (z=%.1f, son %d dk)",
				anomalyScope(c.dim, c.key), metricLabel(c.metric), direction, c.cur, unit, c.mean, c.std, unit, c.z, ac.WindowMin))
	}
}

// AnomalyActive, o an gözlenen tüm sapmaları (sıralı, sınırsız) döndürür.
func (m *Manager) AnomalyActive() []AnomalyDeviation {
	ac := m.Config().Anomaly.normalized()
	if !ac.Enabled {
		return nil
	}
	curBucket, cands := m.evalAnomalyCands(ac)
	out := make([]AnomalyDeviation, 0, len(cands))
	for _, c := range cands {
		out = append(out, AnomalyDeviation{
			Metric: c.metric, Dim: c.dim, Key: c.key, Scope: anomalyScope(c.dim, c.key),
			Bucket: curBucket, Mean: c.mean, Std: c.std, Cur: c.cur, Z: c.z, N: c.n,
		})
	}
	return out
}

// AnomalyBaseline, bir (dim, metric) için materyalize baseline eğrisini
// döndürür — panelin "beklenen bant" grafiği için.
func (m *Manager) AnomalyBaseline(dim, metric string) []store.AnomalyBaselineRow {
	return m.loadBaseline(dim, metric)
}

func (m *Manager) loadBaseline(dim, metric string) []store.AnomalyBaselineRow {
	rows, err := m.st.LoadAnomalyBaseline(dim, metric)
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

func metricLabel(metric string) string {
	switch metric {
	case "bps":
		return "trafik"
	case "dns_qps":
		return "DNS sorgu hızı"
	case "proc_bps":
		return "süreç trafiği"
	}
	return metric
}

func metricUnit(metric string) string {
	if metric == "dns_qps" {
		return "sorgu/sn"
	}
	return "bps"
}

// rebuildAnomalyBaseline, lider-kapili saatlik: materyalize baseline tablosunu
// tum etkin metrik × boyut kombinasyonlarinin mevsimsel alt-toplamlariyla
// gunceller.
func (m *Manager) rebuildAnomalyBaseline(cfg Config) {
	ac := cfg.Anomaly.normalized()
	var rows []store.AnomalyBaselineRow
	for _, metric := range ac.Metrics {
		dims := []string{"fleet"}
		if metric == "bps" {
			dims = append(dims, "local")
		}
		if ac.PerSite {
			dims = append(dims, "site")
		}
		if ac.PerAgent {
			dims = append(dims, "agent")
		}
		for _, dim := range dims {
			bk, err := m.st.BaselineDayBuckets(dim, metric, ac.BaselineDays, ac.Seasonality)
			if err != nil {
				slog.Debug("anomali baseline alt-toplamlari okunamadi", "dim", dim, "metric", metric, "err", err)
				continue
			}
			rows = append(rows, combineBaseline(dim, metric, bk, ac.EWMAHalfLife)...)
		}
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
