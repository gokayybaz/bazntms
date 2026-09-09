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
	SchedulerStore
	EventStore
	IncidentStore
	AIStore
}

// AIStore, AI analiz altyapısı (Faz 26): sağlayıcı profilleri + kalıcı sohbet
// oturumları. api_key_enc vault ile şifreli — server katmanı şifreler/çözer.
// Bkz. docs/decisions/0014-ai-analysis.md.
type AIStore interface {
	ListAIProviders() ([]AIProvider, error)
	AIProviderByID(id int64) (*AIProvider, error)
	CreateAIProvider(p AIProvider) (int64, error)
	UpdateAIProvider(p AIProvider) error
	DeleteAIProvider(id int64) error

	CreateAIConversation(c AIConversation) (int64, error)
	ListAIConversations(f AIConversationFilter) ([]AIConversation, error)
	AIConversationByID(id int64) (*AIConversation, []AIMessage, error)
	AppendAIMessage(m AIMessage) (int64, error)
	FinalizeAIMessage(id int64, content, errStr string, tokensIn, tokensOut int) error
	SetAIConversationMeta(id int64, title, model string, providerID int64) error
	SetAIConversationArchived(id int64, archived bool) error
	DeleteAIConversation(id int64) error
	PruneAIConversations(before int64) ([]int64, error)
}

// IncidentStore, olay korelasyonu (Faz 24-B) — internal/incident.Engine
// lider-kapılı yazar; UI/rapor okur.
type IncidentStore interface {
	OpenIncidentByCorrelation(key string) (*Incident, error)
	CreateIncident(in Incident) (int64, error)
	BumpIncident(id, lastSeen int64, severity string, riskScore int, reason, summary string) error
	AddIncidentEvidence(incidentID int64, ev IncidentEvidence) error
	ListIncidents(f IncidentFilter) ([]Incident, error)
	IncidentByID(id int64) (*Incident, []IncidentEvidence, error)
	SetIncidentStatus(id int64, status, by string, ts int64) error
	SetIncidentExtRef(id int64, ref string) error
	RecentOpenIncidents(since time.Time) ([]Incident, error)
}

// EventStore, normalleştirilmiş olay akışı (Faz 24-A) — kaynak tabloların
// üzerinde birleşik okuma modeli, yeni yazma hattı yok (ADR 0010).
type EventStore interface {
	QueryEvents(f EventFilter) ([]Event, int64, error)
}

// SchedulerStore, hub-içi zamanlanmış işler (Faz 22 S22.18) —
// internal/scheduler bunları lider-kapılı çalıştırır.
type SchedulerStore interface {
	CreateScheduledJob(j ScheduledJob) (int64, error)
	ListScheduledJobs() ([]ScheduledJob, error)
	DueScheduledJobs(now int64) ([]ScheduledJob, error)
	MarkScheduledJobRun(id, ranAt, nextRun int64, status string) error
	SetScheduledJobEnabled(id int64, enabled bool, nextRun int64) error
	DeleteScheduledJob(id int64) error
	// rapor teslim geçmişi (S22.19)
	InsertReportArchive(a ReportArchive) (int64, error)
	ListReportArchive(site string, limit int) ([]ReportArchive, error)
	ReportArchiveByID(id int64) (*ReportArchive, error)
	PruneReportArchive(before int64) ([]string, error)
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
	FleetTrafficBuckets(since time.Time, bucketSecs int, site string) ([]Bucket, error)
	FleetSummary(onlineWindow time.Duration) (FleetSummary, error)
	FleetProtocolTotals(since time.Time) (map[string]uint64, error)
	FleetTopEndpoints(since time.Time, limit int, site string) ([]EndpointDelta, error)
	FleetIfaceHealth(since time.Time, site string) (discards uint64, errors uint64, err error)
	// SLA hedefleri (Faz 22 S22.21)
	ListSLATargets() ([]SLATarget, error)
	SLATargetFor(site string) (SLATarget, error)
	UpsertSLATarget(t SLATarget) error
	DeleteSLATarget(scope, site string) error
}

// AlertStore, uyari motorunun kalici durumu.
type AlertStore interface {
	InsertAlertEvent(e AlertEvent) (int64, error)
	RecentAlertEvents(limit int) ([]AlertEvent, error)
	// yaşam döngüsü (Faz 22 S22.6): aynı (kind,key) açık olay → yeni satır
	// yerine tekrar sayacını artır.
	OpenAlertEventByKey(kind, key string) (*AlertEvent, error)
	BumpAlertEvent(id, ts int64, message string) error
	// otomatik çözülme (S22.8)
	ResolveAlertEvent(id, ts int64) error
	OpenAlertEventsStale(before int64) ([]AlertEvent, error)
	OpenAlertEventsByKind(kind string) ([]AlertEvent, error)
	// korelasyon (S22.9)
	OpenAlertEventsBySiteSince(site string, since int64) ([]AlertEvent, error)
	AlertEventsSince(since int64) ([]AlertEvent, error) // Faz 24-B korelasyon
	SetAlertEventGroup(id int64, groupID string) error
	// bilet entegrasyonu (S22.14)
	SetAlertEventExtRef(id int64, ref string) error
	GroupExtRef(groupID string) (string, error)
	// bakım pencereleri / susturma (S22.10)
	AddAlertSilence(sl AlertSilence) (int64, error)
	ListAlertSilences(activeOnly bool, now int64) ([]AlertSilence, error)
	DeleteAlertSilence(id int64) error
	// filtreli sorgu + operatör aksiyonları (S22.11)
	QueryAlertEvents(f AlertEventFilter) ([]AlertEvent, int64, error)
	AlertEventByID(id int64) (*AlertEvent, error)
	AckAlertEvent(id int64, by string, ts int64, note string) error
	SetAlertEventNote(id int64, note string) error
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
	// SetAgentAttrInfo, agent'ın her batch'te bildirdiği süreç-atıf teşhisini
	// kaydeder: arka uç ("ebpf"|"pcap"|"etw"|"off"), pcap yakalama arayüzü ve
	// kapalı/başlatılamadı nedeni. method boş = değiştirme (eski agent).
	SetAgentAttrInfo(id int64, method, iface, note string) error
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
	// SetAgentUplink, agent'ı bir erişim katmanı cihazına (switch/AP) bağlar;
	// deviceID nil → "Doğrudan". Canlı Akış gruplama.
	SetAgentUplink(agentID int64, deviceID *int64) error

	SaveProcessTraffic(agentID int64, ts int64, samples []telemetry.ProcessTrafficSample) error
	TopProcessTraffic(since time.Time, agentID int64, limit int, site string) ([]ProcessTrafficUsage, error)
	// L7 uygulama gorunurlugu (SNI + HTTP Host, surec bazli)
	SaveL7(agentID int64, ts int64, samples []telemetry.L7Sample) error
	TopL7(since time.Time, agentID int64, limit int, site string) ([]L7Usage, error)
	SaveAgentDNS(agentID int64, ts int64, samples []telemetry.DNSSample) error
	TopAgentDNS(since time.Time, agentID int64, limit int, site string) ([]AgentDNSUsage, error)
	// RecentAgentDomains, IOC eslestirmesi icin son penceredeki L7+DNS alan adlari
	RecentAgentDomains(since time.Time) ([]AgentDomainSeen, error)

	// süreç detayı / derin inceleme (Faz 23-A) — tek agent + süreç adı kapsamlı
	// sunucu-tarafı toplamalar (yeni tablo yok).
	ProcessSummary(agentID int64, process string, since time.Time) (ProcessSummary, error)
	ProcessRemotes(agentID int64, process string, since time.Time, limit int) ([]ProcessRemote, error)
	ProcessAppVisibility(agentID int64, process string, since time.Time, limit int) ([]ProcessAppObservation, error)
	ProcessTimeline(agentID int64, agentName, process string, since time.Time, limit int) ([]ProcessTimelineEntry, error)
}

// DeviceStore, SNMP/NetFlow/syslog cihaz verisi (Faz 3) + FortiGate REST
// toplama (Faz 8).
type DeviceStore interface {
	AddDevice(d Device) (int64, error)
	ListDevices(site string) ([]Device, error)
	DeviceByID(id int64) (*Device, error)
	DeleteDevice(id int64) error
	// SetDeviceUplink, cihazın üst cihazını (switch → router zinciri) atar;
	// uplink nil → köke bağlı.
	SetDeviceUplink(deviceID int64, uplink *int64) error
	UpdateDevicePoll(id int64, sysName, sysDescr string, lastErr string) error
	// UpdateDeviceFortiMeta, FortiGate cihazında tespit edilen FortiOS sürümü +
	// uç yetenek özeti (JSON); boş değerler mevcut veriyi ezmez.
	UpdateDeviceFortiMeta(id int64, version, capsJSON string) error
	// SetDeviceFortiProfile, kullanıcının pinlediği sürüm profili ("" → auto).
	SetDeviceFortiProfile(id int64, profile string) error
	// SetDeviceVDOM, FortiGate cihazının hedef VDOM'u ("" → root; "all" → hepsi).
	SetDeviceVDOM(id int64, vdom string) error
	SaveDeviceIfaceSamples(deviceID int64, ts int64, ifaces []DeviceIface) error
	LatestDeviceIfaces(deviceID int64) ([]DeviceIfaceRate, error)
	SaveFlows(rows []FlowRow) error
	TopFlows(since time.Time, limit int, site string) ([]FlowRow, error)
	// NetFlow konuşma toplama (Faz 23-B) — 5'li / uç-çifti, ham flows üstünde
	// sunucu-tarafı GROUP BY.
	FlowConversations(since time.Time, by, sortBy string, limit int, site string) ([]FlowConversation, error)
	FlowConversationDetail(since time.Time, src, dst, proto string, limit int, site string) ([]FlowRow, error)
	FlowActorsForConversation(since time.Time, ipA, ipB string) ([]FlowActor, error)
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
	ConsumeEnrollToken(id int64) (bool, error) // Faz 25-D — atomik kullanım sayacı
	InsertAuditEvent(e AuditEvent) (int64, error)
	RecentAuditEvents(limit int, site string) ([]AuditEvent, error)
	QueryAuditEvents(f AuditFilter) ([]AuditEvent, error) // Faz 25-C — süzgeçli
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
	// AvgBpsSince, hub-yerel (`samples`) current-window ortalamasi (dim="local").
	AvgBpsSince(since time.Time) (float64, error)
	// AvgMetricByDim, current-window ortalamasi boyut × metrik bazinda
	// (dim = "fleet" | "site" | "agent" ; metric = "bps" | "dns_qps" | "proc_bps")
	// — S22.3/S22.4 cok-boyutlu anomali. Coklu-hub'da `samples` bos oldugu icin
	// baseline karsilastirmasi buradan yapilir.
	AvgMetricByDim(dim, metric string, since time.Time) (map[string]float64, error)
	DropStats(since time.Time) (dropped uint64, pps uint64, err error)
	// anomali baseline alt-toplamlari (S22.2 mevsimsel · S22.3/S22.4 cok-boyutlu):
	// dim = "local"|"fleet"|"site"|"agent", metric = "bps"|"dns_qps"|"proc_bps".
	// rebuildAnomalyBaseline bunlari EWMA agirligiyla birlestirip
	// SaveAnomalyBaseline ile materyalize eder; checkAnomaly LoadAnomalyBaseline
	// ile okur (S22.1).
	BaselineDayBuckets(dim, metric string, days int, seasonality string) ([]BaselineDayBucket, error)
	SaveAnomalyBaseline(rows []AnomalyBaselineRow) error
	LoadAnomalyBaseline(dim, metric string) ([]AnomalyBaselineRow, error)
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
