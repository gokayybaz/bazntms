package store

import (
	"fmt"
	"time"
)

// Filo (fleet) / saha / agent bazli istatistiksel anomali baseline'i. Hub
// yerel paket yakalamasinin doldurdugu `samples` tablosu coklu-hub
// kurulumunda bostur (tum hub'lar -capture=false) → AvgBpsSince 0 doner ve
// z-score anomali motoru sessizce olur. Buradaki sorgular ayni baseline'i
// agent arayuz telemetrisinden (`agent_iface_samples`) uretir:
//
//   - kumulatif rx/tx sayaci → ardisik ornek farki (LAG, agent+arayuz bazli)
//   - fark bit'e cevrilir (×8) ve `fleetBpsBucketSecs` saniyelik kovalarda
//     boyuta (filo / saha / agent) gore toplanir → kova basina verim (bit/sn)
//
// Birim `samples`.bps_in/out ile ayni (bit/sn). Sayac gerilemesi (arayuz/agent
// reset) o adimda 0 katki verir; telemetri araligini asiri asan bosluklar
// (offline agent) `maxGapSecs` ile elenir. Fiziksel olarak imkansiz artislar
// (`maxIfaceBps` ustu — bozuk/sanal arayuz sayaci; canli veride 18 Tbit/sn
// bildiren "Ethernet" arayuzleri gorulmustur) o adimda elenir.
const (
	fleetBpsBucketSecs = 60
	maxGapSecs         = 1800
	maxIfaceBps        = 5_000_000_000 // arayuz basina makul ust sinir (5 Gbit/sn)
)

// fleetBpsDelta, agent_iface_samples'tan (agent, arayuz) bazli ardisik ornek
// farkini bit (`bit_d`), agent_id ve gecen sure (`dt`) olarak veren alt sorgu.
// Tek `?` parametresi: alt sinir ts (unix).
const fleetBpsDelta = `
	SELECT ts, agent_id,
		(CASE WHEN rx_bytes >= LAG(rx_bytes) OVER w
			THEN rx_bytes - LAG(rx_bytes) OVER w ELSE 0 END
		+ CASE WHEN tx_bytes >= LAG(tx_bytes) OVER w
			THEN tx_bytes - LAG(tx_bytes) OVER w ELSE 0 END) * 8 AS bit_d,
		ts - LAG(ts) OVER w AS dt
	FROM agent_iface_samples
	WHERE ts >= ?
	WINDOW w AS (PARTITION BY agent_id, name ORDER BY ts)`

// BaselineDayBuckets, son `days` günün baseline'ini (mevsimsel kova × gün-yaşı)
// alt-toplamları olarak döndürür. dim:
//
//	"local" → hub yerel yakalaması (`samples`), key = ""
//	"fleet" → tüm agent arayüzleri toplamı, key = ""
//	"site"  → agent sahasına göre (agents.site, boş sahalar hariç), key = site
//	"agent" → agent'a göre, key = agent id
//
// Anomali rebuild'i (S22.2/S22.3) bunları EWMA gün ağırlığıyla birleştirir.
func (s *sqlStore) BaselineDayBuckets(dim string, days int, seasonality string) ([]BaselineDayBucket, error) {
	if days <= 0 {
		days = 21
	}
	_, offset := time.Now().Zone()
	now := time.Now().Unix()
	since := now - int64(days)*86400
	// offset/now derleme-dışı ama güvenli tamsayılar (Zone / Unix) — SQL'e
	// gömülür ki mevsimsel ifade tekrar ederken `?` sırası bozulmasın; tek
	// bağlı parametre alt sınır (since).

	if dim == "local" {
		l := fmt.Sprintf("(ts + %d)", offset)
		q := fmt.Sprintf(`SELECT '' AS key, %s AS bucket, (%d - ts) / 86400 AS day_age,
				COUNT(*), COALESCE(SUM(bps_in + bps_out), 0),
				COALESCE(SUM((bps_in + bps_out) * (bps_in + bps_out)), 0)
			FROM samples WHERE ts >= ?
			GROUP BY bucket, day_age`, seasonalBucketExpr(seasonality, l), now)
		return s.scanBaselineDayBuckets(s.q(q), since)
	}

	var keySel, innerKeySel, innerGroup, join, filter, outerGroup string
	switch dim {
	case "fleet":
		keySel, innerGroup, outerGroup = "''", "bts", "bucket, day_age"
	case "agent":
		keySel = "k"
		innerKeySel = ", CAST(d.agent_id AS TEXT) AS k"
		innerGroup, outerGroup = "k, bts", "k, bucket, day_age"
	case "site":
		keySel = "k"
		innerKeySel = ", a.site AS k"
		join = "JOIN agents a ON a.id = d.agent_id"
		filter = "AND a.site <> ''"
		innerGroup, outerGroup = "k, bts", "k, bucket, day_age"
	default:
		return nil, fmt.Errorf("bilinmeyen baseline boyutu: %q", dim)
	}

	inner := fmt.Sprintf(`SELECT (d.ts / %d) * %d AS bts%s, SUM(d.bit_d) * 1.0 / %d AS bps
			FROM (%s) d %s
			WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d %s
			GROUP BY %s`,
		fleetBpsBucketSecs, fleetBpsBucketSecs, innerKeySel, fleetBpsBucketSecs,
		fleetBpsDelta, join, maxGapSecs, maxIfaceBps, filter, innerGroup)
	l := fmt.Sprintf("(bts + %d)", offset)
	q := fmt.Sprintf(`SELECT %s AS key, %s AS bucket, (%d - bts) / 86400 AS day_age,
			COUNT(*), COALESCE(SUM(bps), 0), COALESCE(SUM(bps * bps), 0)
		FROM (%s) b
		GROUP BY %s`, keySel, seasonalBucketExpr(seasonality, l), now, inner, outerGroup)
	return s.scanBaselineDayBuckets(s.q(q), since)
}

func (s *sqlStore) scanBaselineDayBuckets(query string, args ...any) ([]BaselineDayBucket, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaselineDayBucket{}
	for rows.Next() {
		var b BaselineDayBucket
		if err := rows.Scan(&b.Key, &b.Bucket, &b.DayAge, &b.N, &b.Sum, &b.SumSq); err != nil {
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
	q := fmt.Sprintf(`SELECT COALESCE(SUM(bit_d), 0) * 1.0 / %d
		FROM (%s) d
		WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d AND d.ts >= ?`,
		winSecs, fleetBpsDelta, maxGapSecs, maxIfaceBps)
	var avg float64
	err := s.db.QueryRow(s.q(q), lookback, since.Unix()).Scan(&avg)
	return avg, err
}

// AvgBpsByDim, verilen pencerede saha ("site") veya agent ("agent") bazinda
// ortalama verimi (bit/sn) dondurur — checkAnomaly bunu boyut baseline'iyla
// karsilastirir. FleetAvgBpsSince ile ayni delta/kirpma mantigi.
func (s *sqlStore) AvgBpsByDim(dim string, since time.Time) (map[string]float64, error) {
	winSecs := time.Now().Unix() - since.Unix()
	if winSecs <= 0 {
		winSecs = 1
	}
	lookback := since.Unix() - 120
	var keySel, join, filter string
	switch dim {
	case "agent":
		keySel = "CAST(d.agent_id AS TEXT)"
	case "site":
		keySel = "a.site"
		join = "JOIN agents a ON a.id = d.agent_id"
		filter = "AND a.site <> ''"
	default:
		return nil, fmt.Errorf("bilinmeyen boyut: %q", dim)
	}
	q := fmt.Sprintf(`SELECT %s AS k, COALESCE(SUM(d.bit_d), 0) * 1.0 / %d
		FROM (%s) d %s
		WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d AND d.ts >= ? %s
		GROUP BY k`,
		keySel, winSecs, fleetBpsDelta, join, maxGapSecs, maxIfaceBps, filter)
	rows, err := s.db.Query(s.q(q), lookback, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
