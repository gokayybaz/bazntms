package store

// PostgreSQL/TimescaleDB arka ucu (Faz 4.1 + 4.3).
//
// Semanin SQLite surumunden farklari:
//   - id kolonlari BIGSERIAL (SQLite AUTOINCREMENT karsiligi); hypertable'a
//     cevrilecek tablolarda birlesik PK (id, ts) zorunludur
//   - tamsayi sayclar BIGINT, gercek sayilar DOUBLE PRECISION
//   - zaman kolonlari yine unix saniye (BIGINT) — TimescaleDB integer-tabanli
//     hypertable destekler (chunk_time_interval ve time_bucket tamsayi)
//
// TimescaleDB kuruluysa (best-effort):
//   - agir zaman serisi tablolari hypertable'a cevrilir
//   - samples uzerine 1dk (samples_1m) ve 1sa (samples_1h) continuous
//     aggregate'lari kurulur
//   - retention politikalari: ham 7g → 1dk 90g → 1sa 2y
//   - TimescaleDB yoksa duz PostgreSQL calisir; temizlik Prune ile yapilir

import (
	"database/sql"
	"log/slog"
	"strconv"
)

func migratePostgres(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS samples (
	ts        BIGINT NOT NULL,
	device    TEXT   NOT NULL,
	bps_in    DOUBLE PRECISION NOT NULL DEFAULT 0,
	bps_out   DOUBLE PRECISION NOT NULL DEFAULT 0,
	bps_local DOUBLE PRECISION NOT NULL DEFAULT 0,
	pps       BIGINT NOT NULL DEFAULT 0,
	dropped   BIGINT NOT NULL DEFAULT 0,
	protocols TEXT   NOT NULL DEFAULT '{}',
	PRIMARY KEY (ts, device)
);
CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts);

CREATE TABLE IF NOT EXISTS endpoint_stats (
	ts        BIGINT NOT NULL,
	device    TEXT   NOT NULL,
	ip        TEXT   NOT NULL,
	hostname  TEXT   NOT NULL DEFAULT '',
	bytes_in  BIGINT NOT NULL DEFAULT 0,
	bytes_out BIGINT NOT NULL DEFAULT 0,
	packets   BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (ts, device, ip)
);
CREATE INDEX IF NOT EXISTS idx_endpoint_ts ON endpoint_stats(ts);

CREATE TABLE IF NOT EXISTS connection_events (
	id          BIGSERIAL NOT NULL,
	ts          BIGINT    NOT NULL,
	proto       TEXT      NOT NULL,
	local_addr  TEXT      NOT NULL,
	remote_addr TEXT      NOT NULL DEFAULT '',
	status      TEXT      NOT NULL DEFAULT '',
	pid         INTEGER   NOT NULL DEFAULT 0,
	process     TEXT      NOT NULL DEFAULT '',
	count       BIGINT    NOT NULL DEFAULT 1,
	PRIMARY KEY (id, ts)
);
CREATE INDEX IF NOT EXISTS idx_conn_ts ON connection_events(ts);

CREATE TABLE IF NOT EXISTS dns_queries (
	ts        BIGINT NOT NULL,
	domain    TEXT   NOT NULL,
	queries   BIGINT NOT NULL DEFAULT 0,
	responses BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (ts, domain)
);
CREATE INDEX IF NOT EXISTS idx_dns_ts ON dns_queries(ts);

CREATE TABLE IF NOT EXISTS agents (
	id               BIGSERIAL PRIMARY KEY,
	name             TEXT    NOT NULL,
	site             TEXT    NOT NULL DEFAULT '',
	token_hash       TEXT    NOT NULL UNIQUE,
	first_seen       BIGINT  NOT NULL,
	last_seen        BIGINT  NOT NULL,
	version          TEXT    NOT NULL DEFAULT '',
	protocol_version INTEGER NOT NULL DEFAULT 1,
	remote_ip        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen);

CREATE TABLE IF NOT EXISTS agent_iface_samples (
	agent_id   BIGINT NOT NULL,
	ts         BIGINT NOT NULL,
	name       TEXT   NOT NULL,
	rx_bytes   BIGINT NOT NULL DEFAULT 0,
	tx_bytes   BIGINT NOT NULL DEFAULT 0,
	rx_packets BIGINT NOT NULL DEFAULT 0,
	tx_packets BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_agent_iface ON agent_iface_samples(agent_id, ts);

CREATE TABLE IF NOT EXISTS agent_conn_latest (
	agent_id    BIGINT NOT NULL,
	proto       TEXT   NOT NULL,
	local_addr  TEXT   NOT NULL,
	remote_addr TEXT   NOT NULL DEFAULT '',
	status      TEXT   NOT NULL DEFAULT '',
	pid         INTEGER NOT NULL DEFAULT 0,
	process     TEXT   NOT NULL DEFAULT '',
	PRIMARY KEY (agent_id, proto, local_addr, remote_addr)
);

CREATE TABLE IF NOT EXISTS process_traffic (
	ts        BIGINT NOT NULL,
	agent_id  BIGINT NOT NULL,
	pid       INTEGER NOT NULL DEFAULT 0,
	process   TEXT   NOT NULL DEFAULT '',
	proto     TEXT   NOT NULL DEFAULT '',
	remote_ip TEXT   NOT NULL DEFAULT '',
	port      INTEGER NOT NULL DEFAULT 0,
	bytes_in  BIGINT NOT NULL DEFAULT 0,
	bytes_out BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_pt_ts ON process_traffic(ts);
CREATE INDEX IF NOT EXISTS idx_pt_proc ON process_traffic(process, ts);

CREATE TABLE IF NOT EXISTS devices (
	id           BIGSERIAL PRIMARY KEY,
	name         TEXT    NOT NULL,
	host         TEXT    NOT NULL,
	kind         TEXT    NOT NULL DEFAULT 'other',
	vendor       TEXT    NOT NULL DEFAULT 'snmp',
	snmp_version INTEGER NOT NULL DEFAULT 2,
	community    TEXT    NOT NULL DEFAULT '',
	v3_user      TEXT    NOT NULL DEFAULT '',
	v3_auth_proto TEXT   NOT NULL DEFAULT '',
	v3_auth_pass TEXT    NOT NULL DEFAULT '',
	v3_priv_proto TEXT   NOT NULL DEFAULT '',
	v3_priv_pass TEXT    NOT NULL DEFAULT '',
	api_url      TEXT    NOT NULL DEFAULT '',
	api_token_enc TEXT   NOT NULL DEFAULT '',
	api_verify_tls INTEGER NOT NULL DEFAULT 1,
	vdom         TEXT    NOT NULL DEFAULT '',
	poll_seconds INTEGER NOT NULL DEFAULT 60,
	enabled      INTEGER NOT NULL DEFAULT 1,
	sys_name     TEXT    NOT NULL DEFAULT '',
	sys_descr    TEXT    NOT NULL DEFAULT '',
	added_at     BIGINT  NOT NULL,
	last_poll    BIGINT  NOT NULL DEFAULT 0,
	last_error   TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS device_resources (
	ts        BIGINT NOT NULL,
	device_id BIGINT NOT NULL,
	cpu_pct   DOUBLE PRECISION NOT NULL DEFAULT 0,
	mem_pct   DOUBLE PRECISION NOT NULL DEFAULT 0,
	disk_pct  DOUBLE PRECISION NOT NULL DEFAULT 0,
	sessions  BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dev_res ON device_resources(device_id, ts);

CREATE TABLE IF NOT EXISTS fortigate_vpn_status (
	device_id BIGINT NOT NULL,
	vdom      TEXT   NOT NULL DEFAULT '',
	kind      TEXT   NOT NULL,
	name      TEXT   NOT NULL,
	peer      TEXT   NOT NULL DEFAULT '',
	status    TEXT   NOT NULL DEFAULT '',
	uptime    BIGINT NOT NULL DEFAULT 0,
	rx_bytes  BIGINT NOT NULL DEFAULT 0,
	tx_bytes  BIGINT NOT NULL DEFAULT 0,
	ts        BIGINT NOT NULL,
	PRIMARY KEY (device_id, vdom, kind, name)
);

CREATE TABLE IF NOT EXISTS fortigate_sdwan (
	ts       BIGINT NOT NULL,
	device_id BIGINT NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	member    TEXT    NOT NULL,
	health_check TEXT NOT NULL DEFAULT '',
	latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
	jitter_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
	packet_loss_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
	state      TEXT   NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_forti_sdwan ON fortigate_sdwan(device_id, ts);

CREATE TABLE IF NOT EXISTS fortigate_policy_hits (
	ts        BIGINT NOT NULL,
	device_id BIGINT NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	policy_id BIGINT  NOT NULL,
	name      TEXT    NOT NULL DEFAULT '',
	action    TEXT    NOT NULL DEFAULT '',
	hits      BIGINT  NOT NULL DEFAULT 0,
	bytes     BIGINT  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_forti_policy ON fortigate_policy_hits(device_id, ts);

CREATE TABLE IF NOT EXISTS device_iface_samples (
	device_id   BIGINT NOT NULL,
	ts          BIGINT NOT NULL,
	if_index    INTEGER NOT NULL,
	name        TEXT   NOT NULL DEFAULT '',
	alias       TEXT   NOT NULL DEFAULT '',
	speed       BIGINT NOT NULL DEFAULT 0,
	oper_status INTEGER NOT NULL DEFAULT 0,
	rx_bytes    BIGINT NOT NULL DEFAULT 0,
	tx_bytes    BIGINT NOT NULL DEFAULT 0,
	in_errors   BIGINT NOT NULL DEFAULT 0,
	out_errors  BIGINT NOT NULL DEFAULT 0,
	in_discards BIGINT NOT NULL DEFAULT 0,
	out_discards BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dev_iface ON device_iface_samples(device_id, ts);

CREATE TABLE IF NOT EXISTS flows (
	ts       BIGINT NOT NULL,
	device   TEXT   NOT NULL DEFAULT '',
	src      TEXT   NOT NULL DEFAULT '',
	dst      TEXT   NOT NULL DEFAULT '',
	src_port INTEGER NOT NULL DEFAULT 0,
	dst_port INTEGER NOT NULL DEFAULT 0,
	proto    TEXT   NOT NULL DEFAULT '',
	packets  BIGINT NOT NULL DEFAULT 0,
	octets   BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_flows_ts ON flows(ts);

CREATE TABLE IF NOT EXISTS syslog_events (
	id       BIGSERIAL NOT NULL,
	ts       BIGINT    NOT NULL,
	host     TEXT      NOT NULL DEFAULT '',
	severity INTEGER   NOT NULL DEFAULT 7,
	tag      TEXT      NOT NULL DEFAULT '',
	message  TEXT      NOT NULL DEFAULT '',
	PRIMARY KEY (id, ts)
);
CREATE INDEX IF NOT EXISTS idx_syslog_ts ON syslog_events(ts);

CREATE TABLE IF NOT EXISTS alert_events (
	id      BIGSERIAL PRIMARY KEY,
	ts      BIGINT NOT NULL,
	kind    TEXT   NOT NULL,
	key     TEXT   NOT NULL,
	message TEXT   NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_events_ts ON alert_events(ts);

CREATE TABLE IF NOT EXISTS alert_seen (
	kind TEXT   NOT NULL,
	key  TEXT   NOT NULL,
	ts   BIGINT NOT NULL,
	PRIMARY KEY (kind, key)
);

CREATE TABLE IF NOT EXISTS alert_config (
	id  INTEGER PRIMARY KEY CHECK (id = 1),
	cfg TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS insights (
	id             BIGSERIAL PRIMARY KEY,
	ts             BIGINT  NOT NULL,
	model          TEXT    NOT NULL,
	period_minutes INTEGER NOT NULL,
	summary        TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	id            BIGSERIAL PRIMARY KEY,
	username      TEXT    NOT NULL UNIQUE,
	password_hash TEXT    NOT NULL DEFAULT '',
	role          TEXT    NOT NULL DEFAULT 'viewer',
	site          TEXT    NOT NULL DEFAULT '',
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_at    BIGINT  NOT NULL,
	last_login    BIGINT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS api_tokens (
	id         BIGSERIAL PRIMARY KEY,
	name       TEXT    NOT NULL,
	token_hash TEXT    NOT NULL UNIQUE,
	role       TEXT    NOT NULL DEFAULT 'viewer',
	site       TEXT    NOT NULL DEFAULT '',
	created_at BIGINT  NOT NULL,
	last_used  BIGINT  NOT NULL DEFAULT 0,
	revoked    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_events (
	id        BIGSERIAL PRIMARY KEY,
	ts        BIGINT  NOT NULL,
	username  TEXT    NOT NULL,
	role      TEXT    NOT NULL DEFAULT '',
	action    TEXT    NOT NULL,
	target    TEXT    NOT NULL DEFAULT '',
	detail    TEXT    NOT NULL DEFAULT '',
	ip        TEXT    NOT NULL DEFAULT '',
	prev_hash TEXT    NOT NULL DEFAULT '',
	hash      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_events(ts);

CREATE TABLE IF NOT EXISTS topology_links (
	id          BIGSERIAL PRIMARY KEY,
	ts          BIGINT  NOT NULL,
	kind        TEXT    NOT NULL,
	source_type TEXT    NOT NULL,
	source_id   BIGINT  NOT NULL DEFAULT 0,
	source_name TEXT    NOT NULL DEFAULT '',
	local_port  TEXT    NOT NULL DEFAULT '',
	peer_type   TEXT    NOT NULL DEFAULT 'host',
	peer_id     BIGINT  NOT NULL DEFAULT 0,
	peer_name   TEXT    NOT NULL DEFAULT '',
	peer_ip     TEXT    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_topo_dedup ON topology_links(kind, source_type, source_id, local_port, peer_name, peer_ip);
`)
	return err
}

// setupTimescale, TimescaleDB eklentisi varsa olcek altyapisini kurar:
// hypertable'lar, continuous aggregate'lar ve retention politikaları.
// Her adim best-effort'tur; TimescaleDB surumu bir ozellik desteklemiyorsa
// loglanir ve duz PostgreSQL moduyla devam edilir. Donduren deger eklentinin
// aktif olup olmadigidir.
func setupTimescale(db *sql.DB) bool {
	_, _ = db.Exec(`CREATE EXTENSION IF NOT EXISTS timescaledb`) // superuser gerekebilir
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pg_extension WHERE extname = 'timescaledb'`).Scan(&n); err != nil || n == 0 {
		slog.Info("timescaledb yok — duz PostgreSQL modunda calisiyor (temizlik Prune ile)")
		return false
	}

	// integer-tabanli hypertable politikalarinin "simdi" fonksiyonu
	tsTry(db, `CREATE OR REPLACE FUNCTION bazntms_ts_now() RETURNS BIGINT
		LANGUAGE SQL STABLE AS $f$ SELECT EXTRACT(EPOCH FROM clock_timestamp())::BIGINT $f$`,
		"now fonksiyonu")

	// ham zaman serisi tablolari → hypertable
	// chunk_time_interval saniye birimindedir (integer hypertable)
	for _, h := range []struct {
		table string
		chunk int64
	}{
		{"samples", 6 * 3600},
		{"endpoint_stats", 24 * 3600},
		{"dns_queries", 24 * 3600},
		{"connection_events", 24 * 3600},
		{"agent_iface_samples", 24 * 3600},
		{"process_traffic", 24 * 3600},
		{"device_iface_samples", 24 * 3600},
		{"flows", 3600},
		{"syslog_events", 3600},
	} {
		tsTry(db, `SELECT create_hypertable('`+h.table+`', 'ts',
			chunk_time_interval => `+strconv.FormatInt(h.chunk, 10)+`,
			if_not_exists => TRUE, migrate_data => TRUE)`,
			"hypertable "+h.table)
		tsTry(db, `SELECT set_integer_now_func('`+h.table+`', 'bazntms_ts_now')`,
			"integer_now "+h.table)
	}

	// continuous aggregate'lar (downsample): 1dk ve 1sa kovalar
	// materialized_only=false → gerçek-zamanlı agregasyon: ham veri hemen görünür
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS samples_1m
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(60, ts) AS bucket, device,
			AVG(bps_in) AS avg_bps_in, AVG(bps_out) AS avg_bps_out,
			AVG(bps_local) AS avg_bps_local, SUM(pps) AS pps, SUM(dropped) AS dropped
		FROM samples GROUP BY bucket, device WITH NO DATA`,
		"continuous aggregate samples_1m")
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS samples_1h
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(3600, ts) AS bucket, device,
			AVG(bps_in) AS avg_bps_in, AVG(bps_out) AS avg_bps_out,
			AVG(bps_local) AS avg_bps_local, SUM(pps) AS pps, SUM(dropped) AS dropped
		FROM samples GROUP BY bucket, device WITH NO DATA`,
		"continuous aggregate samples_1h")

	// cagg yenileme politikalari
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1m',
		start_offset => 7200::BIGINT, end_offset => 60::BIGINT,
		schedule_interval => INTERVAL '5 minutes')`,
		"cagg policy samples_1m")
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy samples_1h")

	// retention: ham 7g → 1dk 90g → 1sa 2y
	tsTry(db, `SELECT add_retention_policy('samples', retain_after => 604800::BIGINT,
		schedule_interval => INTERVAL '1 hour')`, "retention samples (7g)")
	tsTry(db, `SELECT add_retention_policy('samples_1m', retain_after => 7776000::BIGINT,
		schedule_interval => INTERVAL '1 hour')`, "retention samples_1m (90g)")
	tsTry(db, `SELECT add_retention_policy('samples_1h', retain_after => 63072000::BIGINT,
		schedule_interval => INTERVAL '1 hour')`, "retention samples_1h (2y)")

	slog.Info("timescaledb aktif — hypertable, downsample ve retention politikaları kuruldu")
	return true
}

// tsTry, TimescaleDB kurulum ifadesini calistirir; hata olursa loglayip
// devam eder (best-effort, surum farkliliklarina toleransli).
func tsTry(db *sql.DB, stmt, label string) {
	if _, err := db.Exec(stmt); err != nil {
		slog.Warn("timescale kurulum adimi atlandi", "adim", label, "err", err.Error())
	}
}
