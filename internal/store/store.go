// Package store, hub'in kalici veri katmanidir (Faz 4.1).
//
// Store arayuzunun iki arka ucu vardir; Open(path) DSN semasindan secim yapar:
//   - SQLite (dev modu): path bir dosya yolu (modernc.org/sqlite, CGO'suz)
//   - PostgreSQL/TimescaleDB (olcek modu): path postgres:// veya postgresql://
//     ile baslar (pgx stdlib driver); TimescaleDB kuruluysa hypertable,
//     continuous aggregate ve retention politikalari otomatik acilir (Faz 4.3)
//
// Zaman kolonlari her iki dialect'te de unix saniye (BIGINT/INTEGER) tutulur;
// boylece sorgu mantigi dialect'ten bagimsizdir. Sorgu metinleri '?' yer
// tutucusuyla yazilir; PostgreSQL'e gonderilirken $n'ye cevrilir.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // postgres:// DSN icin driver kaydi
	_ "modernc.org/sqlite"             // SQLite (dev modu) driver kaydi

	"github.com/gokayybaz/bazntms/pkg/telemetry"
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

// Store, hub'in tum kalici veri islemlerinin sozlesmesidir. SQLite ve
// PostgreSQL/TimescaleDB arka uclarinin ikisi de bu arayuzu saglar.
type Store interface {
	Close() error
	Ping() error

	// yakalama verisi (collector)
	InsertSample(sm Sample) error
	InsertEndpointDeltas(list []EndpointDelta) error
	InsertDNSDeltas(list []DNSDelta) error
	InsertConnectionEvents(list []ConnectionEvent) error
	Prune(retention time.Duration) error
	// ConfigureRetention, TimescaleDB modunda native chunk-drop retention
	// politikalarini kurar/gunceller (plain PG / SQLite'ta no-op).
	ConfigureRetention(retention time.Duration) error

	// sorgular
	TimeseriesBuckets(since time.Time) ([]Bucket, error)
	PeriodTotals(since time.Time) (Totals, error)
	TopEndpointsSince(since time.Time, limit int) ([]EndpointDelta, error)
	ProtocolTotals(since time.Time) (map[string]uint64, error)
	TopProcessesSince(since time.Time, limit int) ([]ProcessUsage, error)
	TopDomainsSince(since time.Time, limit int) ([]DNSDelta, error)
	DailyTotals(days int) ([]DayTotal, error)

	// filo raporlama sorgulari (agent arayuz telemetrisi + NetFlow + SNMP
	// cihaz sayaclari — hub yerel yakalamasina/`samples` tablosuna bagli
	// degil; coklu-hub kurulumunda calisan tek rapor kaynagi).
	FleetTrafficBuckets(since time.Time, bucketSecs int) ([]Bucket, error)
	FleetProtocolTotals(since time.Time) (map[string]uint64, error)
	FleetTopEndpoints(since time.Time, limit int) ([]EndpointDelta, error)
	FleetIfaceHealth(since time.Time) (discards uint64, errors uint64, err error)

	// uyarilar
	InsertAlertEvent(e AlertEvent) (int64, error)
	RecentAlertEvents(limit int) ([]AlertEvent, error)
	IsAlertSeen(kind, key string) (bool, error)
	MarkAlertSeen(kind, key string) error
	CountAlertSeen(kind string) (int, error)
	LoadAlertConfig() (string, error)
	SaveAlertConfig(cfg string) error

	// agent filosu (Faz 1)
	RegisterAgent(a Agent) (int64, error)
	AgentByTokenHash(hash string) (*Agent, error)
	TouchAgent(id int64, version, remoteIP string) error
	SaveIfaceSamples(agentID int64, ts int64, samples []telemetry.InterfaceSample) error
	ReplaceConnLatest(agentID int64, conns []telemetry.ConnectionSample) error
	ListAgents(onlineWindow time.Duration, site string) ([]AgentWithRates, error)
	LatestAgentConnections(agentID int64) []telemetry.ConnectionSample
	AgentHistory(agentID int64, since time.Time) ([]Bucket, error)
	AgentByID(id int64) (*Agent, error)
	DeleteAgent(id int64) error
	RenameAgent(id int64, name string) error

	// surec trafigi (Faz 2)
	SaveProcessTraffic(agentID int64, ts int64, samples []telemetry.ProcessTrafficSample) error
	TopProcessTraffic(since time.Time, agentID int64, limit int) ([]ProcessTrafficUsage, error)

	// cihazlar, flow, syslog (Faz 3)
	AddDevice(d Device) (int64, error)
	ListDevices() ([]Device, error)
	DeviceByID(id int64) (*Device, error)
	DeleteDevice(id int64) error
	UpdateDevicePoll(id int64, sysName, sysDescr string, lastErr string) error
	SaveDeviceIfaceSamples(deviceID int64, ts int64, ifaces []DeviceIface) error
	LatestDeviceIfaces(deviceID int64) ([]DeviceIfaceRate, error)
	SaveFlows(rows []FlowRow) error
	TopFlows(since time.Time, limit int) ([]FlowRow, error)
	SaveSyslogEvent(e SyslogEvent) error
	RecentSyslog(limit int) ([]SyslogEvent, error)

	// kullanicilar, token'lar, denetim kaydi (Faz 5)
	CreateUser(u User) (int64, error)
	UserByName(username string) (*User, error)
	UserByID(id int64) (*User, error)
	ListUsers() ([]User, error)
	UpdateUser(u User) error
	UpdateUserPassword(id int64, passwordHash string) error
	TouchUserLogin(id int64) error
	DeleteUser(id int64) error
	CreateAPIToken(t APIToken) (int64, error)
	APITokenByHash(hash string) (*APIToken, error)
	ListAPITokens() ([]APIToken, error)
	RevokeAPIToken(id int64) error
	DeleteAPIToken(id int64) error
	TouchAPIToken(id int64) error
	// enroll_tokens: -enroll-token bayragindaki TEK statik sirrin yaninda,
	// hub yeniden baslatilmadan olusturulup iptal edilebilen, isimli/opsiyonel
	// son kullanma tarihli ek enrollment token'lari (Faz 10 — plan P2).
	CreateEnrollToken(t EnrollToken) (int64, error)
	EnrollTokenByHash(hash string) (*EnrollToken, error)
	ListEnrollTokens() ([]EnrollToken, error)
	RevokeEnrollToken(id int64) error
	TouchEnrollToken(id int64) error
	InsertAuditEvent(e AuditEvent) (int64, error)
	RecentAuditEvents(limit int) ([]AuditEvent, error)
	VerifyAuditChain() (ok bool, brokenAt int64, checked int, err error)

	// topoloji kesfi ve istatistiksel baseline (Faz 6)
	UpsertTopologyLink(l TopologyLink) error
	RecentTopologyLinks(since time.Time) ([]TopologyLink, error)
	PruneTopology(retention time.Duration) error
	SaveAgentSubnets(agentID int64, name string, subnets []string) error
	HourlyBpsStats() ([]HourStat, error)
	AvgBpsSince(since time.Time) (float64, error)
	// filo (agent telemetrisi) tabanli baseline — coklu-hub'da `samples` bos
	FleetHourlyBpsStats() ([]HourStat, error)
	FleetAvgBpsSince(since time.Time) (float64, error)
	DropStats(since time.Time) (dropped uint64, pps uint64, err error)

	// FortiGate REST toplama (Faz 8)
	SaveDeviceResources(r DeviceResource) error
	LatestDeviceResources(deviceID int64, minutes int) ([]DeviceResource, error)
	SaveFortiVPNStatus(deviceID int64, ts int64, rows []FortiVPNStatus) error
	LatestFortiVPN(deviceID int64) ([]FortiVPNStatus, error)
	SaveFortiSDWAN(deviceID int64, ts int64, rows []FortiSDWANSample) error
	SaveFortiPolicyHits(deviceID int64, ts int64, rows []FortiPolicyHit) error
	TopFortiPolicies(deviceID int64, since time.Time, limit int) ([]FortiPolicyHit, error)
	FortiVPNsDown(freshWithin time.Duration) ([]VPNDownRow, error)
	RecentFortiSDWANAll(since time.Time) ([]SDWANRow, error)
	RecentDeviceResourcesAll(since time.Time) ([]ResourceRow, error)

	// 5651 uyumlu loglama (Faz 9)
	AppendComplianceLog(e ComplianceLog) (int64, error)
	ComplianceHashesBetween(from, to int64) ([][]byte, int64, int64, int, error)
	ComplianceLogsBetween(from, to int64) ([]ComplianceLog, error)
	ComplianceStats() (total int64, lastTs int64, err error)
	PruneComplianceLogs(retentionDays int) error
	SaveLogCheckpoint(cp LogCheckpoint) (int64, error)
	CheckpointExists(kind string, bucketStart int64) (bool, error)
	LatestLogCheckpoint(kind string) (*LogCheckpoint, error)
	LogCheckpointsBetween(from, to int64) ([]LogCheckpoint, error)
	SaveComplianceReview(r ComplianceReview) (int64, error)
	RecentComplianceReviews(limit int) ([]ComplianceReview, error)

	// ISMS yönetişimi (Faz 10)
	SyncIsmsAssetsFromFleet() (int, error)
	ListIsmsAssets() ([]IsmsAsset, error)
	UpdateIsmsAsset(a IsmsAsset) error
	DeleteIsmsAsset(id int64) error
	AddIsmsRisk(r IsmsRisk) (int64, error)
	ListIsmsRisks() ([]IsmsRisk, error)
	UpdateIsmsRisk(r IsmsRisk) error
	DeleteIsmsRisk(id int64) error
	ListIsmsSoa() ([]IsmsSoaItem, error)
	UpdateIsmsSoa(item IsmsSoaItem) error
	IsmsSoaCounts() (total, applicable, implemented, verified, excluded int, err error)
	AddIsmsPolicy(p IsmsPolicy) (int64, error)
	ListIsmsPolicies() ([]IsmsPolicy, error)
	UpdateIsmsPolicy(p IsmsPolicy) error
	AddIsmsPolicyVersion(v IsmsPolicyVersion) (int64, error)
	ListIsmsPolicyVersions(policyID int64) ([]IsmsPolicyVersion, error)
	AddIsmsAudit(a IsmsAudit) (int64, error)
	ListIsmsAudits() ([]IsmsAudit, error)
	UpdateIsmsAudit(a IsmsAudit) error
	AddIsmsFinding(f IsmsFinding) (int64, error)
	ListIsmsFindings(auditID int64) ([]IsmsFinding, error)
	UpdateIsmsFinding(f IsmsFinding) error
	AddIsmsMgmtReview(r IsmsMgmtReview) (int64, error)
	ListIsmsMgmtReviews(limit int) ([]IsmsMgmtReview, error)
	AddIsmsSupplier(sp IsmsSupplier) (int64, error)
	ListIsmsSuppliers() ([]IsmsSupplier, error)
	UpdateIsmsSupplier(sp IsmsSupplier) error
	DeleteIsmsSupplier(id int64) error
	AddIsmsContinuityTest(t IsmsContinuityTest) (int64, error)
	ListIsmsContinuityTests(limit int) ([]IsmsContinuityTest, error)
}

// sqlStore, Store arayuzunun tek somut gerceklemesidir; SQLite ve PostgreSQL
// arasindaki fark (driver, yer tutucu, id dondurme) dialect kontroluyle
// yonetilir.
type sqlStore struct {
	db *sql.DB
	pg bool // PostgreSQL/TimescaleDB modu
	ts bool // TimescaleDB eklentisi aktif (pg modunda anlamlı)

	auditMu sync.Mutex // audit hash-zinciri tutarliligi (Faz 5.3)

	complianceMu sync.Mutex // 5651 log zinciri tutarliligi (Faz 9.1)
}

// Open, veri deposunu acar: DSN postgres:// ile basliyorsa PostgreSQL
// (pgx), aksi halde SQLite dosyasi kullanilir.
func Open(path string) (Store, error) {
	if strings.HasPrefix(path, "postgres://") || strings.HasPrefix(path, "postgresql://") {
		return openPostgres(path)
	}
	return openSQLite(path)
}

func openSQLite(path string) (Store, error) {
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
	s := &sqlStore{db: db}
	if err := s.ensureDeviceColumns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("devices kolon migrasyonu: %w", err)
	}
	if err := s.ensureSyslogColumns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("syslog_events kolon migrasyonu: %w", err)
	}
	if err := s.seedIsmsSoa(); err != nil {
		db.Close()
		return nil, fmt.Errorf("SoA seed: %w", err)
	}
	return s, nil
}

func openPostgres(dsn string) (Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// yuksek eszamanli ingest (5000 agent @ 30sn ≈ 170 ist/sn) icin havuz siniri
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(time.Hour)
	if err := migratePostgres(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres migrasyon: %w", err)
	}
	s := &sqlStore{db: db, pg: true}
	if err := s.ensureDeviceColumns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("devices kolon migrasyonu: %w", err)
	}
	if err := s.ensureSyslogColumns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("syslog_events kolon migrasyonu: %w", err)
	}
	if err := s.seedIsmsSoa(); err != nil {
		db.Close()
		return nil, fmt.Errorf("SoA seed: %w", err)
	}
	s.ts = setupTimescale(db) // best-effort; yoksa duz PostgreSQL modu
	return s, nil
}

// q, sorgu metnini dialect'e uyarlar: SQLite '?' yer tutucusu kullanir,
// PostgreSQL $n bekler.
func (s *sqlStore) q(query string) string {
	if !s.pg {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
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

CREATE TABLE IF NOT EXISTS devices (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
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
	added_at     INTEGER NOT NULL,
	last_poll    INTEGER NOT NULL DEFAULT 0,
	last_error   TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS device_resources (
	ts        INTEGER NOT NULL,
	device_id INTEGER NOT NULL,
	cpu_pct   REAL    NOT NULL DEFAULT 0,
	mem_pct   REAL    NOT NULL DEFAULT 0,
	disk_pct  REAL    NOT NULL DEFAULT 0,
	sessions  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dev_res ON device_resources(device_id, ts);

CREATE TABLE IF NOT EXISTS fortigate_vpn_status (
	device_id INTEGER NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	kind      TEXT    NOT NULL,
	name      TEXT    NOT NULL,
	peer      TEXT    NOT NULL DEFAULT '',
	status    TEXT    NOT NULL DEFAULT '',
	uptime    INTEGER NOT NULL DEFAULT 0,
	rx_bytes  INTEGER NOT NULL DEFAULT 0,
	tx_bytes  INTEGER NOT NULL DEFAULT 0,
	ts        INTEGER NOT NULL,
	PRIMARY KEY (device_id, vdom, kind, name)
);

CREATE TABLE IF NOT EXISTS fortigate_sdwan (
	ts       INTEGER NOT NULL,
	device_id INTEGER NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	member    TEXT    NOT NULL,
	health_check TEXT NOT NULL DEFAULT '',
	latency_ms REAL   NOT NULL DEFAULT 0,
	jitter_ms  REAL   NOT NULL DEFAULT 0,
	packet_loss_pct REAL NOT NULL DEFAULT 0,
	state      TEXT   NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_forti_sdwan ON fortigate_sdwan(device_id, ts);

CREATE TABLE IF NOT EXISTS fortigate_policy_hits (
	ts        INTEGER NOT NULL,
	device_id INTEGER NOT NULL,
	vdom      TEXT    NOT NULL DEFAULT '',
	policy_id INTEGER NOT NULL,
	name      TEXT    NOT NULL DEFAULT '',
	action    TEXT    NOT NULL DEFAULT '',
	hits      INTEGER NOT NULL DEFAULT 0,
	bytes     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_forti_policy ON fortigate_policy_hits(device_id, ts);

CREATE TABLE IF NOT EXISTS compliance_logs (
	seq         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          INTEGER NOT NULL,
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
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	kind         TEXT    NOT NULL,
	bucket_start INTEGER NOT NULL,
	bucket_end   INTEGER NOT NULL,
	record_count INTEGER NOT NULL DEFAULT 0,
	prev_root    TEXT    NOT NULL DEFAULT '',
	root         TEXT    NOT NULL DEFAULT '',
	tsa_status   TEXT    NOT NULL DEFAULT '',
	tsa_time     INTEGER NOT NULL DEFAULT 0,
	tsa_token    BLOB,
	signature    TEXT    NOT NULL DEFAULT '',
	signed_at    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_comp_cp ON log_checkpoints(kind, bucket_start);

CREATE TABLE IF NOT EXISTS compliance_reviews (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	ts       INTEGER NOT NULL,
	username TEXT    NOT NULL,
	kind     TEXT    NOT NULL,
	period   TEXT    NOT NULL DEFAULT '',
	notes    TEXT    NOT NULL DEFAULT '',
	finding  TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS device_iface_samples (
	device_id   INTEGER NOT NULL,
	ts          INTEGER NOT NULL,
	if_index    INTEGER NOT NULL,
	name        TEXT    NOT NULL DEFAULT '',
	alias       TEXT    NOT NULL DEFAULT '',
	speed       INTEGER NOT NULL DEFAULT 0,
	oper_status INTEGER NOT NULL DEFAULT 0,
	rx_bytes    INTEGER NOT NULL DEFAULT 0,
	tx_bytes    INTEGER NOT NULL DEFAULT 0,
	in_errors   INTEGER NOT NULL DEFAULT 0,
	out_errors  INTEGER NOT NULL DEFAULT 0,
	in_discards INTEGER NOT NULL DEFAULT 0,
	out_discards INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dev_iface ON device_iface_samples(device_id, ts);

CREATE TABLE IF NOT EXISTS flows (
	ts      INTEGER NOT NULL,
	device  TEXT    NOT NULL DEFAULT '',
	src     TEXT    NOT NULL DEFAULT '',
	dst     TEXT    NOT NULL DEFAULT '',
	src_port INTEGER NOT NULL DEFAULT 0,
	dst_port INTEGER NOT NULL DEFAULT 0,
	proto   TEXT    NOT NULL DEFAULT '',
	packets INTEGER NOT NULL DEFAULT 0,
	octets  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_flows_ts ON flows(ts);

CREATE TABLE IF NOT EXISTS syslog_events (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	ts        INTEGER NOT NULL,
	host      TEXT    NOT NULL DEFAULT '',
	source_ip TEXT    NOT NULL DEFAULT '',
	severity  INTEGER NOT NULL DEFAULT 7,
	tag       TEXT    NOT NULL DEFAULT '',
	message   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_syslog_ts ON syslog_events(ts);

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

CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT    NOT NULL UNIQUE,
	password_hash TEXT    NOT NULL DEFAULT '',
	role          TEXT    NOT NULL DEFAULT 'viewer',
	site          TEXT    NOT NULL DEFAULT '',
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_at    INTEGER NOT NULL,
	last_login    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS api_tokens (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	token_hash TEXT    NOT NULL UNIQUE,
	role       TEXT    NOT NULL DEFAULT 'viewer',
	site       TEXT    NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	last_used  INTEGER NOT NULL DEFAULT 0,
	revoked    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS enroll_tokens (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	token_hash TEXT    NOT NULL UNIQUE,
	site       TEXT    NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL DEFAULT 0,
	last_used  INTEGER NOT NULL DEFAULT 0,
	revoked    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_events (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	ts        INTEGER NOT NULL,
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
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          INTEGER NOT NULL,
	kind        TEXT    NOT NULL,
	source_type TEXT    NOT NULL,
	source_id   INTEGER NOT NULL DEFAULT 0,
	source_name TEXT    NOT NULL DEFAULT '',
	local_port  TEXT    NOT NULL DEFAULT '',
	peer_type   TEXT    NOT NULL DEFAULT 'host',
	peer_id     INTEGER NOT NULL DEFAULT 0,
	peer_name   TEXT    NOT NULL DEFAULT '',
	peer_ip     TEXT    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_topo_dedup ON topology_links(kind, source_type, source_id, local_port, peer_name, peer_ip);

CREATE TABLE IF NOT EXISTS isms_assets (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	kind        TEXT    NOT NULL,
	name        TEXT    NOT NULL,
	owner       TEXT    NOT NULL DEFAULT '',
	criticality TEXT    NOT NULL DEFAULT 'orta',
	auto        INTEGER NOT NULL DEFAULT 0,
	notes       TEXT    NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL,
	UNIQUE (kind, name)
);

CREATE TABLE IF NOT EXISTS isms_risks (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	asset_id       INTEGER NOT NULL DEFAULT 0,
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
	created_at     INTEGER NOT NULL,
	review_ts      INTEGER NOT NULL DEFAULT 0
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
	updated_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS isms_policies (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	ref          TEXT    NOT NULL UNIQUE,
	title        TEXT    NOT NULL,
	owner        TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'draft',
	version      TEXT    NOT NULL DEFAULT '1.0',
	approved_by  TEXT    NOT NULL DEFAULT '',
	approved_at  INTEGER NOT NULL DEFAULT 0,
	published_at INTEGER NOT NULL DEFAULT 0,
	next_review  INTEGER NOT NULL DEFAULT 0,
	created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_policy_versions (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	policy_id   INTEGER NOT NULL,
	version     TEXT    NOT NULL,
	content     TEXT    NOT NULL DEFAULT '',
	change_note TEXT    NOT NULL DEFAULT '',
	created_by  TEXT    NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_isms_polver ON isms_policy_versions(policy_id);

CREATE TABLE IF NOT EXISTS isms_audits (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	title        TEXT    NOT NULL,
	scope        TEXT    NOT NULL DEFAULT '',
	planned_date TEXT    NOT NULL DEFAULT '',
	performed_at INTEGER NOT NULL DEFAULT 0,
	auditor      TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'planned',
	summary      TEXT    NOT NULL DEFAULT '',
	created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_findings (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	audit_id    INTEGER NOT NULL,
	ref         TEXT    NOT NULL DEFAULT '',
	description TEXT    NOT NULL,
	severity    TEXT    NOT NULL DEFAULT 'orta',
	control_id  TEXT    NOT NULL DEFAULT '',
	capa        TEXT    NOT NULL DEFAULT '',
	capa_owner  TEXT    NOT NULL DEFAULT '',
	capa_due    TEXT    NOT NULL DEFAULT '',
	status      TEXT    NOT NULL DEFAULT 'open',
	closed_at   INTEGER NOT NULL DEFAULT 0,
	verified_by TEXT    NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_isms_findings ON isms_findings(audit_id);

CREATE TABLE IF NOT EXISTS isms_mgmt_reviews (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         INTEGER NOT NULL,
	period     TEXT    NOT NULL DEFAULT '',
	attendees  TEXT    NOT NULL DEFAULT '',
	inputs     TEXT    NOT NULL DEFAULT '',
	decisions  TEXT    NOT NULL DEFAULT '',
	actions    TEXT    NOT NULL DEFAULT '',
	created_by TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS isms_suppliers (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT    NOT NULL,
	service      TEXT    NOT NULL DEFAULT '',
	criticality  TEXT    NOT NULL DEFAULT 'orta',
	data_access  TEXT    NOT NULL DEFAULT '',
	contract_ref TEXT    NOT NULL DEFAULT '',
	risk         TEXT    NOT NULL DEFAULT '',
	last_review  INTEGER NOT NULL DEFAULT 0,
	next_review  INTEGER NOT NULL DEFAULT 0,
	notes        TEXT    NOT NULL DEFAULT '',
	created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS isms_continuity_tests (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	kind         TEXT    NOT NULL DEFAULT 'restore',
	title        TEXT    NOT NULL,
	performed_at INTEGER NOT NULL,
	result       TEXT    NOT NULL DEFAULT '',
	evidence     TEXT    NOT NULL DEFAULT '',
	notes        TEXT    NOT NULL DEFAULT '',
	created_by   TEXT    NOT NULL DEFAULT ''
);
`)
	return err
}

func (s *sqlStore) Close() error { return s.db.Close() }

func (s *sqlStore) Ping() error { return s.db.Ping() }

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

// --- uyarilar ---

type AlertEvent struct {
	ID      int64  `json:"id"`
	Ts      int64  `json:"ts"`
	Kind    string `json:"kind"` // bw | port | proc | target
	Key     string `json:"key"`
	Message string `json:"message"`
}

func (s *sqlStore) InsertAlertEvent(e AlertEvent) (int64, error) {
	var id int64
	err := s.db.QueryRow(s.q(`INSERT INTO alert_events (ts, kind, key, message) VALUES (?,?,?,?) RETURNING id`),
		e.Ts, e.Kind, e.Key, e.Message).Scan(&id)
	return id, err
}

func (s *sqlStore) RecentAlertEvents(limit int) ([]AlertEvent, error) {
	rows, err := s.db.Query(s.q(`SELECT id, ts, kind, key, message FROM alert_events ORDER BY id DESC LIMIT ?`), limit)
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
func (s *sqlStore) IsAlertSeen(kind, key string) (bool, error) {
	var one int
	err := s.db.QueryRow(s.q(`SELECT 1 FROM alert_seen WHERE kind = ? AND key = ?`), kind, key).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *sqlStore) MarkAlertSeen(kind, key string) error {
	_, err := s.db.Exec(s.q(`INSERT INTO alert_seen (kind, key, ts) VALUES (?,?,?)
		ON CONFLICT (kind, key) DO NOTHING`), kind, key, time.Now().Unix())
	return err
}

func (s *sqlStore) CountAlertSeen(kind string) (int, error) {
	var n int
	err := s.db.QueryRow(s.q(`SELECT COUNT(*) FROM alert_seen WHERE kind = ?`), kind).Scan(&n)
	return n, err
}

// LoadAlertConfig, tek satirlik JSON yapilandirmasini dondurur; yoksa "" doner.
func (s *sqlStore) LoadAlertConfig() (string, error) {
	var cfg string
	err := s.db.QueryRow(s.q(`SELECT cfg FROM alert_config WHERE id = 1`)).Scan(&cfg)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return cfg, err
}

func (s *sqlStore) SaveAlertConfig(cfg string) error {
	_, err := s.db.Exec(s.q(`INSERT INTO alert_config (id, cfg) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET cfg = excluded.cfg`), cfg)
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
func (s *sqlStore) DailyTotals(days int) ([]DayTotal, error) {
	if days <= 0 {
		days = 7
	}
	_, offset := time.Now().Zone()
	rows, err := s.db.Query(s.q(`SELECT (ts + ?)/86400*86400 AS day,
			AVG(bps_in), AVG(bps_out), MAX(bps_in), MAX(bps_out), COUNT(*)
		FROM samples GROUP BY day ORDER BY day`), offset)
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
