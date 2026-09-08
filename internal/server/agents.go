package server

// Agent filo uclari (Faz 1): enrollment, telemetri, liste.
// UI auth'undan ayrı olarak agent token'lari (Bearer) ile korunur.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	appmetrics "github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

const defaultTelemetryInterval = 30

// enrollAttemptLimiter, /api/v1/agent/hello ucundaki (paylasilan
// X-Enroll-Token'i tahmin etmeye calisan) IP basina deneme sinirlamasidir —
// auth.go'daki login rate-limit (AuthManager.attempts) ile ayni desen
// (attemptLog turu oradan paylasilir) ama enrollment, UI auth'undan (s.auth
// nil olabilir — sifresiz/dev modu) tamamen bagimsiz calismasi gerektigi
// icin ayri bir kilitle tutulur. Guvenlige duyarli karar oldugu icin IP,
// (agentClientIP'nin aksine) dogrudan r.RemoteAddr'dan alinir — bkz.
// agentClientIP'nin kendi yorumundaki gerekce.
type enrollAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptLog
}

func newEnrollAttemptLimiter() *enrollAttemptLimiter {
	return &enrollAttemptLimiter{attempts: map[string]*attemptLog{}}
}

func (l *enrollAttemptLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[ip]
	if !ok {
		return true
	}
	if time.Now().Before(a.block) {
		return false
	}
	if time.Since(a.reset) > attemptWindow {
		a.count = 0
		a.reset = time.Now()
	}
	return a.count < maxAttempts
}

func (l *enrollAttemptLimiter) recordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[ip]
	if !ok {
		a = &attemptLog{reset: time.Now()}
		l.attempts[ip] = a
	}
	a.count++
	if a.count >= maxAttempts {
		a.block = time.Now().Add(attemptWindow)
		a.count = 0
	}
}

func (l *enrollAttemptLimiter) recordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// agentClientIP, agent enrollment/telemetri isteklerinde GORUNTULENECEK
// (yalnizca bilgi amacli — erisim kontrolu/rate-limit icin KULLANILMAZ)
// gercek istemci IP'sini cozer. docker-compose.scale.yml topolojisinde
// agent'lar nginx LB'ye baglanir (bkz. deploy/nginx/lb.conf); LB
// X-Forwarded-For ekliyor ama r.RemoteAddr LB container'inin kendi IP'sini
// gosteriyordu (bkz. docs/TROUBLESHOOTING.md). Yalnizca dogrudan-baglanti
// durumunda (header yok) r.RemoteAddr'a duser; bu fonksiyon guvenlige
// duyarli hicbir karar icin kullanilmamali (auth.go'daki login rate-limit
// KASITLI olarak r.RemoteAddr'i dogrudan kullanmaya devam eder — XFF'e
// guvenmek istemcinin kendi IP'sini sahtelemesine izin verirdi).
func agentClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// agentFromCtx, agentAuth middleware'inin yerlestirdigi kaydi dondurur.
type ctxKey string

const agentCtxKey ctxKey = "agent"

func agentFromCtx(r *http.Request) *store.Agent {
	if v, ok := r.Context().Value(agentCtxKey).(*store.Agent); ok {
		return v
	}
	return nil
}

// agentAuth, agent kimligini dogrular ve kaydi context'e koyar. Iki yol:
//  1. mTLS: TLS katmaninca CA'ya karsi dogrulanmis istemci sertifikasi
//     (CN = bazntms-agent-<id>). Bearer token'a esdeger; sertifika zaten
//     kriptografik olarak baglayici oldugu icin ayrica token istenmez.
//  2. Bearer: Authorization: Bearer <agent_token> (mTLS yoksa tek yol).
func (s *Server) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if agent := s.agentFromClientCert(r); agent != nil {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), agentCtxKey, agent)))
			return
		}
		const prefix = "Bearer "
		token := ""
		if ah := r.Header.Get("Authorization"); len(ah) > len(prefix) && ah[:len(prefix)] == prefix {
			token = ah[len(prefix):]
		}
		if token == "" {
			unauthorized(w, "agent token gerekli")
			return
		}
		agent, err := s.store.AgentByTokenHash(store.TokenHash(token))
		if err != nil {
			unauthorized(w, "gecersiz agent token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), agentCtxKey, agent)))
	})
}

// agentFromClientCert, istegin TLS katmaninca dogrulanmis bir agent istemci
// sertifikasi tasiyip tasimadigina bakar. tls.Config ClientAuth
// VerifyClientCertIfGiven oldugu icin VerifiedChains yalnizca sertifika hem
// SUNULMUS hem de CA'ya karsi DOGRULANMISSA doludur — burada ek kripto
// kontrolu gerekmez, sadece CN→agent cozumu. Silinmis agent'in sertifikasi
// AgentByID hatasiyla reddedilir (CRL'siz iptal).
func (s *Server) agentFromClientCert(r *http.Request) *store.Agent {
	if s.agentCA == nil || r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		return nil
	}
	id := pki.AgentIDFromCN(r.TLS.PeerCertificates[0].Subject.CommonName)
	if id == 0 {
		return nil
	}
	agent, err := s.store.AgentByID(id)
	if err != nil {
		return nil
	}
	return agent
}

// resolveEnrollToken, sunulan X-Enroll-Token'i dogrular ve token'a BAGLI
// site'i dondurur (ok=false ise gecersiz/iptal/suresi dolmus). Kaynaklar:
//   - hub'in TEK statik sirri (-enroll-token): sabit-zamanli karsilastirma;
//     site "" — agent kendi hello.Site beyanini kullanir (bootstrap yolu)
//   - DB enroll_tokens kaydi: hash arama; site doluysa o site BAGLAYICIDIR
//     (A3 — agent istedigi site'i beyan edip filo icinde konum secemez)
func (s *Server) resolveEnrollToken(presented string) (site string, ok bool) {
	if presented == "" {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(presented), []byte(s.enrollToken)) == 1 {
		return "", true
	}
	t, err := s.store.EnrollTokenByHash(store.TokenHash(presented))
	if err != nil || t.Revoked {
		return "", false
	}
	if t.ExpiresAt > 0 && time.Now().Unix() > t.ExpiresAt {
		return "", false
	}
	go func() { _ = s.store.TouchEnrollToken(t.ID) }() // best-effort, akisi bloklamaz
	return t.Site, true
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// handleAgentHello, enrollment: X-Enroll-Token dogrulamasi ile agent kaydeder
// ve kalici agent token'i dondurur.
func (s *Server) handleAgentHello(w http.ResponseWriter, r *http.Request) {
	if s.enrollToken == "" {
		unauthorized(w, "enrollment kapalı: hub'i -enroll-token ile başlatın")
		return
	}
	ip := clientIP(r)
	if !s.enrollAttempts.allow(ip) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": "çok fazla başarısız enrollment denemesi, bir dakika bekleyin"})
		return
	}
	tokenSite, ok := s.resolveEnrollToken(r.Header.Get("X-Enroll-Token"))
	if !ok {
		s.enrollAttempts.recordFailure(ip)
		unauthorized(w, "geçersiz enrollment token")
		return
	}
	// S14.B1: çoklu-saha modunda site sert bir yetki sınırıdır — site'siz
	// (statik veya site'siz DB) token ile kayıt reddedilir. Panelde site-bağlı
	// bir enroll token üretin (Yönetim > Agent Ekle).
	if s.multiSite && tokenSite == "" {
		s.enrollAttempts.recordFailure(ip)
		unauthorized(w, "çoklu-saha modu: agent kaydı için site-bağlı enroll token gerekli")
		return
	}
	s.enrollAttempts.recordSuccess(ip)
	var hello telemetry.AgentHello
	if err := json.NewDecoder(r.Body).Decode(&hello); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if hello.Name == "" {
		http.Error(w, "name zorunlu", http.StatusBadRequest)
		return
	}
	// Protokol sürümü hub'dan yeniyse: sert reddetmek yerine (S21.15) hub
	// eskisini dayatır. Telgraf JSON ileri/geri uyumlu (omitempty + bilinmeyen
	// alan yok sayılır); agent HubReply.ProtocolVersion'a göre degrade eder.
	effProto := hello.ProtocolVersion
	if effProto > maxProtocolVersion {
		slog.Warn("agent protokol surumu hub'dan yeni — hub eskisini dayatiyor",
			"agent_surumu", hello.ProtocolVersion, "hub_max", maxProtocolVersion, "name", hello.Name)
		effProto = maxProtocolVersion
	}

	buf := make([]byte, 32)
	rand.Read(buf)
	agentToken := hex.EncodeToString(buf)
	displayIP := agentClientIP(r) // XFF guvenilir, yalniz goruntuleme icin — yukaridaki rate-limit ip'siyle karistirilmasin

	// A3: site-kapsamli token'da site sunucu tarafinda baglayicidir; statik
	// veya site'siz token'da agent'in beyani (hello.Site) gecerlidir.
	site := hello.Site
	if tokenSite != "" {
		site = tokenSite
	}

	// C3 (Faz 13): kararlı makine kimliği ile eşleşen çevrimdışı kaydı yeniden
	// kullan (state dosyası kaybında `agents` şişmesin). "Çevrimdışı" penceresi
	// sistemin geri kalanıyla aynı: 2*telemetryInterval.
	offlineBefore := time.Now().Add(-2 * time.Duration(s.telemetryInterval) * time.Second).Unix()
	id, reused, err := s.store.RegisterOrReuseAgent(store.Agent{
		Name:            hello.Name,
		Site:            site,
		TokenHash:       store.TokenHash(agentToken),
		Version:         hello.Version,
		ProtocolVersion: effProto,
		RemoteIP:        displayIP,
		MachineID:       hello.MachineID,
	}, offlineBefore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply := telemetry.HubReply{
		Accepted:                 true,
		AgentID:                  id,
		AgentToken:               agentToken,
		TelemetryIntervalSeconds: s.telemetryInterval,
		PCAPEnabled:              s.agentPCAP,
		ProtocolVersion:          maxProtocolVersion,
	}
	// mTLS: CSR verilmis ve hub CA'si acıksa istemci sertifikasi imzala
	if s.agentCA != nil && hello.CSRPEM != "" {
		certPEM, notAfter, err := s.agentCA.SignAgentCSR([]byte(hello.CSRPEM), id)
		if err != nil {
			slog.Warn("agent CSR imzalanamadi — agent Bearer ile devam eder", "agent_id", id, "err", err)
		} else {
			reply.ClientCertPEM = string(certPEM)
			reply.CACertPEM = string(s.agentCA.CertPEM())
			slog.Info("agent istemci sertifikasi verildi", "agent_id", id, "gecerlilik", notAfter.Format(time.RFC3339))
		}
	}
	slog.Info("agent enroll edildi", "agent_id", id, "name", hello.Name,
		"site", site, "site_token_bagli", tokenSite != "", "ip", displayIP,
		"mtls", reply.ClientCertPEM != "", "kayit_yeniden_kullanildi", reused)
	writeJSON(w, reply)
}

// handleAgentCertRenew, mevcut Bearer/mTLS kimligiyle suresi dolmak uzere
// olan istemci sertifikasini yeniler (agentAuth ile korunur).
func (s *Server) handleAgentCertRenew(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r)
	if agent == nil {
		unauthorized(w, "agent kimliği yok")
		return
	}
	if s.agentCA == nil {
		http.Error(w, "mTLS kapalı", http.StatusNotFound)
		return
	}
	var req telemetry.CertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CSRPEM == "" {
		http.Error(w, "csr_pem gerekli", http.StatusBadRequest)
		return
	}
	certPEM, notAfter, err := s.agentCA.SignAgentCSR([]byte(req.CSRPEM), agent.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slog.Info("agent istemci sertifikasi yenilendi", "agent_id", agent.ID, "gecerlilik", notAfter.Format(time.RFC3339))
	writeJSON(w, telemetry.CertReply{ClientCertPEM: string(certPEM), CACertPEM: string(s.agentCA.CertPEM())})
}

// handleAgentTelemetry, batch veriyi kaydeder ve last_seen gunceller.
// Kuyruk (ingest sink) yapilandirilmis ise batch JetStream'e yayinlanir ve
// yazim arka planda store-writer processor'u tarafindan yapilir (Faz 4.2).
func (s *Server) handleAgentTelemetry(w http.ResponseWriter, r *http.Request) {
	agent := agentFromCtx(r)
	if agent == nil {
		unauthorized(w, "agent kimliği yok")
		return
	}
	var batch telemetry.TelemetryBatch
	dec := time.Now()
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	appmetrics.ObserveTelemetryDecode(dec)
	ts := telemetry.ClampTS(batch.TS, time.Now().Unix())
	ip := agentClientIP(r)

	// Surum batch ile gelir (kayitli agent hello'yu atladigi icin enrollment'taki
	// deger guncellenemez); bos ise eski agent'tir, kayitli degeri koru.
	ver := batch.Version
	if ver == "" {
		ver = agent.Version
	}

	if s.ingest != nil {
		if err := s.ingest.PublishTelemetry(agent.ID, ver, ip, ts, &batch); err != nil {
			slog.Error("telemetri kuyrugu yayinlama hatasi", "agent_id", agent.ID, "err", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		slog.Debug("telemetri kuyruga alindi", "agent_id", agent.ID, "ifaces", len(batch.Interfaces))
		writeJSON(w, s.telemetryReply())
		return
	}

	if err := s.store.SaveIfaceSamples(agent.ID, ts, batch.Interfaces); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.ReplaceConnLatest(agent.ID, batch.Connections); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.TouchAgent(agent.ID, ver, batch.ProtocolVersion, ip); err != nil {
		slog.Error("agent touch hatasi", "agent_id", agent.ID, "err", err)
	}
	if batch.AttrMethod != "" {
		if err := s.store.SetAgentAttrMethod(agent.ID, batch.AttrMethod); err != nil {
			slog.Error("agent attr_method kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	if len(batch.ProcessTraffic) > 0 {
		if err := s.store.SaveProcessTraffic(agent.ID, ts, batch.ProcessTraffic); err != nil {
			slog.Error("surec trafik kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	if len(batch.L7) > 0 {
		if err := s.store.SaveL7(agent.ID, ts, batch.L7); err != nil {
			slog.Error("L7 kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	if len(batch.DNS) > 0 {
		if err := s.store.SaveAgentDNS(agent.ID, ts, batch.DNS); err != nil {
			slog.Error("agent DNS kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	if len(batch.Subnets) > 0 {
		// topoloji kesfi (Faz 6.1): agent'in yerel aglari
		if err := s.store.SaveAgentSubnets(agent.ID, agent.Name, batch.Subnets); err != nil {
			slog.Error("subnet kaydi hatasi", "agent_id", agent.ID, "err", err)
		}
	}
	slog.Debug("telemetri alindi", "agent_id", agent.ID, "ifaces", len(batch.Interfaces), "conns", len(batch.Connections))
	writeJSON(w, s.telemetryReply())
}

// telemetryReply, telemetri gonderimine verilen standart yanittir. Agent
// enrollment'i tekrarlamadigi icin (kayitli agent hello'yu atlar) guncel hub
// politikasi — interval + PCAP izni — agent'a her gonderimde buradan iletilir.
func (s *Server) telemetryReply() telemetry.TelemetryReply {
	pcap := s.agentPCAP
	return telemetry.TelemetryReply{OK: true, Interval: s.telemetryInterval, PCAPEnabled: &pcap}
}

// scopedAgentQuery, L7/DNS/Processes uclarinin ortak on isi: pencere, agent_id,
// limit ve RBAC site scope'unu cozer. agent_id verilmis ama site-disi ise
// ok=false (404 yaz). Donen site parametreli store cagrisina gecer.
func (s *Server) scopedAgentQuery(w http.ResponseWriter, r *http.Request) (since time.Time, agentID int64, limit int, site string, ok bool) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	agentID, _ = strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	site = SiteScope(identityFromCtx(r))
	if agentID > 0 && site != "" {
		a, err := s.store.AgentByID(agentID)
		if err != nil || a.Site != site {
			http.Error(w, "agent bulunamadi", http.StatusNotFound)
			return since, 0, 0, "", false
		}
	}
	return time.Now().Add(-time.Duration(minutes) * time.Minute), agentID, limit, site, true
}

// handleL7, surec bazli uygulama gorunurlugu (SNI + HTTP Host) top-listesi.
func (s *Server) handleL7(w http.ResponseWriter, r *http.Request) {
	since, agentID, limit, site, ok := s.scopedAgentQuery(w, r)
	if !ok {
		return
	}
	list, err := s.store.TopL7(since, agentID, limit, site)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleAgentDNS, surec bazli DNS gorunurlugu top-listesi.
func (s *Server) handleAgentDNS(w http.ResponseWriter, r *http.Request) {
	since, agentID, limit, site, ok := s.scopedAgentQuery(w, r)
	if !ok {
		return
	}
	list, err := s.store.TopAgentDNS(since, agentID, limit, site)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleProcesses, surec bazli trafik top-listesi (UI auth ile korunur).
func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	since, agentID, limit, site, ok := s.scopedAgentQuery(w, r)
	if !ok {
		return
	}
	list, err := s.store.TopProcessTraffic(since, agentID, limit, site)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleAgentsList, UI icin filo gorunumu (UI auth'u ile korunur).
// Site-sinirli kimlikte (RBAC site scope) yalnizca kendi sitesi doner.
func (s *Server) handleAgentsList(w http.ResponseWriter, r *http.Request) {
	window := time.Duration(2*s.telemetryInterval) * time.Second
	agents, err := s.store.ListAgents(window, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONETag(w, r, agents) // D4: sık pollanır
}

// agentInScope, site-sinirli kimligin verilen agent'i gorup goremeyecegi.
func (s *Server) agentInScope(r *http.Request, a *store.Agent) bool {
	scope := SiteScope(identityFromCtx(r))
	return scope == "" || (a != nil && a.Site == scope)
}

func (s *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	agent, err := s.store.AgentByID(id)
	if err != nil || !s.agentInScope(r, agent) {
		http.Error(w, "agent bulunamadi", http.StatusNotFound)
		return
	}
	window := time.Duration(2*s.telemetryInterval) * time.Second
	agents, _ := s.store.ListAgents(window, SiteScope(identityFromCtx(r)))
	var withRates *store.AgentWithRates
	for i := range agents {
		if agents[i].ID == id {
			withRates = &agents[i]
			break
		}
	}
	if withRates == nil {
		withRates = &store.AgentWithRates{Agent: *agent, Online: false}
	}
	withRates.Conns = len(s.store.LatestAgentConnections(id))
	writeJSON(w, map[string]any{
		"agent":       withRates,
		"connections": s.store.LatestAgentConnections(id),
	})
}

// handleProcessDetail, tek bir sürecin (Agent → Süreç Trafiği → Enter) tüm ağ
// etkinliğini toplar: kimlik/özet + uzak hedefler + canlı bağlantılar +
// uygulama görünürlüğü (DNS/SNI/Host) + zaman çizelgesi (Faz 23-A). Yeni
// telemetri hattı yok — mevcut tablolar sunucu-tarafı toplanır. UI auth +
// RBAC site scope (agentInScope).
func (s *Server) handleProcessDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	agent, err := s.store.AgentByID(id)
	if err != nil || !s.agentInScope(r, agent) {
		http.Error(w, "agent bulunamadi", http.StatusNotFound)
		return
	}
	process := r.PathValue("process")
	if process == "" {
		http.Error(w, "süreç adı gerekli", http.StatusBadRequest)
		return
	}
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)

	summary, err := s.store.ProcessSummary(id, process, since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if summary.FirstSeen == 0 {
		http.Error(w, "süreç bu pencerede görülmedi", http.StatusNotFound)
		return
	}
	remotes, err := s.store.ProcessRemotes(id, process, since, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	app, err := s.store.ProcessAppVisibility(id, process, since, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	timeline, err := s.store.ProcessTimeline(id, agent.Name, process, since, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// canlı bağlantılar: son telemetri anındaki soket tablosundan süreç süzülür.
	conns := make([]telemetry.ConnectionSample, 0)
	for _, c := range s.store.LatestAgentConnections(id) {
		if c.Process == process {
			conns = append(conns, c)
		}
	}

	// uzak uçları zenginleştir: canlı bağlantı sayısı + (varsa) GeoIP/ASN.
	// 23-E `internal/enrich` gelene kadar opportunistik.
	for i := range remotes {
		for _, c := range conns {
			if connRemoteHost(c.RemoteAddr) == remotes[i].RemoteIP {
				remotes[i].Conns++
			}
		}
		if s.geo != nil {
			info := s.geo.Lookup(remotes[i].RemoteIP)
			remotes[i].Country = info.Country
			remotes[i].ASN = info.ASN
		}
	}

	writeJSON(w, map[string]any{
		"process":        summary,
		"remotes":        remotes,
		"connections":    conns,
		"app_visibility": app,
		"timeline":       timeline,
	})
}

// connRemoteHost, "host:port" (veya "[v6]:port") biçimindeki uzak adresten
// yalnız host kısmını döndürür; ayrıştıramazsa girdiyi olduğu gibi verir.
func connRemoteHost(addr string) string {
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// handleAgentHistory, Agent Detay sayfasindaki throughput grafigi icin
// zaman serisi dondurur (ThroughputChart ile ayni Bucket semasi).
func (s *Server) handleAgentHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if a, err := s.store.AgentByID(id); err != nil || !s.agentInScope(r, a) {
		http.Error(w, "agent bulunamadi", http.StatusNotFound)
		return
	}
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		minutes = 60
	}
	buckets, err := s.store.AgentHistory(id, time.Now().Add(-time.Duration(minutes)*time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, buckets)
}

func (s *Server) handleAgentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	// S14.B2: saha-kısıtlı kimlik yalnız kendi sahasının agent'ını silebilir.
	if a, err := s.store.AgentByID(id); err != nil || !s.agentInScope(r, a) {
		http.Error(w, "agent bulunamadı", http.StatusNotFound)
		return
	}
	if err := s.store.DeleteAgent(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("agent silindi", "agent_id", id)
	s.audit(r, identityFromCtx(r), "agent.delete", fmt.Sprintf("agent:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

// handleAgentRename, agent'a hub uzerinden yeni bir goruntu adi atar (enroll
// sirasinda -name ile verilen ad yerine gecer; agent tarafinda hicbir sey
// degismez, yalnizca kayit defterindeki isim guncellenir).
func (s *Server) handleAgentRename(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		http.Error(w, "isim boş olamaz", http.StatusBadRequest)
		return
	}
	if len(name) > 128 {
		http.Error(w, "isim çok uzun (maksimum 128 karakter)", http.StatusBadRequest)
		return
	}
	// S14.B2: saha-kısıtlı kimlik yalnız kendi sahasının agent'ını adlandırabilir.
	if a, err := s.store.AgentByID(id); err != nil || !s.agentInScope(r, a) {
		http.Error(w, "agent bulunamadı", http.StatusNotFound)
		return
	}
	if err := s.store.RenameAgent(id, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("agent yeniden adlandirildi", "agent_id", id, "name", name)
	s.audit(r, identityFromCtx(r), "agent.rename", fmt.Sprintf("agent:%d", id), name)
	writeJSON(w, map[string]any{"ok": true, "name": name})
}

// handleAgentSetUplink, agent'ı bir erişim katmanı cihazına (switch/AP) bağlar
// veya bağı kaldırır — Canlı Akış gruplama (yönetici manuel atar). Gövde:
// {"device_id": <id>} veya {"device_id": null}.
func (s *Server) handleAgentSetUplink(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	var body struct {
		DeviceID *int64 `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	a, err := s.store.AgentByID(id)
	if err != nil || !s.agentInScope(r, a) {
		http.Error(w, "agent bulunamadı", http.StatusNotFound)
		return
	}
	if body.DeviceID != nil {
		d, err := s.store.DeviceByID(*body.DeviceID)
		if err != nil {
			http.Error(w, "cihaz bulunamadı", http.StatusBadRequest)
			return
		}
		switch d.Kind {
		case "switch", "ap", "router", "firewall":
		default:
			http.Error(w, "uplink yalnızca switch/ap/router/firewall türü olabilir", http.StatusBadRequest)
			return
		}
		if scope := SiteScope(identityFromCtx(r)); scope != "" && d.Site != scope {
			http.Error(w, "cihaz bu sahada değil", http.StatusBadRequest)
			return
		}
		if a.Site != "" && d.Site != "" && a.Site != d.Site {
			http.Error(w, "agent ve cihaz farklı sahalarda", http.StatusBadRequest)
			return
		}
	}
	if err := s.store.SetAgentUplink(id, body.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	detail := "kaldırıldı"
	if body.DeviceID != nil {
		detail = fmt.Sprintf("device:%d", *body.DeviceID)
	}
	slog.Info("agent uplink atandı", "agent_id", id, "uplink", detail)
	s.audit(r, identityFromCtx(r), "agent.uplink", fmt.Sprintf("agent:%d", id), detail)
	writeJSON(w, map[string]any{"ok": true})
}

// maxProtocolVersion, desteklenen en yuksek agent protokol surumu.
const maxProtocolVersion = 1

func parseID(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}
