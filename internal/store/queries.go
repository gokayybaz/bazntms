package store

// Yakalama verisi (collector yazimlari) ve dashboard sorgulari — hub yerel
// samples/endpoint_stats/dns_queries/connection_events tablolari. Coklu-hub
// kurulumunda bu tablolar bos kalir (rapor kaynagi: fleet_report.go).

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Sample struct {
	Ts        int64             `json:"ts"`
	Device    string            `json:"device"`
	BpsIn     float64           `json:"bps_in"`
	BpsOut    float64           `json:"bps_out"`
	BpsLocal  float64           `json:"bps_local"`
	Pps       uint64            `json:"pps"`
	Dropped   uint64            `json:"dropped"`
	Protocols map[string]uint64 `json:"protocols"`
}

type EndpointDelta struct {
	Ts       int64  `json:"ts"`
	Device   string `json:"device"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Country  string `json:"country,omitempty"` // sorgu aninda zenginlestirilir
	ASN      string `json:"asn,omitempty"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	Packets  uint64 `json:"packets"`
}

type ConnectionEvent struct {
	Ts         int64  `json:"ts"`
	Proto      string `json:"proto"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
	Status     string `json:"status"`
	PID        int32  `json:"pid"`
	Process    string `json:"process"`
	Count      uint64 `json:"count"`
}

type DNSDelta struct {
	Ts        int64  `json:"ts"`
	Domain    string `json:"domain"`
	Queries   uint64 `json:"queries"`
	Responses uint64 `json:"responses"`
}

func (s *sqlStore) InsertSample(sm Sample) error {
	protoJSON, err := json.Marshal(sm.Protocols)
	if err != nil {
		protoJSON = []byte("{}")
	}
	_, err = s.db.Exec(s.q(`INSERT INTO samples
		(ts, device, bps_in, bps_out, bps_local, pps, dropped, protocols)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT (ts, device) DO UPDATE SET
			bps_in = excluded.bps_in, bps_out = excluded.bps_out, bps_local = excluded.bps_local,
			pps = excluded.pps, dropped = excluded.dropped, protocols = excluded.protocols`),
		sm.Ts, sm.Device, sm.BpsIn, sm.BpsOut, sm.BpsLocal, sm.Pps, sm.Dropped, string(protoJSON))
	return err
}

func (s *sqlStore) InsertEndpointDeltas(list []EndpointDelta) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO endpoint_stats
		(ts, device, ip, hostname, bytes_in, bytes_out, packets) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT (ts, device, ip) DO UPDATE SET
			hostname = excluded.hostname, bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out, packets = excluded.packets`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range list {
		if _, err := stmt.Exec(e.Ts, e.Device, e.IP, e.Hostname, e.BytesIn, e.BytesOut, e.Packets); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) InsertDNSDeltas(list []DNSDelta) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO dns_queries
		(ts, domain, queries, responses) VALUES (?,?,?,?)
		ON CONFLICT (ts, domain) DO UPDATE SET
			queries = excluded.queries, responses = excluded.responses`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, d := range list {
		if _, err := stmt.Exec(d.Ts, d.Domain, d.Queries, d.Responses); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqlStore) InsertConnectionEvents(list []ConnectionEvent) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(s.q(`INSERT INTO connection_events
		(ts, proto, local_addr, remote_addr, status, pid, process, count) VALUES (?,?,?,?,?,?,?,?)`))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range list {
		if _, err := stmt.Exec(c.Ts, c.Proto, c.LocalAddr, c.RemoteAddr, c.Status, c.PID, c.Process, c.Count); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Prune, retention penceresinin disindaki ham kayitlari siler. TimescaleDB
// modunda hypertable retention politikalarinin yedegi olarak da calisir.
func (s *sqlStore) Prune(retention time.Duration) error {
	cutoff := time.Now().Add(-retention).Unix()
	for _, q := range []string{
		`DELETE FROM samples WHERE ts < ?`,
		`DELETE FROM endpoint_stats WHERE ts < ?`,
		`DELETE FROM connection_events WHERE ts < ?`,
		`DELETE FROM dns_queries WHERE ts < ?`,
		`DELETE FROM alert_events WHERE ts < ?`,
		`DELETE FROM agent_iface_samples WHERE ts < ?`,
		`DELETE FROM process_traffic WHERE ts < ?`,
		`DELETE FROM l7_endpoints WHERE ts < ?`,
		`DELETE FROM agent_dns WHERE ts < ?`,
		`DELETE FROM flows WHERE ts < ?`,
		`DELETE FROM device_iface_samples WHERE ts < ?`, // Faz 4.3: eksik olan iki tablo
		`DELETE FROM syslog_events WHERE ts < ?`,
		`DELETE FROM device_resources WHERE ts < ?`, // Faz 8
		`DELETE FROM fortigate_sdwan WHERE ts < ?`,
		`DELETE FROM fortigate_policy_hits WHERE ts < ?`,
		`DELETE FROM fortigate_vpn_status WHERE ts < ?`, // upsert tablosu: bayat satırlar (tünel silinmişse) temizlenir
	} {
		if _, err := s.db.Exec(s.q(q), cutoff); err != nil {
			return err
		}
	}
	return nil
}

// --- sorgular ---

type Bucket struct {
	Ts    int64   `json:"ts"`
	In    float64 `json:"in"` // bayt/sn
	Out   float64 `json:"out"`
	Local float64 `json:"local"`
	Pps   float64 `json:"pps"`
}

// TimeseriesBuckets, 60 saniyelik kovalara donusturulmis trafik serisi.
// TimescaleDB modunda 1 dakikalik continuous aggregate (samples_1m) uzerinden
// okunur (Faz 4.3 downsample: ham 7g, 1dk 90g, 1sa 2y saklanir).
func (s *sqlStore) TimeseriesBuckets(since time.Time) ([]Bucket, error) {
	if s.ts {
		rows, err := s.db.Query(s.q(`SELECT bucket,
				AVG(avg_bps_in)/8, AVG(avg_bps_out)/8, AVG(avg_bps_local)/8, SUM(pps)
			FROM samples_1m WHERE bucket >= ? GROUP BY bucket ORDER BY bucket`), since.Unix())
		if err == nil {
			out, scanErr := scanBuckets(rows)
			if scanErr == nil {
				return out, nil
			}
			// cagg uzerinden okuma basarisizsa ham tabloya duser (asagida)
		}
	}
	rows, err := s.db.Query(s.q(`SELECT (ts/60)*60 AS bucket,
			AVG(bps_in)/8, AVG(bps_out)/8, AVG(bps_local)/8, AVG(pps)
		FROM samples WHERE ts >= ? GROUP BY bucket ORDER BY bucket`), since.Unix())
	if err != nil {
		return nil, err
	}
	return scanBuckets(rows)
}

func scanBuckets(rows *sql.Rows) ([]Bucket, error) {
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Ts, &b.In, &b.Out, &b.Local, &b.Pps); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type Totals struct {
	AvgBpsIn   float64 `json:"avg_bps_in"`
	AvgBpsOut  float64 `json:"avg_bps_out"`
	PeakBpsIn  float64 `json:"peak_bps_in"`
	PeakBpsOut float64 `json:"peak_bps_out"`
	Seconds    int64   `json:"seconds"`
	Samples    int64   `json:"samples"`
}

func (s *sqlStore) PeriodTotals(since time.Time) (Totals, error) {
	var t Totals
	row := s.db.QueryRow(s.q(`SELECT COUNT(*),
			COALESCE(AVG(bps_in),0), COALESCE(AVG(bps_out),0),
			COALESCE(MAX(bps_in),0), COALESCE(MAX(bps_out),0)
		FROM samples WHERE ts >= ?`), since.Unix())
	err := row.Scan(&t.Samples, &t.AvgBpsIn, &t.AvgBpsOut, &t.PeakBpsIn, &t.PeakBpsOut)
	if err != nil {
		return t, err
	}
	if t.Samples > 0 {
		t.Seconds = t.Samples // yakalama acikken saniyede 1 ornek
	}
	return t, nil
}

func (s *sqlStore) TopEndpointsSince(since time.Time, limit int) ([]EndpointDelta, error) {
	rows, err := s.db.Query(s.q(`SELECT ip, MAX(hostname), SUM(bytes_in), SUM(bytes_out), SUM(packets)
		FROM endpoint_stats WHERE ts >= ?
		GROUP BY ip ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT ?`), since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EndpointDelta
	for rows.Next() {
		var e EndpointDelta
		if err := rows.Scan(&e.IP, &e.Hostname, &e.BytesIn, &e.BytesOut, &e.Packets); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *sqlStore) ProtocolTotals(since time.Time) (map[string]uint64, error) {
	rows, err := s.db.Query(s.q(`SELECT protocols FROM samples WHERE ts >= ?`), since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	totals := map[string]uint64{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var m map[string]uint64
		if json.Unmarshal([]byte(raw), &m) == nil {
			for k, v := range m {
				totals[k] += v
			}
		}
	}
	return totals, rows.Err()
}

type ProcessUsage struct {
	Process     string `json:"process"`
	Connections int64  `json:"connections"`
	Events      int64  `json:"events"`
}

func (s *sqlStore) TopProcessesSince(since time.Time, limit int) ([]ProcessUsage, error) {
	rows, err := s.db.Query(s.q(`SELECT process, COUNT(DISTINCT local_addr || '|' || remote_addr), SUM(count)
		FROM connection_events WHERE ts >= ? AND process != ''
		GROUP BY process ORDER BY COUNT(DISTINCT local_addr || '|' || remote_addr) DESC LIMIT ?`),
		since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProcessUsage
	for rows.Next() {
		var p ProcessUsage
		if err := rows.Scan(&p.Process, &p.Connections, &p.Events); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *sqlStore) TopDomainsSince(since time.Time, limit int) ([]DNSDelta, error) {
	rows, err := s.db.Query(s.q(`SELECT domain, SUM(queries), SUM(responses)
		FROM dns_queries WHERE ts >= ?
		GROUP BY domain ORDER BY SUM(queries + responses) DESC LIMIT ?`), since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DNSDelta
	for rows.Next() {
		var d DNSDelta
		if err := rows.Scan(&d.Domain, &d.Queries, &d.Responses); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// --- karsilastirma sorgulari ---

type DayTotal struct {
	Day        int64   `json:"day"` // yerel gece yarisi (unix)
	AvgBpsIn   float64 `json:"avg_bps_in"`
	AvgBpsOut  float64 `json:"avg_bps_out"`
	PeakBpsIn  float64 `json:"peak_bps_in"`
	PeakBpsOut float64 `json:"peak_bps_out"`
	Samples    int64   `json:"samples"`
}

// DailyTotals, gun bazli ozetler; gun sinirlari yerel gece yarisina hizalanir.
func (s *sqlStore) DailyTotals(days int) ([]DayTotal, error) {
	if days <= 0 {
		days = 7
	}
	_, offset := time.Now().Zone()
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	rows, err := s.db.Query(s.q(`SELECT (ts + ?)/86400*86400 AS day,
			AVG(bps_in), AVG(bps_out), MAX(bps_in), MAX(bps_out), COUNT(*)
		FROM samples WHERE ts >= ? GROUP BY day ORDER BY day`), offset, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayTotal
	for rows.Next() {
		var d DayTotal
		var bucket int64
		if err := rows.Scan(&bucket, &d.AvgBpsIn, &d.AvgBpsOut, &d.PeakBpsIn, &d.PeakBpsOut, &d.Samples); err != nil {
			return nil, err
		}
		d.Day = bucket - int64(offset)
		out = append(out, d)
	}
	if out == nil {
		out = []DayTotal{}
	}
	return out, rows.Err()
}
