package alert

// Anomali tespiti (Faz 6.2): AI'sız erken uyarı. Saat-of-day bazlı
// istatistiksel baseline (son 7 gün) ile o anki verim karşılaştırılır;
// z-skoru eşiği aşılırsa "anomaly" uyarısı üretilir.
//
// Baseline: avg(bps_in + bps_out) ve avg((...)^2) per saat dilimi —
// std = sqrt(mean_sq - mean^2) Go tarafında hesaplanır (SQLite uyumluluğu).

import (
	"fmt"
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
// yalnizca baseline guvenilir ve std > 0 iken degerlendirir.
func (m *Manager) checkAnomaly(cfg Config) {
	ac := cfg.Anomaly.normalized()
	if !ac.Enabled {
		return
	}
	stats, err := m.st.HourlyBpsStats()
	if err != nil {
		return
	}
	curHour := time.Now().Hour() // baseline ((ts+offset)%86400)/3600 = yerel saat
	var base *store.HourStat
	for i := range stats {
		if stats[i].Hour == curHour {
			base = &stats[i]
			break
		}
	}
	if base == nil || base.Count < int64(ac.MinSamples) {
		return
	}
	std := math.Sqrt(math.Max(0, base.MeanSq-base.Mean*base.Mean))
	if std <= 0 {
		return
	}
	window := time.Duration(ac.WindowMin) * time.Minute
	cur, err := m.st.AvgBpsSince(time.Now().Add(-window))
	if err != nil {
		return
	}
	z := (cur - base.Mean) / std
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
