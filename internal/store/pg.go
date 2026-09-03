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
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
)

// migrateLockKey, coklu hub replikasinin (hub-controller + N x hub-ingest,
// bkz. deploy/docker-compose.scale.yml) ayni TAZE veritabanina karsi ayni
// anda baslamasi durumunda es zamanli DDL'leri serilestirmek icin
// kullanilan Postgres advisory lock anahtari (rastgele sabit bir sayi,
// baska hicbir anlami yok).
const migrateLockKey = 8823001

// migratePostgres, semayi olusturur. Coklu replika ayni anda TAZE bir
// veritabanina karsi baslarsa, es zamanli "CREATE TABLE IF NOT EXISTS"
// DDL'leri arasinda pg_type katalog kaydi icin nadir bir yaris durumu
// olusabiliyordu (ERROR: duplicate key value violates unique constraint
// "pg_type_typname_nsp_index") — docker-compose.scale.yml CI duman
// testinde (gercek 3 hub instance'i ayni TAZE Postgres'e karsi neredeyse
// es zamanli basladiginda) canli yakalandi. Tum DDL'i TEK bir baglantida
// (pool'dan degil) bir oturum-duzeyi advisory lock altinda calistirarak
// coklu replikanin migrasyonu SIRAYLA yapmasi (digeri bitirene kadar
// bekleyip sonra tablolarin zaten var oldugunu gorup sessizce devam
// etmesi) saglanir.
func migratePostgres(db *sql.DB) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrasyon baglantisi alinamadi: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrateLockKey); err != nil {
		return fmt.Errorf("migrasyon kilidi alinamadi: %w", err)
	}
	defer conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrateLockKey)

	_, err = conn.ExecContext(ctx, `
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

CREATE TABLE IF NOT EXISTS l7_endpoints (
	ts        BIGINT NOT NULL,
	agent_id  BIGINT NOT NULL,
	pid       INTEGER NOT NULL DEFAULT 0,
	process   TEXT   NOT NULL DEFAULT '',
	kind      TEXT   NOT NULL DEFAULT '',
	host      TEXT   NOT NULL DEFAULT '',
	remote_ip TEXT   NOT NULL DEFAULT '',
	bytes     BIGINT NOT NULL DEFAULT 0,
	hits      BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_l7_ts ON l7_endpoints(ts);
CREATE INDEX IF NOT EXISTS idx_l7_host ON l7_endpoints(host, ts);

CREATE TABLE IF NOT EXISTS devices (
	id           BIGSERIAL PRIMARY KEY,
	name         TEXT    NOT NULL,
	host         TEXT    NOT NULL,
	kind         TEXT    NOT NULL DEFAULT 'other',
	site         TEXT    NOT NULL DEFAULT '',
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
	ts        BIGINT  NOT NULL,
	device_id BIGINT  NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	policy_id BIGINT  NOT NULL,
	name      TEXT    NOT NULL DEFAULT '',
	action    TEXT    NOT NULL DEFAULT '',
	hits      BIGINT  NOT NULL DEFAULT 0,
	bytes     BIGINT  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_forti_policy ON fortigate_policy_hits(device_id, ts);

CREATE TABLE IF NOT EXISTS compliance_logs (
	seq         BIGSERIAL PRIMARY KEY,
	ts          BIGINT  NOT NULL,
	source_type TEXT    NOT NULL,
	source_name TEXT    NOT NULL DEFAULT '',
	src_ip      TEXT    NOT NULL DEFAULT '',
	src_mac     TEXT    NOT NULL DEFAULT '',
	user_id     TEXT    NOT NULL DEFAULT '',
	category    TEXT    NOT NULL DEFAULT 'event',
	message     TEXT    NOT NULL,
	prev_hash   TEXT    NOT NULL DEFAULT '',
	hash        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_comp_logs_ts ON compliance_logs(ts);

CREATE TABLE IF NOT EXISTS log_checkpoints (
	id           BIGSERIAL PRIMARY KEY,
	kind         TEXT    NOT NULL,
	bucket_start BIGINT  NOT NULL,
	bucket_end   BIGINT  NOT NULL,
	record_count INTEGER NOT NULL DEFAULT 0,
	prev_root    TEXT    NOT NULL DEFAULT '',
	root         TEXT    NOT NULL DEFAULT '',
	tsa_status   TEXT    NOT NULL DEFAULT '',
	tsa_time     BIGINT  NOT NULL DEFAULT 0,
	tsa_token    BYTEA,
	signature    TEXT    NOT NULL DEFAULT '',
	signed_at    BIGINT  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_comp_cp ON log_checkpoints(kind, bucket_start);

CREATE TABLE IF NOT EXISTS compliance_reviews (
	id       BIGSERIAL PRIMARY KEY,
	ts       BIGINT  NOT NULL,
	username TEXT    NOT NULL,
	kind     TEXT    NOT NULL,
	period   TEXT    NOT NULL DEFAULT '',
	notes    TEXT    NOT NULL DEFAULT '',
	finding  TEXT    NOT NULL DEFAULT ''
);

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
	id        BIGSERIAL NOT NULL,
	ts        BIGINT    NOT NULL,
	host      TEXT      NOT NULL DEFAULT '',
	source_ip TEXT      NOT NULL DEFAULT '',
	severity  INTEGER   NOT NULL DEFAULT 7,
	tag       TEXT      NOT NULL DEFAULT '',
	message   TEXT      NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS enroll_tokens (
	id         BIGSERIAL PRIMARY KEY,
	name       TEXT    NOT NULL,
	token_hash TEXT    NOT NULL UNIQUE,
	site       TEXT    NOT NULL DEFAULT '',
	created_at BIGINT  NOT NULL,
	expires_at BIGINT  NOT NULL DEFAULT 0,
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

CREATE TABLE IF NOT EXISTS isms_assets (
	id          BIGSERIAL PRIMARY KEY,
	kind        TEXT    NOT NULL,
	name        TEXT    NOT NULL,
	owner       TEXT    NOT NULL DEFAULT '',
	criticality TEXT    NOT NULL DEFAULT 'orta',
	auto        INTEGER NOT NULL DEFAULT 0,
	notes       TEXT    NOT NULL DEFAULT '',
	created_at  BIGINT  NOT NULL,
	UNIQUE (kind, name)
);

CREATE TABLE IF NOT EXISTS isms_risks (
	id             BIGSERIAL PRIMARY KEY,
	asset_id       BIGINT  NOT NULL DEFAULT 0,
	threat         TEXT    NOT NULL,
	vulnerability  TEXT    NOT NULL DEFAULT '',
	impact         INTEGER NOT NULL DEFAULT 3,
	likelihood     INTEGER NOT NULL DEFAULT 3,
	treatment      TEXT    NOT NULL DEFAULT 'mitigate',
	plan           TEXT    NOT NULL DEFAULT '',
	res_impact     INTEGER NOT NULL DEFAULT 0,
	res_likelihood INTEGER NOT NULL DEFAULT 0,
	owner          TEXT    NOT NULL DEFAULT '',
	status         TEXT    NOT NULL DEFAULT 'open',
	created_at     BIGINT  NOT NULL,
	review_ts      BIGINT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS isms_soa (
	control_id    TEXT    PRIMARY KEY,
	category      TEXT    NOT NULL,
	title         TEXT    NOT NULL,
	applicable    INTEGER NOT NULL DEFAULT 1,
	justification TEXT    NOT NULL DEFAULT '',
	status        TEXT    NOT NULL DEFAULT 'planned',
	evidence      TEXT    NOT NULL DEFAULT '',
	owner         TEXT    NOT NULL DEFAULT '',
	updated_at    BIGINT  NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS isms_policies (
	id           BIGSERIAL PRIMARY KEY,
	ref          TEXT    NOT NULL UNIQUE,
	title        TEXT    NOT NULL,
	owner        TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'draft',
	version      TEXT    NOT NULL DEFAULT '1.0',
	approved_by  TEXT    NOT NULL DEFAULT '',
	approved_at  BIGINT  NOT NULL DEFAULT 0,
	published_at BIGINT  NOT NULL DEFAULT 0,
	next_review  BIGINT  NOT NULL DEFAULT 0,
	created_at   BIGINT  NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_policy_versions (
	id          BIGSERIAL PRIMARY KEY,
	policy_id   BIGINT  NOT NULL,
	version     TEXT    NOT NULL,
	content     TEXT    NOT NULL DEFAULT '',
	change_note TEXT    NOT NULL DEFAULT '',
	created_by  TEXT    NOT NULL DEFAULT '',
	created_at  BIGINT  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_isms_polver ON isms_policy_versions(policy_id);

CREATE TABLE IF NOT EXISTS isms_audits (
	id           BIGSERIAL PRIMARY KEY,
	title        TEXT    NOT NULL,
	scope        TEXT    NOT NULL DEFAULT '',
	planned_date TEXT    NOT NULL DEFAULT '',
	performed_at BIGINT  NOT NULL DEFAULT 0,
	auditor      TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'planned',
	summary      TEXT    NOT NULL DEFAULT '',
	created_at   BIGINT  NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_findings (
	id          BIGSERIAL PRIMARY KEY,
	audit_id    BIGINT  NOT NULL,
	ref         TEXT    NOT NULL DEFAULT '',
	description TEXT    NOT NULL,
	severity    TEXT    NOT NULL DEFAULT 'orta',
	control_id  TEXT    NOT NULL DEFAULT '',
	capa        TEXT    NOT NULL DEFAULT '',
	capa_owner  TEXT    NOT NULL DEFAULT '',
	capa_due    TEXT    NOT NULL DEFAULT '',
	status      TEXT    NOT NULL DEFAULT 'open',
	closed_at   BIGINT  NOT NULL DEFAULT 0,
	verified_by TEXT    NOT NULL DEFAULT '',
	created_at  BIGINT  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_isms_findings ON isms_findings(audit_id);

CREATE TABLE IF NOT EXISTS isms_mgmt_reviews (
	id         BIGSERIAL PRIMARY KEY,
	ts         BIGINT  NOT NULL,
	period     TEXT    NOT NULL DEFAULT '',
	attendees  TEXT    NOT NULL DEFAULT '',
	inputs     TEXT    NOT NULL DEFAULT '',
	decisions  TEXT    NOT NULL DEFAULT '',
	actions    TEXT    NOT NULL DEFAULT '',
	created_by TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS isms_suppliers (
	id           BIGSERIAL PRIMARY KEY,
	name         TEXT    NOT NULL,
	service      TEXT    NOT NULL DEFAULT '',
	criticality  TEXT    NOT NULL DEFAULT 'orta',
	data_access  TEXT    NOT NULL DEFAULT '',
	contract_ref TEXT    NOT NULL DEFAULT '',
	risk         TEXT    NOT NULL DEFAULT '',
	last_review  BIGINT  NOT NULL DEFAULT 0,
	next_review  BIGINT  NOT NULL DEFAULT 0,
	notes        TEXT    NOT NULL DEFAULT '',
	created_at   BIGINT  NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_continuity_tests (
	id           BIGSERIAL PRIMARY KEY,
	kind         TEXT    NOT NULL DEFAULT 'restore',
	title        TEXT    NOT NULL,
	performed_at BIGINT  NOT NULL,
	result       TEXT    NOT NULL DEFAULT '',
	evidence     TEXT    NOT NULL DEFAULT '',
	notes        TEXT    NOT NULL DEFAULT '',
	created_by   TEXT    NOT NULL DEFAULT ''
);
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
		{"l7_endpoints", 24 * 3600},
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

	// NetFlow/IPFIX akis ozeti: (cihaz, protokol) basina saatlik toplam. Ham
	// `flows` 7 gunde dusuyor (ConfigureRetention); bu cagg protokol/hacim
	// trendini 1 yil tutar. src/dst KASITLI grupta degil — yuksek kardinalite
	// diski sisirir; "en yogun uc noktalar" ham tablonun 7 gunluk penceresinden
	// gelmeye devam eder.
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS flows_1h
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(3600, ts) AS bucket, device, proto,
			SUM(octets) AS octets, SUM(packets) AS packets, COUNT(*) AS flows
		FROM flows GROUP BY bucket, device, proto WITH NO DATA`,
		"continuous aggregate flows_1h")

	// cagg yenileme politikalari
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1m',
		start_offset => 7200::BIGINT, end_offset => 60::BIGINT,
		schedule_interval => INTERVAL '5 minutes')`,
		"cagg policy samples_1m")
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy samples_1h")
	tsTry(db, `SELECT add_continuous_aggregate_policy('flows_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy flows_1h")

	// downsample cagg'leri icin retention: 1dk kova 90g, 1sa kova 2y.
	// Param adi `drop_after` (eski `retain_after` TS 2.x'te YOK — sessizce
	// fail ediyordu). Ham hypertable'larin retention'i store.ConfigureRetention
	// tarafindan `-retention-hours`'a gore kurulur (acilista, hub main'den).
	tsTry(db, `SELECT add_retention_policy('samples_1m', drop_after => 7776000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention samples_1m (90g)")
	tsTry(db, `SELECT add_retention_policy('samples_1h', drop_after => 63072000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention samples_1h (2y)")
	tsTry(db, `SELECT add_retention_policy('flows_1h', drop_after => 31536000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention flows_1h (1y)")

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
