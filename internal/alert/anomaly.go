package alert

// Anomali tespiti (Faz 6.2): AI'sız erken uyarı. Saat-of-day bazlı
// istatistiksel baseline (son 7 gün) ile o anki verim karşılaştırılır;
// z-skoru eşiği aşılırsa "anomaly" uyarısı üretilir.
//
// Baseline: avg(bps_in + bps_out) ve avg((...)^2) per saat dilimi —
// std = sqrt(mean_sq - mean^2) Go tarafında hesaplanır (SQLite uyumluluğu).

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
}

func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		Enabled:     true,
		Sensitivity: 3.0,
		MinSamples:  120, // ~2 saat ornek (1/sn)
		WindowMin:   5,
	}
}

// normalize, legacy configlerde (JSON'da anomaly alani yok) sifir degerleri
// varsayilanlarla doldurur: Sensitivity 0 gecerli bir esik degildir.
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

// checkAnomaly, periyodik cagirilir: mevcut pencere verimini saatlik
// baseline ile karsilastirir. Değer kaydi maliyetini dusuk tutmak icin
// yalnizca baseline guvenilir ve std > 0 iken degerlendirir. Baseline once
// filo telemetrisinden (agent_iface_samples), yetersizse hub yerel
// yakalamasindan (samples, standalone mod) alinir.
func (m *Manager) checkAnomaly(cfg Config) {
	ac := cfg.Anomaly.normalized()
	if !ac.Enabled {
		return
	}
	curHour := time.Now().Hour() // baseline ((ts+offset)%86400)/3600 = yerel saat
	base, fleet := m.anomalyBaseline(curHour, int64(ac.MinSamples))
	if base == nil {
		slog.Debug("anomali baseline isiniyor — yeterli ornek yok", "saat", curHour, "min", ac.MinSamples)
		return
	}
	std := math.Sqrt(math.Max(0, base.MeanSq-base.Mean*base.Mean))
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
	slog.Debug("anomali baseline", "kaynak", src, "saat", curHour, "n", base.Count,
		"ort_bps", int64(base.Mean), "std_bps", int64(std), "son_bps", int64(cur), "z", math.Round(z*10)/10)
	direction := "yükseliş"
	if z < 0 {
		direction = "düşüş"
	}
	if math.Abs(z) >= ac.Sensitivity {
		m.fire("anomaly", fmt.Sprintf("bps:%d", curHour),
			fmt.Sprintf("Trafiğe alışılmadık sapma (%s): %.0f bps — saatlik ortalama %.0f ± %.0f (z=%.1f, son %d dk)",
				direction, cur, base.Mean, std, z, ac.WindowMin))
	}
}

// anomalyBaseline, mevcut saat kovasinda >= minSamples ornek iceren ilk
// baseline'i dondurur: once filo (dim="fleet"), sonra hub yerel yakalamasi
// (dim="local"). fleet=true ise karsilastirma da FleetAvgBpsSince ile
// yapilmali. Iki boyutta da yeterli veri yoksa (base=nil) motor sessiz kalir.
//
// Faz 22 S22.1: kaynak artik canli LAG taramasi degil, materyalize
// anomaly_baseline tablosu (lider-kapili saatlik rebuildAnomalyBaseline yazar).
func (m *Manager) anomalyBaseline(curHour int, minSamples int64) (base *store.HourStat, fleet bool) {
	if rows, err := m.st.LoadAnomalyBaseline("fleet", "bps"); err == nil {
		if b := baselineBucket(rows, curHour); b != nil && b.N >= minSamples {
			return baselineToHourStat(b), true
		}
	}
	if rows, err := m.st.LoadAnomalyBaseline("local", "bps"); err == nil {
		if b := baselineBucket(rows, curHour); b != nil && b.N >= minSamples {
			return baselineToHourStat(b), false
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

// baselineToHourStat, materyalize satiri checkAnomaly'nin bekledigi HourStat'a
// cevirir. MeanSq'i m2'den geri turetir: MeanSq - Mean^2 == m2/n (popülasyon
// varyansi) — checkAnomaly std'yi bu farktan hesaplar.
func baselineToHourStat(r *store.AnomalyBaselineRow) *store.HourStat {
	return &store.HourStat{
		Hour:   r.Bucket,
		Count:  r.N,
		Mean:   r.Mean,
		MeanSq: r.M2/float64(r.N) + r.Mean*r.Mean,
	}
}

// rebuildAnomalyBaseline, lider-kapili saatlik: materyalize baseline tablosunu
// filo (agent_iface_samples) ve hub yerel (samples) saat-of-day
// istatistikleriyle gunceller. checkAnomaly bu tabloyu okur — degerlendirme
// basina canli LAG taramasi (5.000 agent'ta pahali) yapilmaz.
func (m *Manager) rebuildAnomalyBaseline() {
	var rows []store.AnomalyBaselineRow
	if fs, err := m.st.FleetHourlyBpsStats(); err == nil {
		rows = append(rows, hourStatsToBaseline("fleet", "bps", fs)...)
	} else {
		slog.Debug("anomali baseline: filo istatistikleri okunamadi", "err", err)
	}
	if ls, err := m.st.HourlyBpsStats(); err == nil {
		rows = append(rows, hourStatsToBaseline("local", "bps", ls)...)
	}
	if len(rows) == 0 {
		return // taze kurulum / veri yok — tabloya dokunma
	}
	if err := m.st.SaveAnomalyBaseline(rows); err != nil {
		slog.Warn("anomali baseline yazilamadi", "err", err)
	}
}

// hourStatsToBaseline, saatlik istatistikleri (AVG(x), AVG(x^2)) materyalize
// baseline satirlarina cevirir: m2 = n * (AVG(x^2) - AVG(x)^2).
func hourStatsToBaseline(dim, metric string, stats []store.HourStat) []store.AnomalyBaselineRow {
	out := make([]store.AnomalyBaselineRow, 0, len(stats))
	for _, h := range stats {
		variance := h.MeanSq - h.Mean*h.Mean
		if variance < 0 {
			variance = 0 // kayan nokta artigi
		}
		out = append(out, store.AnomalyBaselineRow{
			Dim: dim, Metric: metric, Key: "", Bucket: h.Hour,
			N: h.Count, Mean: h.Mean, M2: float64(h.Count) * variance,
		})
	}
	return out
}
