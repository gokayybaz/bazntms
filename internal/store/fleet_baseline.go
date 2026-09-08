package store

import (
	"fmt"
	"time"
)

// Boyut (filo / saha / agent / hub-yerel) ve metrik (bps / dns_qps / proc_bps)
// bazli istatistiksel anomali baseline'i. Hub yerel paket yakalamasinin
// doldurdugu `samples` tablosu coklu-hub kurulumunda bostur → z-score anomali
// motoru sessizce olur; agent telemetrisinden uretilen boyutlar bu bosluğu
// doldurur.
//
//   - "bps"      : agent_iface_samples kumulatif rx/tx sayaci → ardisik ornek
//     farki (LAG, agent+arayuz bazli), ×8, 60 sn kovalarda boyuta
//     gore toplanir (bit/sn).
//   - "dns_qps"  : agent_dns.queries (batch-basi delta) 60 sn kovada SUM/60.
//   - "proc_bps" : process_traffic (bytes_in+bytes_out)*8, 60 sn kovada SUM/60.
//
// Sayac gerilemesi (arayuz/agent reset) o adimda 0 katki verir; telemetri
// araligini asiri asan bosluklar (offline agent) `maxGapSecs` ile elenir.
// Fiziksel olarak imkansiz artislar (`maxIfaceBps` ustu — bozuk/sanal arayuz
// sayaci) yalnizca "bps" yolunda o adimda elenir.
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

// metricSource, bir metrik icin batch-basi delta yolundaki kaynak tablo ve
// deger ifadesi (tablo alias'i `d`). lag=true → kumulatif sayac (bps;
// fleetBpsDelta yolu, table/valExpr kullanilmaz). ok=false → bilinmeyen metrik.
func metricSource(metric string) (table, valExpr string, lag, ok bool) {
	switch metric {
	case "bps":
		return "", "", true, true
	case "dns_qps":
		return "agent_dns", "d.queries", false, true
	case "proc_bps":
		return "process_traffic", "(d.bytes_in + d.bytes_out) * 8", false, true
	case "l7_qps":
		// L7 (TLS SNI / HTTP Host) gözlem hızı — endpoint-temas sıçraması
		// ≈ alışılmadık / yeni hedef aktivitesi (Faz 24-D).
		return "l7_endpoints", "d.hits", false, true
	}
	return "", "", false, false
}

// dimSQL, bir boyut icin key-select ifadesi, JOIN ve saha filtresi.
func dimSQL(dim string) (keySel, join, filter string, ok bool) {
	switch dim {
	case "fleet":
		return "''", "", "", true
	case "agent":
		return "CAST(d.agent_id AS TEXT)", "", "", true
	case "site":
		return "a.site", "JOIN agents a ON a.id = d.agent_id", "AND a.site <> ''", true
	}
	return "", "", "", false
}

// BaselineDayBuckets, son `days` günün baseline'ini (mevsimsel kova × gün-yaşı)
// alt-toplamları olarak döndürür. dim ∈ {"local","fleet","site","agent"},
// metric ∈ {"bps","dns_qps","proc_bps"}. "local" yalnız "bps" ile anlamlıdır
// (hub yerel `samples`); diğer kombinasyonlar boş döner.
func (s *sqlStore) BaselineDayBuckets(dim, metric string, days int, seasonality string) ([]BaselineDayBucket, error) {
	if days <= 0 {
		days = 21
	}
	_, offset := time.Now().Zone()
	now := time.Now().Unix()
	since := now - int64(days)*86400
	// offset/now derleme-dışı ama güvenli tamsayılar (Zone / Unix) — SQL'e
	// gömülür ki mevsimsel ifade tekrar ederken `?` sırası bozulmasın.

	if dim == "local" {
		if metric != "bps" {
			return nil, nil // hub yerel yalnız bps
		}
		l := fmt.Sprintf("(ts + %d)", offset)
		q := fmt.Sprintf(`SELECT '' AS key, %s AS bucket, (%d - ts) / 86400 AS day_age,
				COUNT(*), COALESCE(SUM(bps_in + bps_out), 0),
				COALESCE(SUM((bps_in + bps_out) * (bps_in + bps_out)), 0)
			FROM samples WHERE ts >= ?
			GROUP BY bucket, day_age`, seasonalBucketExpr(seasonality, l), now)
		return s.scanBaselineDayBuckets(s.q(q), since)
	}

	table, valExpr, lag, ok := metricSource(metric)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen metrik: %q", metric)
	}
	keySel, join, filter, ok := dimSQL(dim)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen baseline boyutu: %q", dim)
	}
	keyed := dim != "fleet"
	innerKeySel, innerGroup, outerGroup := "", "bts", "bucket, day_age"
	if keyed {
		innerKeySel = ", " + keySel + " AS k"
		innerGroup, outerGroup = "k, bts", "k, bucket, day_age"
	}
	outerKeySel := keySel
	if keyed {
		outerKeySel = "k"
	}

	var inner string
	if lag {
		inner = fmt.Sprintf(`SELECT (d.ts / %d) * %d AS bts%s, SUM(d.bit_d) * 1.0 / %d AS v
				FROM (%s) d %s
				WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d %s
				GROUP BY %s`,
			fleetBpsBucketSecs, fleetBpsBucketSecs, innerKeySel, fleetBpsBucketSecs,
			fleetBpsDelta, join, maxGapSecs, maxIfaceBps, filter, innerGroup)
	} else {
		inner = fmt.Sprintf(`SELECT (d.ts / %d) * %d AS bts%s, SUM(%s) * 1.0 / %d AS v
				FROM %s d %s
				WHERE d.ts >= ? %s
				GROUP BY %s`,
			fleetBpsBucketSecs, fleetBpsBucketSecs, innerKeySel, valExpr, fleetBpsBucketSecs,
			table, join, filter, innerGroup)
	}
	l := fmt.Sprintf("(bts + %d)", offset)
	q := fmt.Sprintf(`SELECT %s AS key, %s AS bucket, (%d - bts) / 86400 AS day_age,
			COUNT(*), COALESCE(SUM(v), 0), COALESCE(SUM(v * v), 0)
		FROM (%s) b
		GROUP BY %s`, outerKeySel, seasonalBucketExpr(seasonality, l), now, inner, outerGroup)
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

// AvgMetricByDim, current-window ortalamasini boyut × metrik bazinda dondurur
// (checkAnomaly bunu baseline ile karsilastirir). dim ∈ {"fleet","site","agent"}.
// Donen harita anahtari: fleet için "" ; site adi / agent id.
func (s *sqlStore) AvgMetricByDim(dim, metric string, since time.Time) (map[string]float64, error) {
	winSecs := time.Now().Unix() - since.Unix()
	if winSecs <= 0 {
		winSecs = 1
	}
	table, valExpr, lag, ok := metricSource(metric)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen metrik: %q", metric)
	}
	keySel, join, filter, ok := dimSQL(dim)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen boyut: %q", dim)
	}
	grp := "GROUP BY k"
	if dim == "fleet" {
		grp = ""
	}

	if lag {
		lookback := since.Unix() - 120
		q := fmt.Sprintf(`SELECT %s AS k, COALESCE(SUM(d.bit_d), 0) * 1.0 / %d
			FROM (%s) d %s
			WHERE d.dt > 0 AND d.dt <= %d AND d.bit_d <= d.dt * %d AND d.ts >= ? %s
			%s`,
			keySel, winSecs, fleetBpsDelta, join, maxGapSecs, maxIfaceBps, filter, grp)
		return s.scanKeyedFloat(s.q(q), lookback, since.Unix())
	}
	q := fmt.Sprintf(`SELECT %s AS k, COALESCE(SUM(%s), 0) * 1.0 / %d
		FROM %s d %s
		WHERE d.ts >= ? %s
		%s`,
		keySel, valExpr, winSecs, table, join, filter, grp)
	return s.scanKeyedFloat(s.q(q), since.Unix())
}

func (s *sqlStore) scanKeyedFloat(query string, args ...any) (map[string]float64, error) {
	rows, err := s.db.Query(query, args...)
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
