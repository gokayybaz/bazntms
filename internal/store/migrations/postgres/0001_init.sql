-- 0001_init (PostgreSQL) — Faz 13 S13.2 baseline.
-- Faz 13 öncesi migratePostgres() fonksiyonundaki tek CREATE TABLE bloğuyla
-- BİREBİR aynıdır (advisory lock sarmalayıcısı runner'a taşındı). Mevcut
-- kurulumlarda çalıştırılmaz, doğrudan "uygulandı" işaretlenir
-- (bkz. docs/decisions/0002-migration-framework.md).
--
-- SQLite sürümünden farkları: id kolonları BIGSERIAL, hypertable'a çevrilecek
-- tablolarda birleşik PK (id, ts), tamsayı sayaçlar BIGINT, gerçek sayılar
-- DOUBLE PRECISION. TimescaleDB kurulumu (hypertable / continuous aggregate /
-- retention) migrasyon dışıdır — setupTimescale() best-effort çalıştırır.

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

CREATE TABLE IF NOT EXISTS agent_dns (
	ts        BIGINT NOT NULL,
	agent_id  BIGINT NOT NULL,
	pid       INTEGER NOT NULL DEFAULT 0,
	process   TEXT   NOT NULL DEFAULT '',
	domain    TEXT   NOT NULL DEFAULT '',
	queries   BIGINT NOT NULL DEFAULT 0,
	responses BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_adns_ts ON agent_dns(ts);
CREATE INDEX IF NOT EXISTS idx_adns_dom ON agent_dns(domain, ts);

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
