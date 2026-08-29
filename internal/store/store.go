package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
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

type Insight struct {
	ID            int64  `json:"id"`
	Ts            int64  `json:"ts"`
	Model         string `json:"model"`
	PeriodMinutes int    `json:"period_minutes"`
	Summary       string `json:"summary"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS samples (
	ts        INTEGER NOT NULL,
	device    TEXT    NOT NULL,
	bps_in    REAL    NOT NULL DEFAULT 0,
	bps_out   REAL    NOT NULL DEFAULT 0,
	bps_local REAL    NOT NULL DEFAULT 0,
	pps       INTEGER NOT NULL DEFAULT 0,
	dropped   INTEGER NOT NULL DEFAULT 0,
	protocols TEXT    NOT NULL DEFAULT '{}',
	PRIMARY KEY (ts, device)
);
CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts);

CREATE TABLE IF NOT EXISTS endpoint_stats (
	ts        INTEGER NOT NULL,
	device    TEXT    NOT NULL,
	ip        TEXT    NOT NULL,
	hostname  TEXT    NOT NULL DEFAULT '',
	bytes_in  INTEGER NOT NULL DEFAULT 0,
	bytes_out INTEGER NOT NULL DEFAULT 0,
	packets   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ts, device, ip)
);
CREATE INDEX IF NOT EXISTS idx_endpoint_ts ON endpoint_stats(ts);

CREATE TABLE IF NOT EXISTS connection_events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          INTEGER NOT NULL,
	proto       TEXT    NOT NULL,
	local_addr  TEXT    NOT NULL,
	remote_addr TEXT    NOT NULL DEFAULT '',
	status      TEXT    NOT NULL DEFAULT '',
	pid         INTEGER NOT NULL DEFAULT 0,
	process     TEXT    NOT NULL DEFAULT '',
	count       INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_conn_ts ON connection_events(ts);

CREATE TABLE IF NOT EXISTS dns_queries (
	ts        INTEGER NOT NULL,
	domain    TEXT    NOT NULL,
	queries   INTEGER NOT NULL DEFAULT 0,
	responses INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (ts, domain)
);
CREATE INDEX IF NOT EXISTS idx_dns_ts ON dns_queries(ts);

CREATE TABLE IF NOT EXISTS agents (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	name             TEXT    NOT NULL,
	site             TEXT    NOT NULL DEFAULT '',
	token_hash       TEXT    NOT NULL UNIQUE,
	first_seen       INTEGER NOT NULL,
	last_seen        INTEGER NOT NULL,
	version          TEXT    NOT NULL DEFAULT '',
	protocol_version INTEGER NOT NULL DEFAULT 1,
	remote_ip        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

CREATE TABLE IF NOT EXISTS agent_iface_samples (
	agent_id   INTEGER NOT NULL,
	ts         INTEGER NOT NULL,
	name       TEXT    NOT NULL,
	rx_bytes   INTEGER NOT NULL DEFAULT 0,
	tx_bytes   INTEGER NOT NULL DEFAULT 0,
	rx_packets INTEGER NOT NULL DEFAULT 0,
	tx_packets INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_agent_iface ON agent_iface_samples(agent_id, ts);

CREATE TABLE IF NOT EXISTS agent_conn_latest (
	agent_id    INTEGER NOT NULL,
	proto       TEXT    NOT NULL,
	local_addr  TEXT    NOT NULL,
	remote_addr TEXT    NOT NULL DEFAULT '',
	status      TEXT    NOT NULL DEFAULT '',
	pid         INTEGER NOT NULL DEFAULT 0,
	process     TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (agent_id, proto, local_addr, remote_addr)
);

CREATE TABLE IF NOT EXISTS process_traffic (
	ts        INTEGER NOT NULL,
	agent_id  INTEGER NOT NULL,
	pid       INTEGER NOT NULL DEFAULT 0,
	process   TEXT    NOT NULL DEFAULT '',
	proto     TEXT    NOT NULL DEFAULT '',
	remote_ip TEXT    NOT NULL DEFAULT '',
	port      INTEGER NOT NULL DEFAULT 0,
	bytes_in  INTEGER NOT NULL DEFAULT 0,
	bytes_out INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_pt_ts ON process_traffic(ts);
CREATE INDEX IF NOT EXISTS idx_pt_proc ON process_traffic(process, ts);

CREATE TABLE IF NOT EXISTS alert_events (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	ts      INTEGER NOT NULL,
	kind    TEXT    NOT NULL,
	key     TEXT    NOT NULL,
	message TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_events_ts ON alert_events(ts);

CREATE TABLE IF NOT EXISTS alert_seen (
	kind TEXT    NOT NULL,
	key  TEXT    NOT NULL,
	ts   INTEGER NOT NULL,
	PRIMARY KEY (kind, key)
);

CREATE TABLE IF NOT EXISTS alert_config (
	id  INTEGER PRIMARY KEY CHECK (id = 1),
	cfg TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS insights (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	ts             INTEGER NOT NULL,
	model          TEXT    NOT NULL,
	period_minutes INTEGER NOT NULL,
	summary        TEXT    NOT NULL
);
`)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) InsertSample(sm Sample) error {
	protoJSON, err := json.Marshal(sm.Protocols)
	if err != nil {
		protoJSON = []byte("{}")
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO samples
		(ts, device, bps_in, bps_out, bps_local, pps, dropped, protocols)
		VALUES (?,?,?,?,?,?,?,?)`,
		sm.Ts, sm.Device, sm.BpsIn, sm.BpsOut, sm.BpsLocal, sm.Pps, sm.Dropped, string(protoJSON))
	return err
}

func (s *Store) InsertEndpointDeltas(list []EndpointDelta) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO endpoint_stats
		(ts, device, ip, hostname, bytes_in, bytes_out, packets) VALUES (?,?,?,?,?,?,?)`)
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

func (s *Store) InsertDNSDeltas(list []DNSDelta) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO dns_queries
		(ts, domain, queries, responses) VALUES (?,?,?,?)`)
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

func (s *Store) InsertConnectionEvents(list []ConnectionEvent) error {
	if len(list) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO connection_events
		(ts, proto, local_addr, remote_addr, status, pid, process, count) VALUES (?,?,?,?,?,?,?,?)`)
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

func (s *Store) Prune(retention time.Duration) error {
	cutoff := time.Now().Add(-retention).Unix()
	for _, q := range []string{
		`DELETE FROM samples WHERE ts < ?`,
		`DELETE FROM endpoint_stats WHERE ts < ?`,
		`DELETE FROM connection_events WHERE ts < ?`,
		`DELETE FROM dns_queries WHERE ts < ?`,
		`DELETE FROM alert_events WHERE ts < ?`,
		`DELETE FROM agent_iface_samples WHERE ts < ?`,
		`DELETE FROM process_traffic WHERE ts < ?`,
	} {
		if _, err := s.db.Exec(q, cutoff); err != nil {
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

func (s *Store) TimeseriesBuckets(since time.Time) ([]Bucket, error) {
	rows, err := s.db.Query(`SELECT (ts/60)*60 AS bucket,
			AVG(bps_in)/8, AVG(bps_out)/8, AVG(bps_local)/8, AVG(pps)
		FROM samples WHERE ts >= ? GROUP BY bucket ORDER BY bucket`, since.Unix())
	if err != nil {
		return nil, err
	}
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

func (s *Store) PeriodTotals(since time.Time) (Totals, error) {
	var t Totals
	row := s.db.QueryRow(`SELECT COUNT(*),
			COALESCE(AVG(bps_in),0), COALESCE(AVG(bps_out),0),
			COALESCE(MAX(bps_in),0), COALESCE(MAX(bps_out),0)
		FROM samples WHERE ts >= ?`, since.Unix())
	err := row.Scan(&t.Samples, &t.AvgBpsIn, &t.AvgBpsOut, &t.PeakBpsIn, &t.PeakBpsOut)
	if err != nil {
		return t, err
	}
	if t.Samples > 0 {
		t.Seconds = t.Samples // yakalama acikken saniyede 1 ornek
	}
	return t, nil
}

func (s *Store) TopEndpointsSince(since time.Time, limit int) ([]EndpointDelta, error) {
	rows, err := s.db.Query(`SELECT ip, MAX(hostname), SUM(bytes_in), SUM(bytes_out), SUM(packets)
		FROM endpoint_stats WHERE ts >= ?
		GROUP BY ip ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT ?`, since.Unix(), limit)
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

func (s *Store) ProtocolTotals(since time.Time) (map[string]uint64, error) {
	rows, err := s.db.Query(`SELECT protocols FROM samples WHERE ts >= ?`, since.Unix())
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

func (s *Store) TopProcessesSince(since time.Time, limit int) ([]ProcessUsage, error) {
	rows, err := s.db.Query(`SELECT process, COUNT(DISTINCT local_addr || '|' || remote_addr), SUM(count)
		FROM connection_events WHERE ts >= ? AND process != ''
		GROUP BY process ORDER BY COUNT(DISTINCT local_addr || '|' || remote_addr) DESC LIMIT ?`,
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

func (s *Store) TopDomainsSince(since time.Time, limit int) ([]DNSDelta, error) {
	rows, err := s.db.Query(`SELECT domain, SUM(queries), SUM(responses)
		FROM dns_queries WHERE ts >= ?
		GROUP BY domain ORDER BY SUM(queries + responses) DESC LIMIT ?`, since.Unix(), limit)
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

// --- uyarilar ---

type AlertEvent struct {
	ID      int64  `json:"id"`
	Ts      int64  `json:"ts"`
	Kind    string `json:"kind"` // bw | port | proc | target
	Key     string `json:"key"`
	Message string `json:"message"`
}

func (s *Store) InsertAlertEvent(e AlertEvent) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO alert_events (ts, kind, key, message) VALUES (?,?,?,?)`,
		e.Ts, e.Kind, e.Key, e.Message)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RecentAlertEvents(limit int) ([]AlertEvent, error) {
	rows, err := s.db.Query(`SELECT id, ts, kind, key, message FROM alert_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.Ts, &e.Kind, &e.Key, &e.Message); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []AlertEvent{}
	}
	return out, rows.Err()
}

// IsAlertSeen, kalici gorulmusluk kontrolu (yeni surec/hedef kurallari icin).
func (s *Store) IsAlertSeen(kind, key string) (bool, error) {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM alert_seen WHERE kind = ? AND key = ?`, kind, key).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) MarkAlertSeen(kind, key string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO alert_seen (kind, key, ts) VALUES (?,?,?)`,
		kind, key, time.Now().Unix())
	return err
}

func (s *Store) CountAlertSeen(kind string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM alert_seen WHERE kind = ?`, kind).Scan(&n)
	return n, err
}

// LoadAlertConfig, tek satirlik JSON yapilandirmasini dondurur; yoksa "" doner.
func (s *Store) LoadAlertConfig() (string, error) {
	var cfg string
	err := s.db.QueryRow(`SELECT cfg FROM alert_config WHERE id = 1`).Scan(&cfg)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cfg, err
}

func (s *Store) SaveAlertConfig(cfg string) error {
	_, err := s.db.Exec(`INSERT INTO alert_config (id, cfg) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET cfg = excluded.cfg`, cfg)
	return err
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
func (s *Store) DailyTotals(days int) ([]DayTotal, error) {
	if days <= 0 {
		days = 7
	}
	_, offset := time.Now().Zone()
	rows, err := s.db.Query(`SELECT (ts + ?)/86400*86400 AS day,
			AVG(bps_in), AVG(bps_out), MAX(bps_in), MAX(bps_out), COUNT(*)
		FROM samples GROUP BY day ORDER BY day`, offset)
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

type HourAvg struct {
	Hour   int     `json:"hour"`
	BpsIn  float64 `json:"bps_in"`
	BpsOut float64 `json:"bps_out"`
}

// HourlyAverages, verilen yerel gunun saatlik ortalama serisi (24 kayit;
// veri olmayan saatler 0 ile doldurulur).
func (s *Store) HourlyAverages(dayStart time.Time) ([]HourAvg, error) {
	_, offset := time.Now().Zone()
	start := dayStart.Unix()
	rows, err := s.db.Query(`SELECT ((ts + ?) % 86400)/3600 AS hour, AVG(bps_in), AVG(bps_out)
		FROM samples WHERE ts >= ? AND ts < ?
		GROUP BY hour ORDER BY hour`, offset, start, start+86400)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HourAvg, 24)
	for i := range out {
		out[i].Hour = i
	}
	for rows.Next() {
		var h int
		var in, outBps float64
		if err := rows.Scan(&h, &in, &outBps); err != nil {
			return nil, err
		}
		if h >= 0 && h < 24 {
			out[h].BpsIn = in
			out[h].BpsOut = outBps
		}
	}
	return out, rows.Err()
}

func (s *Store) InsertInsight(i Insight) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO insights (ts, model, period_minutes, summary) VALUES (?,?,?,?)`,
		i.Ts, i.Model, i.PeriodMinutes, i.Summary)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RecentInsights(limit int) ([]Insight, error) {
	rows, err := s.db.Query(`SELECT id, ts, model, period_minutes, summary FROM insights ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Insight
	for rows.Next() {
		var i Insight
		if err := rows.Scan(&i.ID, &i.Ts, &i.Model, &i.PeriodMinutes, &i.Summary); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// Ping, veritabani baglantisini dogrular (/readyz icin).
func (s *Store) Ping() error { return s.db.Ping() }
