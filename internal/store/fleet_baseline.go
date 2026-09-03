package store

import (
	"fmt"
	"time"
)

// Filo (fleet) tabanli istatistiksel anomali baseline'i. Hub yerel paket
// yakalamasinin doldurdugu `samples` tablosu coklu-hub kurulumunda bostur
// (tum hub'lar -capture=false) → HourlyBpsStats/AvgBpsSince 0 satir doner ve
// z-score anomali motoru sessizce olur. Buradaki iki sorgu ayni baseline'i
// agent arayuz telemetrisinden (`agent_iface_samples`) uretir:
//
//   - kumulatif rx/tx sayaci → ardisik ornek farki (LAG, agent+arayuz bazli)
//   - fark bit'e cevrilir (×8) ve `fleetBpsBucketSecs` saniyelik kovalarda
//     tum filo icin toplanir → kova basina toplam filo verimi (bit/sn)
//
// Birim `samples`.bps_in/out ile ayni (bit/sn) — anomaly.go ikisini de ayni
// z-score esigiyle degerlendirebilir. Sayac gerilemesi (arayuz/agent reset)
// o adimda 0 katki verir; telemetri araligini asiri asan bosluklar (offline
// agent) `maxGapSecs` ile elenir. Fiziksel olarak imkansiz artislar
// (`maxIfaceBps` ustu — bozuk/sanal arayuz sayaci; canli veride 18 Tbit/sn
// bildiren "Ethernet" arayuzleri gorulmustur) baseline'i zehirlememeleri
// icin o adimda elenir.
const (
	fleetBpsBucketSecs = 60
	maxGapSecs         = 1800
	maxIfaceBps        = 5_000_000_000 // arayuz basina makul ust sinir (5 Gbit/sn)
)

// fleetBpsDelta, agent_iface_samples'tan (agent, arayuz) bazli ardisik ornek
// farkini bit (`bit_d`) ve gecen sure (`dt`) olarak veren alt sorgu.
// Tek `?` parametresi: alt sinir ts (unix).
const fleetBpsDelta = `
	SELECT ts,
		(CASE WHEN rx_bytes >= LAG(rx_bytes) OVER w
			THEN rx_bytes - LAG(rx_bytes) OVER w ELSE 0 END
		+ CASE WHEN tx_bytes >= LAG(tx_bytes) OVER w
			THEN tx_bytes - LAG(tx_bytes) OVER w ELSE 0 END) * 8 AS bit_d,
		ts - LAG(ts) OVER w AS dt
	FROM agent_iface_samples
	WHERE ts >= ?
	WINDOW w AS (PARTITION BY agent_id, name ORDER BY ts)`

// FleetHourlyBpsStats, son 7 gunun saat-of-day filo baseline istatistiklerini
// dondurur (HourlyBpsStats'in filo karsiligi, ayni HourStat semasi).
func (s *sqlStore) FleetHourlyBpsStats() ([]HourStat, error) {
	_, offset := time.Now().Zone()
	since := time.Now().Add(-7 * 24 * time.Hour).Unix()
	q := fmt.Sprintf(`SELECT ((bucket + ?) %% 86400) / 3600 AS hour, COUNT(*),
			COALESCE(AVG(bps), 0), COALESCE(AVG(bps * bps), 0)
		FROM (
			SELECT (ts / %d) * %d AS bucket, SUM(bit_d) * 1.0 / %d AS bps
			FROM (%s) d
			WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d
			GROUP BY bucket
		) b
		GROUP BY hour ORDER BY hour`,
		fleetBpsBucketSecs, fleetBpsBucketSecs, fleetBpsBucketSecs, fleetBpsDelta, maxGapSecs, maxIfaceBps)
	rows, err := s.db.Query(s.q(q), offset, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HourStat{}
	for rows.Next() {
		var h HourStat
		if err := rows.Scan(&h.Hour, &h.Count, &h.Mean, &h.MeanSq); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// FleetAvgBpsSince, verilen pencerede tum filonun ortalama verimini (bit/sn)
// dondurur (AvgBpsSince'in filo karsiligi). Pencerenin ilk ornegi icin LAG
// gerektiginden alt sinir 120 sn geriden alinir, deltalar pencereye kirpilir.
func (s *sqlStore) FleetAvgBpsSince(since time.Time) (float64, error) {
	winSecs := time.Now().Unix() - since.Unix()
	if winSecs <= 0 {
		winSecs = 1
	}
	lookback := since.Unix() - 120
	q := fmt.Sprintf(`SELECT COALESCE(SUM(bit_d), 0) * 1.0 / ?
		FROM (%s) d
		WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d AND d.ts >= ?`,
		fleetBpsDelta, maxGapSecs, maxIfaceBps)
	var avg float64
	err := s.db.QueryRow(s.q(q), winSecs, lookback, since.Unix()).Scan(&avg)
	return avg, err
}
