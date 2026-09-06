package store

import (
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// Store, hub'in tum kalici veri islemlerinin sozlesmesidir; alan bazli alt
// arayuzlerin birlesimidir (D2, Faz 13 S13.8). SQLite ve PostgreSQL/TimescaleDB
// arka uclarinin ikisi de (sqlStore) bu arayuzu saglar. Alt arayuzler
// mock'lamayi ve alan bazli bagimlilik bildirmeyi kolaylastirir; her alt
// arayuzun implementasyonu ilgili dosyada (agents.go, devices.go, ...).
type Store interface {
	BaseStore
	CaptureStore
	FleetReportStore
	AlertStore
	AgentStore
	DeviceStore
	AuthStore
	TopologyStore
	ComplianceStore
	IsmsStore
	ClusterStore
}

// ClusterStore, çoklu replika koordinasyonu (C1, Faz 15): tek-sahipli roller
// (uyarı motoru, SNMP poller) için DB tabanlı liderlik.
type ClusterStore interface {
	Leader(key int64, name string) *Leader
}

// BaseStore, her arka ucun sagladigi yasam dongusu.
type BaseStore interface {
	Close() error
	Ping() error
}

// CaptureStore, hub yerel yakalamasinin (collector) yazdigi zaman-serisi
// tablolari ve bunlarin sorgulari.
type CaptureStore interface {
	InsertSample(sm Sample) error
	InsertEndpointDeltas(list []EndpointDelta) error
	InsertDNSDeltas(list []DNSDelta) error
	InsertConnectionEvents(list []ConnectionEvent) error
	Prune(retention time.Duration) error
	// ConfigureRetention, TimescaleDB modunda native chunk-drop retention
	// politikalarini kurar/gunceller (plain PG / SQLite'ta no-op).
	ConfigureRetention(retention time.Duration) error

	TimeseriesBuckets(since time.Time) ([]Bucket, error)
	PeriodTotals(since time.Time) (Totals, error)
	TopEndpointsSince(since time.Time, limit int) ([]EndpointDelta, error)
	ProtocolTotals(since time.Time) (map[string]uint64, error)
	TopProcessesSince(since time.Time, limit int) ([]ProcessUsage, error)
	TopDomainsSince(since time.Time, limit int) ([]DNSDelta, error)
	DailyTotals(days int) ([]DayTotal, error)
}

// FleetReportStore, filo raporlama sorgulari (agent arayuz telemetrisi +
// NetFlow + SNMP cihaz sayaclari — hub yerel yakalamasina/`samples` tablosuna
// bagli degil; coklu-hub kurulumunda calisan tek rapor kaynagi).
type FleetReportStore interface {
	FleetTrafficBuckets(since time.Time, bucketSecs int) ([]Bucket, error)
	FleetSummary(onlineWindow time.Duration) (FleetSummary, error)
	FleetProtocolTotals(since time.Time) (map[string]uint64, error)
	FleetTopEndpoints(since time.Time, limit int, site string) ([]EndpointDelta, error)
	FleetIfaceHealth(since time.Time) (discards uint64, errors uint64, err error)
}

// AlertStore, uyari motorunun kalici durumu.
type AlertStore interface {
	InsertAlertEvent(e AlertEvent) (int64, error)
	RecentAlertEvents(limit int) ([]AlertEvent, error)
	IsAlertSeen(kind, key string) (bool, error)
	MarkAlertSeen(kind, key string) error
	CountAlertSeen(kind string) (int, error)
	LoadAlertConfig() (string, error)
	SaveAlertConfig(cfg string) error
}

// AgentStore, agent filosu (Faz 1) + surec trafigi / L7 / DNS (Faz 2).
type AgentStore interface {
	RegisterAgent(a Agent) (int64, error)
	// RegisterOrReuseAgent, machine_id ile eşleşen çevrimdışı kaydı yeniden
	// kullanır (C3, Faz 13) — state dosyası kaybında filo listesi şişmesin.
	RegisterOrReuseAgent(a Agent, offlineBefore int64) (id int64, reused bool, err error)
	AgentByTokenHash(hash string) (*Agent, error)
	TouchAgent(id int64, version string, protoVersion int, remoteIP string) error
	SaveIfaceSamples(agentID int64, ts int64, samples []telemetry.InterfaceSample) error
	ReplaceConnLatest(agentID int64, conns []telemetry.ConnectionSample) error
	ListAgents(onlineWindow time.Duration, site string) ([]AgentWithRates, error)
	LatestAgentConnections(agentID int64) []telemetry.ConnectionSample
	AgentHistory(agentID int64, since time.Time) ([]Bucket, error)
	AgentByID(id int64) (*Agent, error)
	DeleteAgent(id int64) error
	// PruneOfflineAgents, offlineFor'dan uzun süredir görülmeyen agent'ları tam
	// cascade ile siler (S13.7); silinen sayısını döndürür. offlineFor <= 0 → no-op.
	PruneOfflineAgents(offlineFor time.Duration) (int, error)
	RenameAgent(id int64, name string) error

	SaveProcessTraffic(agentID int64, ts int64, samples []telemetry.ProcessTrafficSample) error
	TopProcessTraffic(since time.Time, agentID int64, limit int, site string) ([]ProcessTrafficUsage, error)
	// L7 uygulama gorunurlugu (SNI + HTTP Host, surec bazli)
	SaveL7(agentID int64, ts int64, samples []telemetry.L7Sample) error
	TopL7(since time.Time, agentID int64, limit int, site string) ([]L7Usage, error)
	SaveAgentDNS(agentID int64, ts int64, samples []telemetry.DNSSample) error
	TopAgentDNS(since time.Time, agentID int64, limit int, site string) ([]AgentDNSUsage, error)
	// RecentAgentDomains, IOC eslestirmesi icin son penceredeki L7+DNS alan adlari
	RecentAgentDomains(since time.Time) ([]AgentDomainSeen, error)
}

// DeviceStore, SNMP/NetFlow/syslog cihaz verisi (Faz 3) + FortiGate REST
// toplama (Faz 8).
type DeviceStore interface {
	AddDevice(d Device) (int64, error)
	ListDevices(site string) ([]Device, error)
	DeviceByID(id int64) (*Device, error)
	DeleteDevice(id int64) error
	UpdateDevicePoll(id int64, sysName, sysDescr string, lastErr string) error
	SaveDeviceIfaceSamples(deviceID int64, ts int64, ifaces []DeviceIface) error
	LatestDeviceIfaces(deviceID int64) ([]DeviceIfaceRate, error)
	SaveFlows(rows []FlowRow) error
	TopFlows(since time.Time, limit int, site string) ([]FlowRow, error)
	SaveSyslogEvent(e SyslogEvent) error
	RecentSyslog(limit int, site string) ([]SyslogEvent, error)

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
}

// AuthStore, kullanicilar / API token'lari / enroll token'lari / denetim
// kaydi (Faz 5, Faz 10 P2).
type AuthStore interface {
	CreateUser(u User) (int64, error)
	UserByName(username string) (*User, error)
	UserByID(id int64) (*User, error)
	ListUsers() ([]User, error)
	AdminUserExists() (bool, error)
	CountAdmins() (int, error)
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
	RecentAuditEvents(limit int, site string) ([]AuditEvent, error)
	VerifyAuditChain() (ok bool, brokenAt int64, checked int, err error)

	// paylaşımlı oturum deposu (A4, Faz 15 — -session-store=db)
	PutSession(s Session) error
	GetSession(tokenHash string) (*Session, error)
	DeleteSession(tokenHash string) error
	PruneSessions() error
}

// TopologyStore, topoloji kesfi + istatistiksel baseline (Faz 6).
type TopologyStore interface {
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
}

// ComplianceStore, 5651 uyumlu loglama: hash-zincir + Merkle checkpoint +
// gozden gecirme (Faz 9).
type ComplianceStore interface {
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
}

// IsmsStore, ISMS yonetisimi: varlik/risk/SoA/politika/denetim/tedarikci
// (Faz 10).
type IsmsStore interface {
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
