package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gokayybaz/bazntms/internal/store"
)

const (
	sessionTTL    = 7 * 24 * time.Hour
	sessionCookie = "nm_session"
	maxAttempts   = 5
	attemptWindow = time.Minute
)

// AuthManager, oturum ve kimlik denetimidir. Uc giris yolu vardir:
//   - legacy: tek sifre (-auth-password) → admin kimligi (geriye uyumlu)
//   - user:   users tablosu (bcrypt) → rol + site scope kimligi
//   - token:  api_tokens tablosu (Bearer) → entegrasyon kimligi
//
// Oturumlar SessionStore arkasinda: varsayilan bellek-ici (tek replika),
// -session-store=db ile Postgres (coklu replika, A4 / Faz 15). Oturum anahtari
// sha256(cerez token'i) — ham token depoda tutulmaz.
type AuthManager struct {
	mu       sync.Mutex
	password string
	st       store.Store
	users    bool // users tablosunda kayit var mi (ilk kontrolde ogrenilir)
	sessions SessionStore
	attempts map[string]*attemptLog
}

type attemptLog struct {
	count int
	reset time.Time
	block time.Time
}

func NewAuthManager(password string, st store.Store) *AuthManager {
	a := &AuthManager{
		password: password,
		st:       st,
		sessions: newMemSessionStore(),
		attempts: map[string]*attemptLog{},
	}
	if st != nil {
		if users, err := st.ListUsers(); err == nil {
			a.users = len(users) > 0
		}
	}
	if password == "" && !a.users {
		return nil // kimlik dogrulama kapali (dev modu)
	}
	return a
}

// Enabled, kimlik dogrulama aktif mi?
func (a *AuthManager) Enabled() bool { return a != nil }

// SetSessionStore, oturum deposunu degistirir (ilk istekten once cagrilmali —
// -session-store=db icin srv.UseDBSessions).
func (a *AuthManager) SetSessionStore(s SessionStore) {
	if a != nil && s != nil {
		a.sessions = s
	}
}

func (a *AuthManager) UsersExist() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.users
}

// LegacyLoginDisabled, legacy tek-sifre girisinin devre disi olup olmadigini
// dondurur: etkin bir RBAC admin'i varsa legacy sifre kapalidir (B6).
func (a *AuthManager) LegacyLoginDisabled() bool {
	if a == nil || a.st == nil {
		return false
	}
	exists, _ := a.st.AdminUserExists()
	return exists
}

// LegacyPasswordSet, -auth-password (veya AUTH_PASSWORD) ile bir legacy
// sifre yapilandirilmis mi. "Son admin" korumasi bunu bir kurtarma yolu
// (break-glass) olarak sayar (B6).
func (a *AuthManager) LegacyPasswordSet() bool {
	return a != nil && a.password != ""
}

func newSessionToken() string {
	buf := make([]byte, 32)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

// Login, legacy tek sifre ile giris: admin kimligi dondurur.
// blocked=true, ayni IP cok fazla hatali deneme yaptigini gosterir.
func (a *AuthManager) Login(password, clientIP string) (string, *Identity, bool, bool) {
	if a == nil {
		return "", &Identity{Username: "anonim", Role: RoleAdmin, Kind: "legacy"}, true, false
	}
	// B6: etkin bir RBAC admin'i varsa legacy tek-sifre girisi devre disi
	// (rate-limit'e sayilmaz — sifre denemesi degil, politika reddi).
	if a.LegacyLoginDisabled() {
		slog.Warn("legacy -auth-password girisi reddedildi — RBAC aktif (etkin admin kullanici var)", "ip", clientIP)
		return "", nil, false, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.allowAttempt(clientIP) {
		return "", nil, false, true
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		a.recordFailure(clientIP)
		return "", nil, false, false
	}
	delete(a.attempts, clientIP)

	token := newSessionToken()
	ident := &Identity{Username: "admin", Role: RoleAdmin, Kind: "legacy"}
	_ = a.sessions.Put(TokenHashString(token), *ident, time.Now().Add(sessionTTL))
	return token, ident, true, false
}

// LoginUser, users tablosundaki hesapla giris (bcrypt).
func (a *AuthManager) LoginUser(username, password, clientIP string) (string, *Identity, bool, bool) {
	if a == nil {
		return "", nil, false, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.allowAttempt(clientIP) {
		return "", nil, false, true
	}

	u, err := a.st.UserByName(username)
	if err != nil || !u.Enabled || u.PasswordHash == "" ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		a.recordFailure(clientIP)
		return "", nil, false, false
	}
	delete(a.attempts, clientIP)
	a.users = true

	token := newSessionToken()
	ident := &Identity{Username: u.Username, Role: Role(u.Role), Site: u.Site, Kind: "user"}
	if !ident.Role.Valid() {
		ident.Role = RoleViewer
	}
	sanitizeIdentity(ident)
	_ = a.sessions.Put(TokenHashString(token), *ident, time.Now().Add(sessionTTL))

	go func() { _ = a.st.TouchUserLogin(u.ID) }() // son giris zamani (best-effort)
	return token, ident, true, false
}

// IdentityForToken, Bearer token'i cozer: once oturum, sonra API token'i.
// API token'lari DB'den dogrulanir; hash anahtariyla (sha256 hex) saklanir.
func (a *AuthManager) IdentityForToken(token string) *Identity {
	if a == nil || token == "" {
		return nil
	}
	if ident, ok := a.sessions.Get(TokenHashString(token)); ok {
		// Derinlik savunması: oturum yazımda zaten sanitize edilir, ama
		// tutarsız (elle düzenlenmiş / gelecekte eksik yazılmış) bir kayıt
		// site-admin'i fiilen global yapmasın (site'siz site-admin → viewer).
		return sanitizeIdentity(ident)
	}

	if a.st == nil {
		return nil
	}
	t, err := a.st.APITokenByHash(TokenHashString(token))
	if err != nil || t.Revoked {
		return nil
	}
	role := Role(t.Role)
	if !role.Valid() {
		role = RoleViewer
	}
	go func() { _ = a.st.TouchAPIToken(t.ID) }()
	return sanitizeIdentity(&Identity{Username: t.Name, Role: role, Site: t.Site, Kind: "token"})
}

func TokenHashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Valid, istekteki oturumun gecerli olup olmadigini kontrol eder.
// Tarayicilar cookie (WS handshake'i dahil), scriptler Bearer token kullanir.
func (a *AuthManager) Valid(r *http.Request) bool {
	return a.Identity(r) != nil
}

// Identity, istekle iliskili kimligi dondurur (yoksa nil).
func (a *AuthManager) Identity(r *http.Request) *Identity {
	if a == nil {
		return nil
	}
	token := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		token = c.Value
	}
	if token == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token == "" {
		return nil
	}
	return a.IdentityForToken(token)
}

// Logout, oturumu iptal eder.
func (a *AuthManager) Logout(token string) {
	if a == nil {
		return
	}
	_ = a.sessions.Delete(TokenHashString(token))
}

// LogoutCookie, request'teki cookie/bearer oturumunu kapatir.
func (a *AuthManager) LogoutRequest(r *http.Request) {
	if a == nil {
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.Logout(c.Value)
	}
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
		a.Logout(strings.TrimPrefix(ah, "Bearer "))
	}
}

func (a *AuthManager) allowAttempt(ip string) bool {
	l, ok := a.attempts[ip]
	if !ok {
		return true
	}
	if time.Now().Before(l.block) {
		return false
	}
	if time.Since(l.reset) > attemptWindow {
		l.count = 0
		l.reset = time.Now()
	}
	return l.count < maxAttempts
}

func (a *AuthManager) recordFailure(ip string) {
	l, ok := a.attempts[ip]
	if !ok {
		l = &attemptLog{reset: time.Now()}
		a.attempts[ip] = l
	}
	l.count++
	if l.count >= maxAttempts {
		l.block = time.Now().Add(attemptWindow)
		l.count = 0
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// authMiddleware, /api/* ve /ws isteklerini oturum denetiminden gecirir;
// basarili kimligi context'e koyar. Statik dosyalar (SPA kabugu) acik kalir.
// Login uclari ve agent Bearer yolu her zaman aciktir (agent auth ayri).
func (a *AuthManager) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		// agent uclari UI auth'undan muaf: kendi Bearer agent auth'unu kullanir
		if path == "/api/login" || path == "/api/auth/status" ||
			path == "/api/openapi.yaml" || path == "/api/openapi.json" || path == "/api/docs" ||
			strings.HasPrefix(path, "/api/auth/oidc/") ||
			strings.HasPrefix(path, "/api/v1/agent/") || !requiresAuth(path) {
			next.ServeHTTP(w, r)
			return
		}
		if ident := a.Identity(r); ident != nil {
			next.ServeHTTP(w, r.WithContext(contextWithIdentity(r, ident)))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "oturum gerekli", "auth_required": true})
	})
}

func requiresAuth(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/ws"
}

// audit, denetim kaydini zincire ekler; hata olursa loglar, akisi bozmaz.
// result varsayilan "ok"; before/after nil → durum farki yazilmaz.
func (s *Server) audit(r *http.Request, id *Identity, action, target, detail string) {
	s.auditEvent(r, id, action, target, detail, auditExtra{})
}

// auditDiff, islem oncesi/sonrasi durumu da yazar (Faz 25-C). before/after
// redactAuditJSON'dan gecirilir — sir alanlari "•••" maskelenir.
func (s *Server) auditDiff(r *http.Request, id *Identity, action, target, detail string, before, after any) {
	s.auditEvent(r, id, action, target, detail, auditExtra{
		result: "ok",
		before: redactAuditJSON(before),
		after:  redactAuditJSON(after),
	})
}

// auditResult, sonucu acikca belirtir ("error" | "denied") — basarisiz
// yetkilendirme / giris denemeleri icin.
func (s *Server) auditResult(r *http.Request, id *Identity, action, target, detail, result string) {
	s.auditEvent(r, id, action, target, detail, auditExtra{result: result})
}

type auditExtra struct {
	result string
	before string
	after  string
}

func (s *Server) auditEvent(r *http.Request, id *Identity, action, target, detail string, x auditExtra) {
	if s.store == nil {
		return
	}
	ident := id
	if ident == nil {
		ident = &Identity{Username: "-", Role: RoleViewer, Kind: "-"}
	}
	result := x.result
	if result == "" {
		result = "ok"
	}
	actorType := ident.Kind
	if actorType == "" {
		actorType = "-"
	}
	ev := store.AuditEvent{
		Username: ident.Username, Role: string(ident.Role), Site: ident.Site,
		Action: action, Target: target, Detail: detail,
		IP:         clientIP(r),
		ActorType:  actorType,
		Result:     result,
		BeforeJSON: x.before,
		AfterJSON:  x.after,
	}
	if r != nil {
		ev.RequestID = requestIDFromCtx(r)
		ev.UserAgent = r.UserAgent()
	}
	if _, err := s.store.InsertAuditEvent(ev); err != nil {
		slog.Error("denetim kaydi yazilamadi", "action", action, "err", err)
	}
}

// --- denetim durum farki maskeleme (Faz 25-C) ---

// redactAuditJSON, bir degeri denetim kaydi icin JSON'a cevirir ve sir iceren
// alan adlarini ("password", "token", "secret", "community", …) "•••" ile
// maskeler. nil → "". Nesne map'e cozulup ozyinelemeli taranir.
func redactAuditJSON(v any) string {
	if v == nil {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var m any
	if err := json.Unmarshal(raw, &m); err != nil {
		return string(raw)
	}
	redactAuditWalk(m)
	out, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(out)
}

func redactAuditWalk(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if auditSecretKey(k) {
				if val != nil && val != "" {
					t[k] = secretMask
				}
				continue
			}
			redactAuditWalk(val)
		}
	case []any:
		for _, val := range t {
			redactAuditWalk(val)
		}
	}
}

func auditSecretKey(k string) bool {
	k = strings.ToLower(k)
	for _, s := range []string{
		"password", "passwd", "secret", "token", "apikey", "api_key",
		"passphrase", "community", "authpass", "auth_pass", "privpass",
		"priv_pass", "credential", "private_key", "seed",
	} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// handleLogin, sifre ile oturum acar: username verilmisse users tablosundan
// (RBAC), verilmemisse legacy tek sifre (admin) kullanilir. Cookie + token
// dondurur.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ip := clientIP(r)
	var (
		token   string
		ident   *Identity
		ok      bool
		blocked bool
	)
	if req.Username != "" {
		token, ident, ok, blocked = s.auth.LoginUser(req.Username, req.Password, ip)
		if ok {
			s.audit(r, ident, "login", "user:"+req.Username, "kullanici girisi")
		} else {
			s.auditResult(r, nil, "login.failed", "user:"+req.Username, "hatali kullanici girisi", "error")
		}
	} else {
		token, ident, ok, blocked = s.auth.Login(req.Password, ip)
		if ok {
			s.audit(r, ident, "login", "legacy", "tek sifre girisi (admin)")
		} else {
			s.auditResult(r, nil, "login.failed", "legacy", "hatali sifre", "error")
		}
	}
	if !ok {
		status := http.StatusUnauthorized
		if blocked {
			status = http.StatusTooManyRequests
			w.Header().Set("Retry-After", "60")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		msg := "şifre hatalı"
		switch {
		case blocked:
			msg = "çok fazla deneme yapıldı, bir dakika bekleyin"
		case req.Username == "" && s.auth.LegacyLoginDisabled():
			msg = "tek-şifre girişi kapalı: RBAC etkin — kullanıcı adınızla giriş yapın"
		}
		json.NewEncoder(w).Encode(map[string]any{"error": msg})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, map[string]any{"ok": true, "token": token, "role": string(ident.Role), "username": ident.Username, "site": ident.Site})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if id := identityFromCtx(r); id != nil {
		s.audit(r, id, "logout", "user:"+id.Username, "")
	}
	s.auth.LogoutRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	ident := s.auth.Identity(r)
	resp := map[string]any{
		"required":      s.auth.Enabled(),
		"authenticated": ident != nil,
		"oidc":          s.oidc.Enabled(),
		"multi_site":    s.multiSite,
		"public_url":    s.publicURL, // agent kurulum sihirbazı için (boş = origin kullan)
	}
	if ident != nil {
		resp["username"] = ident.Username
		resp["role"] = string(ident.Role)
		resp["site"] = ident.Site
		resp["kind"] = ident.Kind
	}
	writeJSON(w, resp)
}
