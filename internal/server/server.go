package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/geoip"
	appmetrics "github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/report"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
	"github.com/gokayybaz/bazntms/internal/version"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

type Server struct {
	engine            *capture.Engine
	hub               *Hub
	staticFS          fs.FS
	store             store.Store
	ingest            TelemetrySink // nil ise telemetri dogrudan store'a yazilir
	dbPath            string
	alerts            *alert.Manager
	geo               *geoip.Resolver
	auth              *AuthManager
	oidc              *OIDCManager
	updatesDir        string         // guncelleme kanali dizini (Faz 7.3; bos = kapali)
	reportsDir        string         // zamanlanmis rapor dosya dizini (Faz 22 S22.19)
	compliance        ComplianceInfo // 5651 uyum durumu (Faz 9)
	enrollToken       string
	telemetryInterval int
	agentPCAP         bool
	// multiSite (çoklu-saha / MSP modu, S14.B1): site sert bir yetki sınırıdır
	// — agent kaydı için site-bağlı enroll token zorunlu, enroll token üretimi
	// site ister. Bkz. docs/DEPLOYMENT-MODEL.md.
	multiSite bool
	// mockDevices (S21.2, -mock-devices): ölçek testi için vendor=mock cihaz
	// eklemeye izin ver. Yalnız yük üreteci senaryolarında.
	mockDevices bool
	// publicURL, panelin dış adresi (-public-url). Agent kurulum sihirbazı
	// enroll komutundaki hub adresi için bunu tercih eder — panel bir tünel/
	// reverse-proxy arkasından localhost'ta açılmış olabilir.
	publicURL      string
	vault          *vault.Vault
	agentCA        *pki.CA // nil ise mTLS kapali (enroll CSR imzalamaz, client-cert auth yok)
	enrollAttempts *enrollAttemptLimiter

	httpRequests   *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	wsClients      prometheus.Gauge
	captureRun     prometheus.Gauge
	notifyFailures *prometheus.CounterVec
	ingestDead     *prometheus.CounterVec
	registry       *prometheus.Registry
}

// IngestDead, bir telemetri/flow/syslog mesajı JetStream DLQ'ya taşındığında
// çağrılır (C4, Faz 15). queue.SetDeadLetterHook ile bağlanır.
func (s *Server) IngestDead(subject string) {
	if s.ingestDead != nil {
		s.ingestDead.WithLabelValues(subject).Inc()
	}
}

// TelemetrySink, agent telemetrisini kuyruga aktaran arayuzdur (Faz 4.2,
// NATS JetStream). nil ise handler dogrudan store'a yazar (kuyruksuz mod).
type TelemetrySink interface {
	PublishTelemetry(agentID int64, version, remoteIP string, ts int64, batch *telemetry.TelemetryBatch) error
}

func New(staticFS fs.FS, engine *capture.Engine, st store.Store, dbPath string, alerts *alert.Manager, geo *geoip.Resolver, password string, enrollToken string, telemetryInterval int, agentPCAP bool, v *vault.Vault, ingest TelemetrySink, oidcOpts *OIDCOptions) *Server {
	s := &Server{
		engine:   engine,
		hub:      NewHub(alerts),
		staticFS: staticFS,
		store:    st,
		ingest:   ingest,
		dbPath:   dbPath,
		alerts:   alerts,
		geo:      geo,
		auth:     NewAuthManager(password, st),
		oidc:     NewOIDCManager(derefOIDC(oidcOpts)),
	}
	if telemetryInterval <= 0 {
		telemetryInterval = defaultTelemetryInterval
	}
	s.enrollToken = enrollToken
	s.telemetryInterval = telemetryInterval
	s.agentPCAP = agentPCAP
	s.vault = v
	s.enrollAttempts = newEnrollAttemptLimiter()
	s.hub.setFleetSource(st, telemetryInterval) // WS tick'i filo özetini de taşır
	if s.enrollToken == "" {
		// otomatik token uret; hub banner'i loglar
		buf := make([]byte, 12)
		rand.Read(buf)
		s.enrollToken = hex.EncodeToString(buf)
	}
	// Prometheus metrikleri
	s.httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_http_requests_total",
		Help: "HTTP istek sayisi (method, path sablonu, durum kodu)",
	}, []string{"method", "path", "status"})
	s.httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bazntms_http_request_duration_seconds",
		Help:    "HTTP istek sureleri (saniye)",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
	s.wsClients = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bazntms_ws_clients",
		Help: "Bagli WebSocket istemci sayisi",
	})
	s.captureRun = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bazntms_capture_running",
		Help: "Paket yakalama aktif mi (1/0)",
	})
	s.notifyFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_notify_failures_total",
		Help: "Kanal basina bildirim teslim hatasi sayisi",
	}, []string{"channel"})
	s.ingestDead = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bazntms_ingest_dead_total",
		Help: "JetStream DLQ'ya (ingest.dead) tasinan mesaj sayisi (orijinal konu bazinda)",
	}, []string{"subject"})
	// server-basina registry: testlerde coklu New() cagrisi guvenli olur
	s.registry = prometheus.NewRegistry()
	s.registry.MustRegister(s.httpRequests, s.httpDuration, s.wsClients, s.captureRun, s.notifyFailures, s.ingestDead)
	s.registry.MustRegister(collectors.NewGoCollector())
	// process_resident_memory_bytes / process_open_fds / process_cpu_seconds_total
	// — soak testi + operasyon için RSS/FD/CPU izleme (S21.13). Linux'ta /proc'tan;
	// diğer platformlarda sessizce boş.
	s.registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	if alerts != nil {
		alerts.SetNotifyFailHook(func(channel string) {
			s.notifyFailures.WithLabelValues(channel).Inc()
		})
	}

	return s
}

func derefOIDC(o *OIDCOptions) OIDCOptions {
	if o == nil {
		return OIDCOptions{}
	}
	return *o
}

// SetAgentCA, agent↔hub mTLS'i etkinlestirir: enrollment sirasinda agent
// CSR'lari bu CA ile imzalanir ve /api/v1/agent/* uclarinda dogrulanmis bir
// istemci sertifikasi Bearer token'a esdeger kimlik sayilir. nil verilirse
// (varsayilan) mTLS kapali kalir.
func (s *Server) SetAgentCA(ca *pki.CA) { s.agentCA = ca }

// SetMultiSite, çoklu-saha (MSP) modunu açar/kapatır (S14.B1).
func (s *Server) SetMultiSite(on bool) { s.multiSite = on }

// SetMockDevices, ölçek testi için `vendor=mock` cihaz eklemeye izin verir
// (S21.2 — `-mock-devices`). Kapalıyken (varsayılan) POST /api/v1/devices bu
// vendor'ı 400 ile reddeder; sürücü seçimi (driver.For) her zaman mock'u tanır
// ama hiçbir üretim cihazının vendor'ı "mock" olamaz.
func (s *Server) SetMockDevices(on bool) { s.mockDevices = on }

// SetPublicURL, panelin dış adresini (-public-url) kaydeder — agent kurulum
// sihirbazı enroll komutundaki hub adresi için `window.location.origin` yerine
// bunu kullanır (panel bir tünel/proxy arkasından localhost'ta açılmış olabilir).
func (s *Server) SetPublicURL(u string) { s.publicURL = strings.TrimRight(u, "/") }

// SetWSOrigins, WebSocket handshake için izin verilen origin host'larını
// ayarlar (B5 — CSWSH savunması). localhost/127.0.0.1/[::1] her zaman eklenir.
// Boş liste → tüm origin'ler kabul (bugünkü davranış) + uyarı logu.
func (s *Server) SetWSOrigins(hosts []string) {
	s.hub.setAllowedOrigins(append([]string{"localhost", "127.0.0.1", "::1"}, hosts...))
}

// UseDBSessions, oturumları paylaşımlı `sessions` tablosuna taşır (A4, Faz 15
// — panel HA). İlk istekten ÖNCE (New sonrası, ListenAndServe öncesi)
// çağrılmalı. ctx bitene dek süresi geçmiş oturumları temizleyen bir janitor
// goroutine başlatır.
func (s *Server) UseDBSessions(ctx context.Context) {
	ds := newDBSessionStore(s.store)
	s.auth.SetSessionStore(ds)
	go ds.runJanitor(ctx)
}

// MultiSite, çoklu-saha modu açık mı.
func (s *Server) MultiSite() bool { return s.multiSite }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /api/report", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReport)))
	// zamanlanmış raporlar + arşiv (S22.19)
	mux.Handle("GET /api/v1/reports/archive", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportArchiveList)))
	mux.Handle("GET /api/v1/reports/archive/{id}", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportArchiveGet)))
	mux.Handle("GET /api/v1/reports/schedules", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportSchedulesList)))
	mux.Handle("POST /api/v1/reports/schedules", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportSchedulePost)))
	mux.Handle("DELETE /api/v1/reports/schedules/{id}", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportScheduleDelete)))
	mux.Handle("POST /api/v1/reports/generate", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReportGenerate)))
	// SLA hedefleri (S22.21)
	mux.Handle("GET /api/v1/sla/targets", s.requirePerm(PermView, http.HandlerFunc(s.handleSLATargetsGet)))
	mux.Handle("PUT /api/v1/sla/targets", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleSLATargetPut)))
	mux.Handle("DELETE /api/v1/sla/targets", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleSLATargetDelete)))
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)
	// GET de admin-korumalı: alert.Config bildirim sırlarını (Telegram token,
	// EmailPass, WebhookV2Secret, SIEM token) taşır — PUT zaten admin'di.
	mux.Handle("GET /api/alerts", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleAlertsGet)))
	mux.Handle("PUT /api/alerts", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleAlertsPut)))
	mux.Handle("GET /api/alerts/status", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleAlertsStatus)))
	mux.Handle("POST /api/alerts/test", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleAlertsTest)))
	mux.HandleFunc("GET /api/alerts/events", s.handleAlertEvents)
	// anomali paneli (S22.5) — düşük hassasiyetli analiz verisi, PermView.
	mux.Handle("GET /api/v1/anomaly/baseline", s.requirePerm(PermView, http.HandlerFunc(s.handleAnomalyBaseline)))
	mux.Handle("GET /api/v1/anomaly/active", s.requirePerm(PermView, http.HandlerFunc(s.handleAnomalyActive)))
	// bakım pencereleri / susturma (S22.10)
	mux.Handle("GET /api/v1/alerts/silences", s.requirePerm(PermView, http.HandlerFunc(s.handleSilencesGet)))
	mux.Handle("POST /api/v1/alerts/silences", s.requirePerm(PermOperate, http.HandlerFunc(s.handleSilencesPost)))
	mux.Handle("DELETE /api/v1/alerts/silences/{id}", s.requirePerm(PermOperate, http.HandlerFunc(s.handleSilenceDelete)))
	// filtreli olay listesi + operatör aksiyonları (S22.11)
	mux.Handle("GET /api/v1/alerts/events", s.requirePerm(PermView, http.HandlerFunc(s.handleAlertEventsV1)))
	mux.Handle("POST /api/v1/alerts/events/{id}/ack", s.requirePerm(PermOperate, http.HandlerFunc(s.handleAlertEventAck)))
	mux.Handle("POST /api/v1/alerts/events/{id}/resolve", s.requirePerm(PermOperate, http.HandlerFunc(s.handleAlertEventResolve)))
	mux.Handle("POST /api/v1/alerts/events/{id}/note", s.requirePerm(PermOperate, http.HandlerFunc(s.handleAlertEventNote)))

	// agent filo uclari (agentAuth: Bearer agent token)
	mux.HandleFunc("POST /api/v1/agent/hello", s.handleAgentHello)
	mux.Handle("POST /api/v1/agent/cert", s.agentAuth(http.HandlerFunc(s.handleAgentCertRenew)))
	mux.Handle("POST /api/v1/agent/telemetry", s.agentAuth(http.HandlerFunc(s.handleAgentTelemetry)))
	mux.Handle("GET /api/v1/agent/update/manifest", s.agentAuth(http.HandlerFunc(s.handleUpdateManifest)))
	mux.Handle("GET /api/v1/agent/update/file/{channel}/{name}", s.agentAuth(http.HandlerFunc(s.handleUpdateFile)))
	mux.HandleFunc("GET /api/v1/processes", s.handleProcesses)
	mux.HandleFunc("GET /api/v1/l7", s.handleL7)
	mux.HandleFunc("GET /api/v1/dns", s.handleAgentDNS)
	mux.HandleFunc("GET /api/v1/geo", s.handleGeo)

	// filo yonetimi (UI auth'u ile korunur; silme = netops+)
	mux.HandleFunc("GET /api/v1/agents", s.handleAgentsList)
	mux.HandleFunc("GET /api/v1/agents/{id}", s.handleAgentDetail)
	mux.HandleFunc("GET /api/v1/agents/{id}/history", s.handleAgentHistory)
	mux.HandleFunc("GET /api/v1/agents/{id}/processes/{process}", s.handleProcessDetail)
	mux.Handle("DELETE /api/v1/agents/{id}", s.requirePerm(PermManageAgents, http.HandlerFunc(s.handleAgentDelete)))
	mux.Handle("PATCH /api/v1/agents/{id}", s.requirePerm(PermManageAgents, http.HandlerFunc(s.handleAgentRename)))
	mux.Handle("PUT /api/v1/agents/{id}/uplink", s.requirePerm(PermManageAgents, http.HandlerFunc(s.handleAgentSetUplink)))

	// cihazlar ve ag cihazi verileri (Faz 3; ekleme/silme = netops+)
	mux.HandleFunc("GET /api/v1/devices", s.handleDevicesList)
	mux.Handle("POST /api/v1/devices", s.requirePerm(PermManageDevices, http.HandlerFunc(s.handleDeviceAdd)))
	mux.Handle("DELETE /api/v1/devices/{id}", s.requirePerm(PermManageDevices, http.HandlerFunc(s.handleDeviceDelete)))
	mux.Handle("PUT /api/v1/devices/{id}/uplink", s.requirePerm(PermManageDevices, http.HandlerFunc(s.handleDeviceSetUplink)))
	mux.HandleFunc("GET /api/v1/devices/{id}/interfaces", s.handleDeviceIfaces)
	mux.HandleFunc("GET /api/v1/devices/{id}/resources", s.handleDeviceResources)
	mux.HandleFunc("GET /api/v1/devices/{id}/vpn", s.handleDeviceVPN)
	mux.HandleFunc("GET /api/v1/devices/{id}/sdwan", s.handleDeviceSDWAN)
	mux.HandleFunc("GET /api/v1/devices/{id}/policies", s.handleDevicePolicies)
	mux.HandleFunc("GET /api/v1/flows", s.handleFlows)
	mux.HandleFunc("GET /api/v1/syslog", s.handleSyslogEvents)
	mux.HandleFunc("GET /api/v1/topology", s.handleTopology)

	// RBAC yonetimi (Faz 5): kullanicilar, token'lar, denetim kaydi — admin
	mux.HandleFunc("GET /api/v1/users", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleUsersList)).ServeHTTP)
	mux.Handle("POST /api/v1/users", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleUserCreate)))
	mux.Handle("PUT /api/v1/users/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleUserUpdate)))
	mux.Handle("DELETE /api/v1/users/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleUserDelete)))
	mux.HandleFunc("GET /api/v1/tokens", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleTokensList)).ServeHTTP)
	mux.Handle("POST /api/v1/tokens", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleTokenCreate)))
	mux.Handle("DELETE /api/v1/tokens/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleTokenDelete)))
	mux.HandleFunc("GET /api/v1/enroll-tokens", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleEnrollTokensList)).ServeHTTP)
	mux.Handle("POST /api/v1/enroll-tokens", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleEnrollTokenCreate)))
	mux.Handle("DELETE /api/v1/enroll-tokens/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleEnrollTokenDelete)))
	mux.HandleFunc("GET /api/v1/audit", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleAuditList)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/audit/verify", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleAuditVerify)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/compliance/status", s.handleComplianceStatus)
	mux.HandleFunc("GET /api/v1/compliance/evidence", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleComplianceEvidence)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/compliance/reviews", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleComplianceReviewsGet)).ServeHTTP)
	mux.Handle("POST /api/v1/compliance/reviews", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleComplianceReviewAdd)))

	// ISMS yönetişimi (Faz 10): tümü global-admin — saha-üstü yönetişim,
	// varlıklar filodan senkronlanır (çoklu-saha'da tüm sahaları kapsar). S14.B2.
	mux.Handle("GET /api/v1/isms/summary", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSummary)))
	mux.Handle("GET /api/v1/isms/assets", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAssetsList)))
	mux.Handle("POST /api/v1/isms/assets/sync", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAssetsSync)))
	mux.Handle("PUT /api/v1/isms/assets/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAssetUpdate)))
	mux.Handle("DELETE /api/v1/isms/assets/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAssetDelete)))
	mux.Handle("GET /api/v1/isms/risks", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsRisksList)))
	mux.Handle("POST /api/v1/isms/risks", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsRiskAdd)))
	mux.Handle("PUT /api/v1/isms/risks/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsRiskUpdate)))
	mux.Handle("DELETE /api/v1/isms/risks/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsRiskDelete)))
	mux.Handle("GET /api/v1/isms/soa", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSoaList)))
	mux.Handle("PUT /api/v1/isms/soa/{control}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSoaUpdate)))
	mux.Handle("GET /api/v1/isms/policies", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPoliciesList)))
	mux.Handle("POST /api/v1/isms/policies", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPolicyAdd)))
	mux.Handle("PUT /api/v1/isms/policies/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPolicyUpdate)))
	mux.Handle("POST /api/v1/isms/policies/{id}/transition", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPolicyTransition)))
	mux.Handle("GET /api/v1/isms/policies/{id}/versions", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPolicyVersionsList)))
	mux.Handle("POST /api/v1/isms/policies/{id}/versions", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsPolicyVersionAdd)))
	mux.Handle("GET /api/v1/isms/audits", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAuditsList)))
	mux.Handle("POST /api/v1/isms/audits", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAuditAdd)))
	mux.Handle("PUT /api/v1/isms/audits/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAuditUpdate)))
	mux.Handle("GET /api/v1/isms/audits/{id}/findings", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsFindingsList)))
	mux.Handle("POST /api/v1/isms/audits/{id}/findings", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsFindingAdd)))
	mux.Handle("PUT /api/v1/isms/findings/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsFindingUpdate)))
	mux.Handle("GET /api/v1/isms/mgmt-reviews", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsMgmtReviewsList)))
	mux.Handle("POST /api/v1/isms/mgmt-reviews", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsMgmtReviewAdd)))
	mux.Handle("GET /api/v1/isms/suppliers", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSuppliersList)))
	mux.Handle("POST /api/v1/isms/suppliers", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSupplierAdd)))
	mux.Handle("PUT /api/v1/isms/suppliers/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSupplierUpdate)))
	mux.Handle("DELETE /api/v1/isms/suppliers/{id}", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsSupplierDelete)))
	mux.Handle("GET /api/v1/isms/continuity", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsContinuityList)))
	mux.Handle("POST /api/v1/isms/continuity", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsContinuityAdd)))
	mux.HandleFunc("GET /api/v1/isms/auditor-package", s.requirePerm(PermGlobalAdmin, http.HandlerFunc(s.handleIsmsAuditorPackage)).ServeHTTP)

	// gozlemlenebilirlik (auth muaf — Prometheus/healthcheck standartlari)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	// API sozlesmesi (auth muaf — kesif/entegrasyon dokumantasyonu)
	mux.HandleFunc("GET /api/openapi.yaml", s.handleOpenAPIYAML)
	mux.HandleFunc("GET /api/openapi.json", s.handleOpenAPIJSON)
	mux.HandleFunc("GET /api/docs", s.handleAPIDocs)

	mux.HandleFunc("GET /ws", s.hub.ServeWS)
	mux.HandleFunc("/api/", http.NotFound)

	if s.staticFS != nil {
		fileServer := http.FileServerFS(s.staticFS)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			path := r.URL.Path
			if path == "/" {
				serveIndex(w, s.staticFS)
				return
			}
			f, err := s.staticFS.Open(path[1:])
			if err != nil {
				serveIndex(w, s.staticFS) // SPA history fallback
				return
			}
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
		})
	}

	return s.observe(s.auth.middleware(mux))
}

// statusWriter, yanit durum kodunu yakalar (metrik + loglama icin).
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func newRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// observe, tum isteklere request-id atar, Prometheus metriklerini gunceller
// ve yapılandırılmış slog kaydı dusurur. /ws ve /metrics sessizdir.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		rid := newRequestID()
		w.Header().Set("X-Request-Id", rid)
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)

		pattern := r.Pattern
		if pattern == "" {
			pattern = r.URL.Path
		}
		dur := time.Since(start)
		s.httpRequests.WithLabelValues(r.Method, pattern, strconv.Itoa(sw.code)).Inc()
		s.httpDuration.WithLabelValues(r.Method, pattern).Observe(dur.Seconds())
		slog.Info("http",
			"method", r.Method,
			"path", pattern,
			"status", sw.code,
			"duration_ms", dur.Milliseconds(),
			"request_id", rid,
		)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// version alanı: sürüm/yükseltme doğrulaması için (kimlik doğrulamasız —
	// yalnızca binary sürümü, sır değil). Bkz. docs/RELEASE-RUNBOOK.md.
	writeJSON(w, map[string]any{
		"status":           "ok",
		"version":          version.Version,
		"protocol_version": version.ProtocolVersion,
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unavailable", "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"status": "ready"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.wsClients.Set(float64(s.hub.count()))
	if s.engine.Snapshot().Running {
		s.captureRun.Set(1)
	} else {
		s.captureRun.Set(0)
	}
	// server'ın instance-başına registry'si + paketler-arası ingest metrik
	// registry'si (internal/metrics — global) birlikte sunulur.
	g := prometheus.Gatherers{s.registry, appmetrics.Registry()}
	promhttp.HandlerFor(g, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

func serveIndex(w http.ResponseWriter, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "frontend build bulunamadi (npm run build calistirin)", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeJSONETag, sık çekilen listeler için içerik-hash tabanlı ETag ekler
// (D4 / Faz 16). If-None-Match eşleşirse 304 döner — istemci yeniden
// serileştirmez / yeniden render etmez, ağ trafiği düşer. Zayıf önbellek:
// yanıt yine de hesaplanır (DB sorgusu çalışır) ama gövde gönderilmez.
func writeJSONETag(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", etag)
	// private: kimliğe bağlı (site-kapsamlı) yanıt — paylaşımlı proxy saklamamalı.
	// no-cache: tarayıcı saklar ama her seferinde revalidate eder.
	w.Header().Set("Cache-Control", "private, no-cache")
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// etagMatch, If-None-Match başlığında (virgülle ayrılmış olabilir; "*" özel)
// verilen etag var mı.
func etagMatch(header, etag string) bool {
	if strings.TrimSpace(header) == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "W/")) == etag {
			return true
		}
	}
	return false
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	scope := SiteScope(identityFromCtx(r))
	rtype := r.URL.Query().Get("type")
	// Trafik raporu filo-geneli akış/protokol verisine dayanır; site-kısıtlı
	// kimlik erişemez (B3). Kurumsal + uyumluluk raporları S22.22'de sahaya
	// kırpılarak açıldı.
	if scope != "" && rtype != "enterprise" && rtype != "compliance" {
		http.Error(w, "site-kisitli kimlik filo raporuna erisemez", http.StatusForbidden)
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 90 {
		days = 7
	}
	// Faz 6.4: kurumsal rapor (SLA/kapasite/banding). S22.20: ?site= saha
	// kırılımı + ?format=pdf. S22.22: site-kısıtlı kimlik kendi sahasına kırpılır.
	if rtype == "enterprise" {
		site := r.URL.Query().Get("site")
		if scope != "" {
			site = scope
		}
		data, err := report.BuildEnterprise(s.store, days, site)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("format") == "pdf" {
			pdfBytes, perr := data.RenderEnterprisePDF()
			if perr != nil {
				http.Error(w, "PDF üretilemedi: "+perr.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(pdfBytes)
			return
		}
		htmlBytes, err := data.RenderEnterpriseHTML()
		if err != nil {
			http.Error(w, "HTML üretilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(htmlBytes)
		return
	}
	// Faz 9.5: ISO 27001 kontrol haritası + 5651 durum raporu
	if r.URL.Query().Get("type") == "compliance" {
		data, err := report.BuildComplianceData(s.store)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		htmlBytes, err := report.RenderComplianceHTML(data)
		if err != nil {
			http.Error(w, "HTML üretilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(htmlBytes)
		return
	}
	data, err := report.Build(s.store, s.geo, days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch r.URL.Query().Get("format") {
	case "pdf":
		pdfBytes, err := data.RenderPDF()
		if err != nil {
			http.Error(w, "PDF üretilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=ag-trafik-raporu-%s-%dgun.pdf", time.Now().Format("2006-01-02"), days))
		w.Write(pdfBytes)
	default:
		htmlBytes, err := data.RenderHTML()
		if err != nil {
			http.Error(w, "HTML üretilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(htmlBytes)
	}
}

func (s *Server) handleAlertsGet(w http.ResponseWriter, r *http.Request) {
	cfg := s.alerts.Config()
	// bildirim sırlarını UI'ya maskeli gönder; PUT'ta maskeli değer =
	// "değiştirme" (aşağıda handleAlertsPut geri açar).
	cfg.Notifiers.TelegramToken = maskNonEmpty(cfg.Notifiers.TelegramToken)
	cfg.Notifiers.EmailPass = maskNonEmpty(cfg.Notifiers.EmailPass)
	cfg.Notifiers.WebhookV2Secret = maskNonEmpty(cfg.Notifiers.WebhookV2Secret)
	cfg.Notifiers.SIEM.Token = maskNonEmpty(cfg.Notifiers.SIEM.Token)
	cfg.Notifiers.Jira.APIToken = maskNonEmpty(cfg.Notifiers.Jira.APIToken)
	cfg.Notifiers.ServiceNow.Password = maskNonEmpty(cfg.Notifiers.ServiceNow.Password)
	writeJSON(w, cfg)
}

func (s *Server) handleAlertsPut(w http.ResponseWriter, r *http.Request) {
	var cfg alert.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// maskeli sır alanı (•••) geldiyse kullanıcı onu değiştirmedi — mevcut
	// saklı değeri koru, maske metnini DB'ye yazma.
	cur := s.alerts.Config()
	keepIfMasked := func(in *string, stored string) {
		if *in == secretMask {
			*in = stored
		}
	}
	keepIfMasked(&cfg.Notifiers.TelegramToken, cur.Notifiers.TelegramToken)
	keepIfMasked(&cfg.Notifiers.EmailPass, cur.Notifiers.EmailPass)
	keepIfMasked(&cfg.Notifiers.WebhookV2Secret, cur.Notifiers.WebhookV2Secret)
	keepIfMasked(&cfg.Notifiers.SIEM.Token, cur.Notifiers.SIEM.Token)
	keepIfMasked(&cfg.Notifiers.Jira.APIToken, cur.Notifiers.Jira.APIToken)
	keepIfMasked(&cfg.Notifiers.ServiceNow.Password, cur.Notifiers.ServiceNow.Password)

	if err := s.alerts.UpdateConfig(cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.audit(r, identityFromCtx(r), "alerts.update", "", "")
	writeJSON(w, map[string]any{"ok": true})
}

// handleAlertsStatus, her bildirim kanalının son teslim denemesinin
// durumunu döndürür (D3).
func (s *Server) handleAlertsStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"channels": s.alerts.NotifierStatus()})
}

// handleAlertsTest, etkin bildirim kanallarına sentetik bir uyarı gönderir
// ve kanal başına sonucu döndürür (admin — "Test Et").
func (s *Server) handleAlertsTest(w http.ResponseWriter, r *http.Request) {
	res := s.alerts.TestNotifiers()
	s.audit(r, identityFromCtx(r), "alerts.test", "", fmt.Sprintf("%d kanal denendi", len(res)))
	writeJSON(w, map[string]any{"channels": res})
}

func (s *Server) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	writeJSONETag(w, r, s.alerts.RecentEvents(limit)) // D4: sık pollanır
}

// handleAnomalyBaseline, bir (dim, metric) icin materyalize baseline egrisini
// dondurur — panelin "beklenen bant" grafigi. Site-kapsamli kimlik yalniz
// kendi sahasinin/filonun egrisini gorur (agent boyutu reddedilir).
func (s *Server) handleAnomalyBaseline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dim := q.Get("dim")
	if dim == "" {
		dim = "fleet"
	}
	metric := q.Get("metric")
	if metric == "" {
		metric = "bps"
	}
	key := q.Get("key")
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		switch dim {
		case "site":
			key = scope // kendi sahasina sabitle
		case "agent":
			writeJSON(w, map[string]any{"dim": dim, "metric": metric, "rows": []any{}})
			return
		}
	}
	rows := s.alerts.AnomalyBaseline(dim, metric)
	if key != "" {
		filtered := rows[:0]
		for _, rw := range rows {
			if rw.Key == key {
				filtered = append(filtered, rw)
			}
		}
		rows = filtered
	}
	writeJSON(w, map[string]any{
		"dim": dim, "metric": metric, "key": key,
		"seasonality": s.alerts.Config().Anomaly.Seasonality,
		"rows":        rows,
	})
}

// handleAnomalyActive, o an gozlenen sapmalari dondurur. Site-kapsamli kimlik
// yalniz filo + kendi sahasi sapmalarini gorur (agent boyutu elenir).
func (s *Server) handleAnomalyActive(w http.ResponseWriter, r *http.Request) {
	devs := s.alerts.AnomalyActive()
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		kept := devs[:0]
		for _, d := range devs {
			if d.Dim == "fleet" || d.Dim == "local" || (d.Dim == "site" && d.Key == scope) {
				kept = append(kept, d)
			}
		}
		devs = kept
	}
	writeJSON(w, map[string]any{"deviations": devs})
}

// --- filtreli olay listesi + operatör aksiyonları (S22.11) ---

func (s *Server) handleAlertEventsV1(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	since, _ := strconv.ParseInt(q.Get("since"), 10, 64)
	cursor, _ := strconv.ParseInt(q.Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	f := store.AlertEventFilter{
		Kind: q.Get("kind"), Severity: q.Get("severity"), State: q.Get("state"),
		Site: q.Get("site"), Group: q.Get("group"), Since: since, Cursor: cursor, Limit: limit,
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		f.Site = scope // site-kapsamlı kimlik yalnız kendi sahasının olaylarını görür
	}
	events, next, err := s.alerts.QueryEvents(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONETag(w, r, map[string]any{"events": events, "next_cursor": next})
}

// alertEventForAction, {id}'yi çözer ve site kapsamını doğrular.
func (s *Server) alertEventForAction(w http.ResponseWriter, r *http.Request) (*store.AlertEvent, bool) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return nil, false
	}
	e, err := s.alerts.EventByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	if e == nil {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return nil, false
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" && e.Site != scope {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return nil, false
	}
	return e, true
}

func decodeNote(r *http.Request) string {
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.Note
}

func (s *Server) handleAlertEventAck(w http.ResponseWriter, r *http.Request) {
	e, ok := s.alertEventForAction(w, r)
	if !ok {
		return
	}
	ident := identityFromCtx(r)
	by := ""
	if ident != nil {
		by = ident.Username
	}
	if err := s.alerts.AckEvent(e.ID, by, decodeNote(r)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, ident, "alert.ack", fmt.Sprintf("alert:%d", e.ID), e.Kind)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAlertEventResolve(w http.ResponseWriter, r *http.Request) {
	e, ok := s.alertEventForAction(w, r)
	if !ok {
		return
	}
	if err := s.alerts.ResolveEventByID(e.ID, *e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "alert.resolve", fmt.Sprintf("alert:%d", e.ID), e.Kind)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAlertEventNote(w http.ResponseWriter, r *http.Request) {
	e, ok := s.alertEventForAction(w, r)
	if !ok {
		return
	}
	if err := s.alerts.NoteEvent(e.ID, decodeNote(r)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "alert.note", fmt.Sprintf("alert:%d", e.ID), "")
	writeJSON(w, map[string]any{"ok": true})
}

// --- bakım pencereleri (S22.10) ---

func (s *Server) handleSilencesGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("active")
	sl, err := s.alerts.ListSilences(q == "1" || q == "true")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		kept := sl[:0]
		for _, x := range sl {
			if x.MatchSite == "" || x.MatchSite == scope {
				kept = append(kept, x)
			}
		}
		sl = kept
	}
	writeJSON(w, map[string]any{"silences": sl})
}

func (s *Server) handleSilencesPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MatchKind   string `json:"match_kind"`
		MatchSite   string `json:"match_site"`
		MatchKey    string `json:"match_key"`
		Reason      string `json:"reason"`
		StartsTs    int64  `json:"starts_ts"`
		EndsTs      int64  `json:"ends_ts"`
		DurationMin int    `json:"duration_min"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	now := time.Now().Unix()
	if req.StartsTs <= 0 {
		req.StartsTs = now
	}
	if req.EndsTs <= 0 && req.DurationMin > 0 {
		req.EndsTs = req.StartsTs + int64(req.DurationMin)*60
	}
	if req.EndsTs <= req.StartsTs {
		http.Error(w, "bitiş zamanı başlangıçtan sonra olmalı (ends_ts veya duration_min)", http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	if scope := SiteScope(ident); scope != "" {
		req.MatchSite = scope // site-kapsamlı kimlik yalnız kendi sahasını susturabilir
	}
	by := ""
	if ident != nil {
		by = ident.Username
	}
	id, err := s.alerts.AddSilence(store.AlertSilence{
		MatchKind: req.MatchKind, MatchSite: req.MatchSite, MatchKey: req.MatchKey,
		StartsTs: req.StartsTs, EndsTs: req.EndsTs, Reason: req.Reason,
		CreatedBy: by, CreatedTs: now,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, ident, "alert.silence.create", fmt.Sprintf("silence:%d", id), req.Reason)
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) handleSilenceDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		// site-kapsamlı kimlik yalnız kendi sahasının susturmasını silebilir
		all, _ := s.alerts.ListSilences(false)
		ok := false
		for _, x := range all {
			if x.ID == id && (x.MatchSite == scope) {
				ok = true
			}
		}
		if !ok {
			http.Error(w, "bulunamadı", http.StatusNotFound)
			return
		}
	}
	if err := s.alerts.DeleteSilence(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "alert.silence.delete", fmt.Sprintf("silence:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

// EnrollToken, otomatik uretilen enrollment token'ini dondurur (banner logu icin).
func (s *Server) EnrollToken() string { return s.enrollToken }
