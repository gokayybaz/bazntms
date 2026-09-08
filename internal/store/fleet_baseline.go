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

// FleetBaselineDayBuckets, son `days` günün filo baseline'ini (mevsimsel kova ×
// gün-yaşı) alt-toplamları olarak döndürür. Anomali rebuild'i (S22.2) bunları
// EWMA gün ağırlığıyla birleştirir. Kaynak: agent arayüz telemetrisi
// (`agent_iface_samples`) — çoklu-hub'da `samples` boş.
func (s *sqlStore) FleetBaselineDayBuckets(days int, seasonality string) ([]BaselineDayBucket, error) {
	if days <= 0 {
		days = 21
	}
	_, offset := time.Now().Zone()
	now := time.Now().Unix()
	since := now - int64(days)*86400
	// offset/now derleme-dışı ama güvenli tamsayılar (Zone / Unix) — SQL'e
	// gömülür ki mevsimsel ifade `l`'yi tekrar ederken `?` sırası bozulmasın;
	// tek bağlı parametre fleetBpsDelta'nın alt sınırı (since).
	l := fmt.Sprintf("(bts + %d)", offset)
	inner := fmt.Sprintf(`SELECT (ts / %d) * %d AS bts, SUM(bit_d) * 1.0 / %d AS bps
			FROM (%s) d
			WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d
			GROUP BY bts`,
		fleetBpsBucketSecs, fleetBpsBucketSecs, fleetBpsBucketSecs, fleetBpsDelta, maxGapSecs, maxIfaceBps)
	q := fmt.Sprintf(`SELECT %s AS bucket, (%d - bts) / 86400 AS day_age, COUNT(*),
			COALESCE(SUM(bps), 0), COALESCE(SUM(bps * bps), 0)
		FROM (%s) b
		GROUP BY bucket, day_age`, seasonalBucketExpr(seasonality, l), now, inner)
	rows, err := s.db.Query(s.q(q), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaselineDayBucket{}
	for rows.Next() {
		var b BaselineDayBucket
		if err := rows.Scan(&b.Bucket, &b.DayAge, &b.N, &b.Sum, &b.SumSq); err != nil {
			return nil, err
		}
		out = append(out, b)
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
