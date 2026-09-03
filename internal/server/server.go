package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/report"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
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
	compliance        ComplianceInfo // 5651 uyum durumu (Faz 9)
	enrollToken       string
	telemetryInterval int
	agentPCAP         bool
	vault             *vault.Vault
	agentCA           *pki.CA // nil ise mTLS kapali (enroll CSR imzalamaz, client-cert auth yok)
	enrollAttempts    *enrollAttemptLimiter

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	wsClients    prometheus.Gauge
	captureRun   prometheus.Gauge
	registry     *prometheus.Registry
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
	// server-basina registry: testlerde coklu New() cagrisi guvenli olur
	s.registry = prometheus.NewRegistry()
	s.registry.MustRegister(s.httpRequests, s.httpDuration, s.wsClients, s.captureRun)
	s.registry.MustRegister(collectors.NewGoCollector())

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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /api/report", s.requirePerm(PermAnalyze, http.HandlerFunc(s.handleReport)))
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)
	mux.HandleFunc("GET /api/alerts", s.handleAlertsGet)
	mux.Handle("PUT /api/alerts", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleAlertsPut)))
	mux.HandleFunc("GET /api/alerts/events", s.handleAlertEvents)

	// agent filo uclari (agentAuth: Bearer agent token)
	mux.HandleFunc("POST /api/v1/agent/hello", s.handleAgentHello)
	mux.Handle("POST /api/v1/agent/cert", s.agentAuth(http.HandlerFunc(s.handleAgentCertRenew)))
	mux.Handle("POST /api/v1/agent/telemetry", s.agentAuth(http.HandlerFunc(s.handleAgentTelemetry)))
	mux.Handle("GET /api/v1/agent/update/manifest", s.agentAuth(http.HandlerFunc(s.handleUpdateManifest)))
	mux.Handle("GET /api/v1/agent/update/file/{channel}/{name}", s.agentAuth(http.HandlerFunc(s.handleUpdateFile)))
	mux.HandleFunc("GET /api/v1/processes", s.handleProcesses)
	mux.HandleFunc("GET /api/v1/l7", s.handleL7)

	// filo yonetimi (UI auth'u ile korunur; silme = netops+)
	mux.HandleFunc("GET /api/v1/agents", s.handleAgentsList)
	mux.HandleFunc("GET /api/v1/agents/{id}", s.handleAgentDetail)
	mux.HandleFunc("GET /api/v1/agents/{id}/history", s.handleAgentHistory)
	mux.Handle("DELETE /api/v1/agents/{id}", s.requirePerm(PermManageAgents, http.HandlerFunc(s.handleAgentDelete)))
	mux.Handle("PATCH /api/v1/agents/{id}", s.requirePerm(PermManageAgents, http.HandlerFunc(s.handleAgentRename)))

	// cihazlar ve ag cihazi verileri (Faz 3; ekleme/silme = netops+)
	mux.HandleFunc("GET /api/v1/devices", s.handleDevicesList)
	mux.Handle("POST /api/v1/devices", s.requirePerm(PermManageDevices, http.HandlerFunc(s.handleDeviceAdd)))
	mux.Handle("DELETE /api/v1/devices/{id}", s.requirePerm(PermManageDevices, http.HandlerFunc(s.handleDeviceDelete)))
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
	mux.HandleFunc("GET /api/v1/audit/verify", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleAuditVerify)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/compliance/status", s.handleComplianceStatus)
	mux.HandleFunc("GET /api/v1/compliance/evidence", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleComplianceEvidence)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/compliance/reviews", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleComplianceReviewsGet)).ServeHTTP)
	mux.Handle("POST /api/v1/compliance/reviews", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleComplianceReviewAdd)))

	// ISMS yönetişimi (Faz 10): okuma oturumla, yazma admin
	mux.HandleFunc("GET /api/v1/isms/summary", s.handleIsmsSummary)
	mux.HandleFunc("GET /api/v1/isms/assets", s.handleIsmsAssetsList)
	mux.Handle("POST /api/v1/isms/assets/sync", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAssetsSync)))
	mux.Handle("PUT /api/v1/isms/assets/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAssetUpdate)))
	mux.Handle("DELETE /api/v1/isms/assets/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAssetDelete)))
	mux.HandleFunc("GET /api/v1/isms/risks", s.handleIsmsRisksList)
	mux.Handle("POST /api/v1/isms/risks", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsRiskAdd)))
	mux.Handle("PUT /api/v1/isms/risks/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsRiskUpdate)))
	mux.Handle("DELETE /api/v1/isms/risks/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsRiskDelete)))
	mux.HandleFunc("GET /api/v1/isms/soa", s.handleIsmsSoaList)
	mux.Handle("PUT /api/v1/isms/soa/{control}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsSoaUpdate)))
	mux.HandleFunc("GET /api/v1/isms/policies", s.handleIsmsPoliciesList)
	mux.Handle("POST /api/v1/isms/policies", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsPolicyAdd)))
	mux.Handle("PUT /api/v1/isms/policies/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsPolicyUpdate)))
	mux.Handle("POST /api/v1/isms/policies/{id}/transition", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsPolicyTransition)))
	mux.HandleFunc("GET /api/v1/isms/policies/{id}/versions", s.handleIsmsPolicyVersionsList)
	mux.Handle("POST /api/v1/isms/policies/{id}/versions", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsPolicyVersionAdd)))
	mux.HandleFunc("GET /api/v1/isms/audits", s.handleIsmsAuditsList)
	mux.Handle("POST /api/v1/isms/audits", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAuditAdd)))
	mux.Handle("PUT /api/v1/isms/audits/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAuditUpdate)))
	mux.HandleFunc("GET /api/v1/isms/audits/{id}/findings", s.handleIsmsFindingsList)
	mux.Handle("POST /api/v1/isms/audits/{id}/findings", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsFindingAdd)))
	mux.Handle("PUT /api/v1/isms/findings/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsFindingUpdate)))
	mux.HandleFunc("GET /api/v1/isms/mgmt-reviews", s.handleIsmsMgmtReviewsList)
	mux.Handle("POST /api/v1/isms/mgmt-reviews", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsMgmtReviewAdd)))
	mux.HandleFunc("GET /api/v1/isms/suppliers", s.handleIsmsSuppliersList)
	mux.Handle("POST /api/v1/isms/suppliers", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsSupplierAdd)))
	mux.Handle("PUT /api/v1/isms/suppliers/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsSupplierUpdate)))
	mux.Handle("DELETE /api/v1/isms/suppliers/{id}", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsSupplierDelete)))
	mux.HandleFunc("GET /api/v1/isms/continuity", s.handleIsmsContinuityList)
	mux.Handle("POST /api/v1/isms/continuity", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsContinuityAdd)))
	mux.HandleFunc("GET /api/v1/isms/auditor-package", s.requirePerm(PermAdmin, http.HandlerFunc(s.handleIsmsAuditorPackage)).ServeHTTP)

	// gozlemlenebilirlik (auth muaf — Prometheus/healthcheck standartlari)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

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
			f.Close()
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
	writeJSON(w, map[string]any{"status": "ok"})
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
	promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
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

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 90 {
		days = 7
	}
	// Faz 6.4: kurumsal rapor (SLA/kapasite/banding)
	if r.URL.Query().Get("type") == "enterprise" {
		data, err := report.BuildEnterprise(s.store, days)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
	writeJSON(w, s.alerts.Config())
}

func (s *Server) handleAlertsPut(w http.ResponseWriter, r *http.Request) {
	var cfg alert.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.alerts.UpdateConfig(cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.audit(r, identityFromCtx(r), "alerts.update", "", "")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	writeJSON(w, s.alerts.RecentEvents(limit))
}

// EnrollToken, otomatik uretilen enrollment token'ini dondurur (banner logu icin).
func (s *Server) EnrollToken() string { return s.enrollToken }
