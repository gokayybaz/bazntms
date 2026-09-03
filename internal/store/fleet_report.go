package store

import (
	"database/sql"
	"strings"
	"time"
)

// Filo (fleet) raporlama sorgulari. Eski rapor katmani hub'in yerel paket
// yakalamasindan beslenen `samples` / `endpoint_stats` / `dns_queries`
// tablolarina baglıydi; coklu-hub kurulumunda tum hub'lar `-capture=false`
// oldugu icin bu tablolar bos kalir. Buradaki sorgular ayni raporu agent
// arayuz telemetrisi (`agent_iface_samples`), NetFlow (`flows`), agent surec
// trafigi (`process_traffic`) ve SNMP cihaz sayaclarindan (`device_iface_
// samples`) uretir.
//
// Kumulatif sayac tablolarinda (agent/cihaz arayuzleri) donemsel hiz, ardisik
// ornekler arasindaki fark alinarak hesaplanir — `AgentHistory` ile ayni
// yaklasim, ama tum filo icin ve SQL tarafinda `LAG()` pencere fonksiyonuyla.
// Sayac gerilemesi (arayuz/agent reset) → o adim 0 katki verir.

// FleetTrafficBuckets, tum agent arayuzlerinin toplam verimini `bucketSecs`
// saniyelik kovalara indirir. Bucket.In/Out bayt/sn, Pps paket/sn.
func (s *sqlStore) FleetTrafficBuckets(since time.Time, bucketSecs int) ([]Bucket, error) {
	if bucketSecs <= 0 {
		bucketSecs = 300
	}
	// dt ust siniri: agent bir sure offline kalip geri donerse tek ornekte
	// saatlerce birikmis sayac farki gelir; bunu tek kovaya yazmak kovayi
	// sisirir. 30 dk'yi asan adimlar (varsayilan telemetri araligi 30 sn →
	// 60×) offline bosluk sayilir ve elenir; kucuk kovalarda bile en az bu
	// pencere korunur.
	maxDt := int64(4 * bucketSecs)
	if maxDt < 1800 {
		maxDt = 1800
	}
	q := `SELECT (d.ts / ?) * ? AS bucket,
			COALESCE(SUM(d.rx_d), 0), COALESCE(SUM(d.tx_d), 0), COALESCE(SUM(d.pk_d), 0)
		FROM (
			SELECT ts,
				CASE WHEN rx_bytes >= LAG(rx_bytes) OVER w
					THEN rx_bytes - LAG(rx_bytes) OVER w ELSE 0 END AS rx_d,
				CASE WHEN tx_bytes >= LAG(tx_bytes) OVER w
					THEN tx_bytes - LAG(tx_bytes) OVER w ELSE 0 END AS tx_d,
				CASE WHEN (rx_packets + tx_packets) >= LAG(rx_packets + tx_packets) OVER w
					THEN (rx_packets + tx_packets) - LAG(rx_packets + tx_packets) OVER w ELSE 0 END AS pk_d,
				ts - LAG(ts) OVER w AS dt
			FROM agent_iface_samples
			WHERE ts >= ?
			WINDOW w AS (PARTITION BY agent_id, name ORDER BY ts)
		) d
		WHERE d.dt > 0 AND d.dt <= ?
			AND (d.rx_d + d.tx_d) * 8 <= d.dt * ?
		GROUP BY bucket ORDER BY bucket`
	rows, err := s.db.Query(s.q(q), bucketSecs, bucketSecs, since.Unix(), maxDt, int64(maxIfaceBps))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bucket
	div := float64(bucketSecs)
	for rows.Next() {
		var b Bucket
		var rx, tx, pk uint64
		if err := rows.Scan(&b.Ts, &rx, &tx, &pk); err != nil {
			return nil, err
		}
		b.In = float64(rx) / div
		b.Out = float64(tx) / div
		b.Pps = float64(pk) / div
		out = append(out, b)
	}
	return out, rows.Err()
}

// FleetProtocolTotals, donemdeki NetFlow kayitlarindan protokol basina toplam
// bayt (octets) dagilimi. `flows` bossa bos map doner.
func (s *sqlStore) FleetProtocolTotals(since time.Time) (map[string]uint64, error) {
	rows, err := s.db.Query(s.q(`SELECT proto, COALESCE(SUM(octets), 0)
		FROM flows WHERE ts >= ? AND proto <> '' GROUP BY proto`), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := map[string]uint64{}
	for rows.Next() {
		var proto string
		var n uint64
		if err := rows.Scan(&proto, &n); err != nil {
			return nil, err
		}
		totals[strings.ToUpper(proto)] += n
	}
	return totals, rows.Err()
}

// FleetTopEndpoints, donemdeki en yogun uzak uc noktalar. Once NetFlow
// (`flows` — hem kaynak hem hedef yonu), NetFlow yoksa agent surec
// trafigindeki `remote_ip`. EndpointDelta.BytesIn = uca gelen, BytesOut =
// uctan giden. Hostname bos kalir; Country/ASN rapor katmaninda doldurulur.
func (s *sqlStore) FleetTopEndpoints(since time.Time, limit int) ([]EndpointDelta, error) {
	if limit <= 0 || limit > 100 {
		limit = 15
	}
	rows, err := s.db.Query(s.q(`SELECT ip,
			COALESCE(SUM(in_oct), 0), COALESCE(SUM(out_oct), 0), COALESCE(SUM(pk), 0)
		FROM (
			SELECT dst AS ip, octets AS in_oct, 0 AS out_oct, packets AS pk FROM flows WHERE ts >= ?
			UNION ALL
			SELECT src AS ip, 0 AS in_oct, octets AS out_oct, packets AS pk FROM flows WHERE ts >= ?
		) t
		WHERE ip <> ''
		GROUP BY ip ORDER BY SUM(in_oct + out_oct) DESC LIMIT ?`), since.Unix(), since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	out, err := scanEndpointVolumes(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	// NetFlow yok — agent surec trafigindeki uzak IP'lere dus
	rows2, err := s.db.Query(s.q(`SELECT remote_ip,
			COALESCE(SUM(bytes_in), 0), COALESCE(SUM(bytes_out), 0), 0
		FROM process_traffic WHERE ts >= ? AND remote_ip <> ''
		GROUP BY remote_ip ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT ?`), since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	return scanEndpointVolumes(rows2)
}

func scanEndpointVolumes(rows *sql.Rows) ([]EndpointDelta, error) {
	defer rows.Close()
	var out []EndpointDelta
	for rows.Next() {
		var e EndpointDelta
		if err := rows.Scan(&e.IP, &e.BytesIn, &e.BytesOut, &e.Packets); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FleetIfaceHealth, donemdeki SNMP cihaz arayuzlerinin toplam iskarta ve hata
// sayaci artislari (ifIn/OutDiscards, ifIn/OutErrors). Kurumsal raporun
// "paket dusme" SLA satirinin filo karsiligi — cihaz yoksa 0/0 doner.
func (s *sqlStore) FleetIfaceHealth(since time.Time) (discards uint64, errs uint64, err error) {
	q := `SELECT COALESCE(SUM(disc_d), 0), COALESCE(SUM(err_d), 0)
		FROM (
			SELECT
				CASE WHEN (in_discards + out_discards) >= LAG(in_discards + out_discards) OVER w
					THEN (in_discards + out_discards) - LAG(in_discards + out_discards) OVER w ELSE 0 END AS disc_d,
				CASE WHEN (in_errors + out_errors) >= LAG(in_errors + out_errors) OVER w
					THEN (in_errors + out_errors) - LAG(in_errors + out_errors) OVER w ELSE 0 END AS err_d,
				ts - LAG(ts) OVER w AS dt
			FROM device_iface_samples
			WHERE ts >= ?
			WINDOW w AS (PARTITION BY device_id, if_index ORDER BY ts)
		) d
		WHERE d.dt > 0`
	err = s.db.QueryRow(s.q(q), since.Unix()).Scan(&discards, &errs)
	return discards, errs, err
}
