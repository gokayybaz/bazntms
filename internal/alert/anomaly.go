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
// uyumlu hale getirir (anomali alani yoksa varsayilanla acilir).
func NormalizeConfig(cfg Config) Config {
	if cfg.Anomaly.Sensitivity <= 0 {
		cfg.Anomaly = DefaultAnomalyConfig()
	} else {
		cfg.Anomaly = cfg.Anomaly.normalized()
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
// baseline'i dondurur: once filo (agent telemetrisi), sonra hub yerel
// yakalamasi (samples). fleet=true ise karsilastirma da FleetAvgBpsSince ile
// yapilmali. Iki kaynakta da yeterli veri yoksa (base=nil) motor sessiz kalir.
func (m *Manager) anomalyBaseline(curHour int, minSamples int64) (base *store.HourStat, fleet bool) {
	if fs, err := m.st.FleetHourlyBpsStats(); err == nil {
		if b := hourBucket(fs, curHour); b != nil && b.Count >= minSamples {
			return b, true
		}
	}
	if ss, err := m.st.HourlyBpsStats(); err == nil {
		if b := hourBucket(ss, curHour); b != nil && b.Count >= minSamples {
			return b, false
		}
	}
	return nil, false
}

func hourBucket(stats []store.HourStat, hour int) *store.HourStat {
	for i := range stats {
		if stats[i].Hour == hour {
			return &stats[i]
		}
	}
	return nil
}
