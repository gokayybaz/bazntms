package store

// Anomali motoru baseline'ının materyalize hali (Faz 22 S22.1). Önceden
// alert.checkAnomaly her değerlendirmede agent_iface_samples üstünde LAG
// penceresi taraması yapıyordu; artık lider-kapılı saatlik bir rebuild bu
// tabloyu doldurur (SaveAnomalyBaseline) ve değerlendirme yalnızca
// LoadAnomalyBaseline ile okur. Şema: migrations/*/0009_anomaly_baseline.sql.

import (
	"math"
	"time"
)

// BaselineDayBucket, bir (mevsimsel kova, gün-yaşı) alt-toplamı. Anomali
// baseline rebuild'i (S22.2) bunları gün yaşına göre EWMA ağırlığıyla
// birleştirir: yeni günler eskilerden ağır basar (yavaş drift'e uyum).
type BaselineDayBucket struct {
	Bucket int     // mevsimsel kova — SeasonalBucket ile aynı şema
	DayAge int     // kaç gün önce (0 = son 24 saat)
	N      int64   // örnek sayısı — MinSamples geçidi için (ağırlıksız)
	Sum    float64 // Σ bps
	SumSq  float64 // Σ bps²
}

// SeasonalBucket, bir zaman damgasını mevsimsel kova numarasına eşler.
// seasonalBucketExpr (SQL) ile aynı eşlemeyi Go tarafında yapar — ikisi
// tutarlı olmalı (rebuild SQL ile yazar, checkAnomaly Go ile okur).
//
//	"hourly"  → saat-of-day            (0-23)
//	"weekday" → hafta içi/sonu × saat  (0-47; 24+ = hafta sonu)
//	"dow"     → haftanın günü × saat   (0-167; 0=Pazar)
func SeasonalBucket(t time.Time, seasonality string) int {
	h := t.Hour()
	switch seasonality {
	case "dow":
		return int(t.Weekday())*24 + h
	case "weekday":
		if wd := t.Weekday(); wd == time.Sunday || wd == time.Saturday {
			return 24 + h
		}
		return h
	default: // "hourly"
		return h
	}
}

// seasonalBucketExpr, SeasonalBucket'ın SQL karşılığı. l, yerel-saate
// kaydırılmış unix saniye veren bir alt-ifade (ör. "(ts + ?)"). Epoch günü 0
// (1970-01-01) Perşembe → haftanın günü = (gün + 4) % 7, 0=Pazar.
func seasonalBucketExpr(seasonality, l string) string {
	hour := "(" + l + " % 86400) / 3600"
	dow := "((" + l + " / 86400) + 4) % 7"
	switch seasonality {
	case "dow":
		return "(" + dow + ") * 24 + " + hour
	case "weekday":
		return hour + " + CASE WHEN (" + dow + ") IN (0, 6) THEN 24 ELSE 0 END"
	default:
		return hour
	}
}

// AnomalyBaselineRow, tek bir (boyut, metrik, anahtar, kova) baseline dilimi.
// std = sqrt(m2/n) — popülasyon varyansı (mevcut motorun AVG(x^2)-AVG(x)^2
// hesabıyla aynı). m2, S22.2'deki EWMA/artımlı birleştirme için ikinci moment.
type AnomalyBaselineRow struct {
	Dim    string  `json:"dim"`    // "fleet" | "local" | (S22.3) "site" | "agent"
	Metric string  `json:"metric"` // "bps" | (S22.4) "dns_qps" | "proc_bytes" | "conn_count"
	Key    string  `json:"key"`    // boyut anahtarı; fleet/local için ""
	Bucket int     `json:"bucket"` // saat-of-day (S22.1) / mevsimsel kova (S22.2)
	N      int64   `json:"n"`
	Mean   float64 `json:"mean"`
	M2     float64 `json:"m2"`
}

// Std, dilimin standart sapması (n < 2 → 0).
func (r AnomalyBaselineRow) Std() float64 {
	if r.N < 2 {
		return 0
	}
	return math.Sqrt(math.Max(0, r.M2/float64(r.N)))
}

// SaveAnomalyBaseline, verilen dilimleri yazar. Aynı (dim, metric) çiftinin
// eski dilimleri tek transaction içinde değiştirilir — okuyucular commit'e
// kadar eski baseline'ı görür, yani "yeniden kurulurken boş" penceresi yok.
func (s *sqlStore) SaveAnomalyBaseline(rows []AnomalyBaselineRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	cleared := map[[2]string]bool{}
	for _, r := range rows {
		k := [2]string{r.Dim, r.Metric}
		if !cleared[k] {
			if _, err := tx.Exec(s.q(`DELETE FROM anomaly_baseline WHERE dim = ? AND metric = ?`), r.Dim, r.Metric); err != nil {
				return err
			}
			cleared[k] = true
		}
	}
	now := time.Now().Unix()
	for _, r := range rows {
		if _, err := tx.Exec(s.q(`INSERT INTO anomaly_baseline
			(dim, metric, key, bucket, n, mean, m2, updated_ts) VALUES (?,?,?,?,?,?,?,?)`),
			r.Dim, r.Metric, r.Key, r.Bucket, r.N, r.Mean, r.M2, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadAnomalyBaseline, bir (dim, metric) için tüm baseline dilimlerini kova
// sırasıyla döndürür.
func (s *sqlStore) LoadAnomalyBaseline(dim, metric string) ([]AnomalyBaselineRow, error) {
	rows, err := s.db.Query(s.q(`SELECT dim, metric, key, bucket, n, mean, m2
		FROM anomaly_baseline WHERE dim = ? AND metric = ? ORDER BY bucket`), dim, metric)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AnomalyBaselineRow{}
	for rows.Next() {
		var r AnomalyBaselineRow
		if err := rows.Scan(&r.Dim, &r.Metric, &r.Key, &r.Bucket, &r.N, &r.Mean, &r.M2); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
