package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gokayybaz/bazntms/internal/ai"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/report"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/sysmon"
	"github.com/gokayybaz/bazntms/internal/version"
)

type Server struct {
	engine            *capture.Engine
	hub               *Hub
	staticFS          fs.FS
	store             *store.Store
	dbPath            string
	aiClient          *ai.Client
	alerts            *alert.Manager
	geo               *geoip.Resolver
	auth              *AuthManager
	enrollToken       string
	telemetryInterval int
	agentPCAP         bool

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	wsClients    prometheus.Gauge
	captureRun   prometheus.Gauge
	registry     *prometheus.Registry
}

func New(staticFS fs.FS, engine *capture.Engine, st *store.Store, dbPath string, aiClient *ai.Client, alerts *alert.Manager, geo *geoip.Resolver, password string, enrollToken string, telemetryInterval int, agentPCAP bool) *Server {
	s := &Server{
		engine:   engine,
		hub:      NewHub(engine, alerts, geo),
		staticFS: staticFS,
		store:    st,
		dbPath:   dbPath,
		aiClient: aiClient,
		alerts:   alerts,
		geo:      geo,
		auth:     NewAuthManager(password),
	}
	if telemetryInterval <= 0 {
		telemetryInterval = defaultTelemetryInterval
	}
	s.enrollToken = enrollToken
	s.telemetryInterval = telemetryInterval
	s.agentPCAP = agentPCAP
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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/interfaces", s.handleInterfaces)
	mux.HandleFunc("GET /api/connections", s.handleConnections)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/capture/start", s.handleCaptureStart)
	mux.HandleFunc("POST /api/capture/stop", s.handleCaptureStop)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/compare", s.handleCompare)
	mux.HandleFunc("GET /api/report", s.handleReport)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/alerts", s.handleAlertsGet)
	mux.HandleFunc("PUT /api/alerts", s.handleAlertsPut)
	mux.HandleFunc("GET /api/alerts/events", s.handleAlertEvents)
	mux.HandleFunc("POST /api/record/start", s.handleRecordStart)
	mux.HandleFunc("POST /api/record/stop", s.handleRecordStop)
	mux.HandleFunc("GET /api/record/status", s.handleRecordStatus)
	mux.HandleFunc("GET /api/record/files", s.handleRecordFiles)
	mux.HandleFunc("GET /api/record/download", s.handleRecordDownload)
	mux.HandleFunc("GET /api/ai/status", s.handleAIStatus)
	mux.HandleFunc("GET /api/ai/models", s.handleAIModels)
	mux.HandleFunc("POST /api/ai/analyze", s.handleAIAnalyze)
	mux.HandleFunc("GET /api/ai/insights", s.handleAIInsights)

	// /api/v1 — kurumsal API sablonu (Faz 1 ingest uclari da buraya gelir)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/interfaces", s.handleInterfaces)
	mux.HandleFunc("GET /api/v1/connections", s.handleConnections)
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)

	// agent filo uclari (agentAuth: Bearer agent token)
	mux.HandleFunc("POST /api/v1/agent/hello", s.handleAgentHello)
	mux.Handle("POST /api/v1/agent/telemetry", s.agentAuth(http.HandlerFunc(s.handleAgentTelemetry)))
	mux.HandleFunc("GET /api/v1/processes", s.handleProcesses)

	// filo yonetimi (UI auth'u ile korunur)
	mux.HandleFunc("GET /api/v1/agents", s.handleAgentsList)
	mux.HandleFunc("GET /api/v1/agents/{id}", s.handleAgentDetail)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", s.handleAgentDelete)

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

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, version.Info())
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

func (s *Server) handleInterfaces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sysmon.ListInterfaces())
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sysmon.ListConnections())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.engine.Snapshot()
	if s.geo != nil && s.geo.Enabled() {
		for i := range snap.TopEndpoints {
			info := s.geo.Lookup(snap.TopEndpoints[i].IP)
			snap.TopEndpoints[i].Country = info.Country
			snap.TopEndpoints[i].ASN = info.ASN
		}
	}
	writeJSON(w, snap)
}

func (s *Server) handleCaptureStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Device == "" {
		http.Error(w, "device zorunlu", http.StatusBadRequest)
		return
	}
	if err := s.engine.Start(req.Device); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("yakalama basladi", "device", req.Device)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleCaptureStop(w http.ResponseWriter, r *http.Request) {
	s.engine.Stop()
	slog.Info("yakalama durduruldu")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	buckets, err := s.store.TimeseriesBuckets(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if buckets == nil {
		buckets = []store.Bucket{}
	}
	totals, err := s.store.PeriodTotals(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"range_minutes": minutes,
		"db_bytes":      store.FileSize(s.dbPath),
		"totals":        totals,
		"buckets":       buckets,
	})
}

// localMidnight, bugunden offsetDays Gun onceki yerel gece yarisi.
func localMidnight(offsetDays int) time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location()).AddDate(0, 0, offsetDays)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days < 2 || days > 30 {
		days = 7
	}
	daily, err := s.store.DailyTotals(days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	todayH, err := s.store.HourlyAverages(localMidnight(0))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	yestH, err := s.store.HourlyAverages(localMidnight(-1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"days":            daily,
		"today_hours":     todayH,
		"yesterday_hours": yestH,
	})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 || days > 90 {
		days = 7
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
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	writeJSON(w, s.alerts.RecentEvents(limit))
}

// --- pcap kayit ---

func (s *Server) handleRecordStart(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.StartRecording(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("pcap kaydi basladi")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleRecordStop(w http.ResponseWriter, r *http.Request) {
	info, err := s.engine.StopRecording()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("pcap kaydi durduruldu", "file", info.File, "packets", info.Packets)
	writeJSON(w, map[string]any{"ok": true, "file": info.File, "packets": info.Packets, "bytes": info.Bytes})
}

func (s *Server) handleRecordStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.engine.RecordStatus())
}

func (s *Server) handleRecordFiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.engine.ListRecordings())
}

func (s *Server) handleRecordDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Query().Get("file")) // path traversal korumasi
	if !strings.HasSuffix(name, ".pcap") || name == ".pcap" {
		http.Error(w, "geçersiz dosya adı", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.engine.RecordDir(), name)
	if _, err := os.Stat(path); err != nil {
		http.Error(w, "dosya bulunamadı", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, path)
}

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"enabled": s.aiClient.Enabled(),
		"model":   s.aiClient.Model(),
	})
}

func (s *Server) handleAIModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.aiClient.ListModels(r.Context())
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "models": models})
}

func (s *Server) handleAIAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Minutes int    `json:"minutes"`
		Model   string `json:"model"`
		Chunked bool   `json:"chunked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Minutes <= 0 {
		req.Minutes = 30
	}

	// analiz oncesi veri kontrolu: bos donem icin modeli bosuna yorma
	totals, err := s.store.PeriodTotals(time.Now().Add(-time.Duration(req.Minutes) * time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if totals.Samples == 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "Bu dönem için veritabanında kayıt yok. Veriler yalnızca yakalama açıkken birikir — yakalamayı başlatın, birkaç dakika veri toplansın ya da daha uzun bir dönem (ör. 24 saat) seçin."})
		return
	}

	started := time.Now()
	summary, err := s.aiClient.Analyze(r.Context(), s.store, ai.AnalyzeOptions{
		Minutes: req.Minutes,
		Model:   req.Model,
		Chunked: req.Chunked,
	})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	model := s.aiClient.Model()
	if req.Model != "" {
		model = req.Model
	}
	id, err := s.store.InsertInsight(store.Insight{
		Ts: time.Now().Unix(), Model: model,
		PeriodMinutes: req.Minutes, Summary: summary,
	})
	if err != nil {
		slog.Error("AI analiz kaydi hatasi", "err", err)
	}
	slog.Info("AI analizi tamamlandi", "sure", time.Since(started).Round(time.Second).String())
	writeJSON(w, map[string]any{"ok": true, "id": id, "model": model, "summary": summary})
}

func (s *Server) handleAIInsights(w http.ResponseWriter, r *http.Request) {
	insights, err := s.store.RecentInsights(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if insights == nil {
		insights = []store.Insight{}
	}
	writeJSON(w, insights)
}

// EnrollToken, otomatik uretilen enrollment token'ini dondurur (banner logu icin).
func (s *Server) EnrollToken() string { return s.enrollToken }
