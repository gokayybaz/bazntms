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

// session, kimlik tasiran oturumdur (Faz 5.1).
type session struct {
	ident Identity
	exp   time.Time
}

// AuthManager, oturum ve kimlik denetimidir. Uc giris yolu vardir:
//   - legacy: tek sifre (-auth-password) → admin kimligi (geriye uyumlu)
//   - user:   users tablosu (bcrypt) → rol + site scope kimligi
//   - token:  api_tokens tablosu (Bearer) → entegrasyon kimligi
//
// Oturumlar bellekte tutulur (7 gun) — sunucu yeniden baslayinca tekrar
// giris gerekir.
type AuthManager struct {
	mu       sync.Mutex
	password string
	st       store.Store
	users    bool // users tablosunda kayit var mi (ilk kontrolde ogrenilir)
	sessions map[string]*session
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
		sessions: map[string]*session{},
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

func (a *AuthManager) UsersExist() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.users
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
	a.sessions[token] = &session{ident: *ident, exp: time.Now().Add(sessionTTL)}
	a.pruneLocked()
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
	a.sessions[token] = &session{ident: *ident, exp: time.Now().Add(sessionTTL)}
	a.pruneLocked()

	go a.st.TouchUserLogin(u.ID) // son giris zamani (hatasiz olsun diye sessiz)
	return token, ident, true, false
}

// IdentityForToken, Bearer token'i cozer: once oturum, sonra API token'i.
// API token'lari DB'den dogrulanir; hash anahtariyla (sha256 hex) saklanir.
func (a *AuthManager) IdentityForToken(token string) *Identity {
	if a == nil || token == "" {
		return nil
	}
	a.mu.Lock()
	if sess, ok := a.sessions[token]; ok {
		ident := sess.ident
		a.mu.Unlock()
		return &ident
	}
	a.mu.Unlock()

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
	go a.st.TouchAPIToken(t.ID)
	return &Identity{Username: t.Name, Role: role, Site: t.Site, Kind: "token"}
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
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
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

// pruneLocked, suresi gecmis oturumlari temizler (mu kilitliyken cagir).
func (a *AuthManager) pruneLocked() {
	now := time.Now()
	for t, sess := range a.sessions {
		if now.After(sess.exp) {
			delete(a.sessions, t)
		}
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
func (s *Server) audit(r *http.Request, id *Identity, action, target, detail string) {
	if s.store == nil {
		return
	}
	ident := id
	if ident == nil {
		ident = &Identity{Username: "-", Role: RoleViewer, Kind: "-"}
	}
	_, err := s.store.InsertAuditEvent(store.AuditEvent{
		Username: ident.Username, Role: string(ident.Role),
		Action: action, Target: target, Detail: detail,
		IP: clientIP(r),
	})
	if err != nil {
		slog.Error("denetim kaydi yazilamadi", "action", action, "err", err)
	}
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
			s.audit(r, nil, "login.failed", "user:"+req.Username, "hatali kullanici girisi")
		}
	} else {
		token, ident, ok, blocked = s.auth.Login(req.Password, ip)
		if ok {
			s.audit(r, ident, "login", "legacy", "tek sifre girisi (admin)")
		} else {
			s.audit(r, nil, "login.failed", "legacy", "hatali sifre")
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
		if blocked {
			msg = "çok fazla deneme yapıldı, bir dakika bekleyin"
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
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, map[string]any{"ok": true, "token": token, "role": string(ident.Role), "username": ident.Username})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if id := identityFromCtx(r); id != nil {
		s.audit(r, id, "logout", "user:"+id.Username, "")
	}
	s.auth.LogoutRequest(r)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	ident := s.auth.Identity(r)
	resp := map[string]any{
		"required":      s.auth.Enabled(),
		"authenticated": ident != nil,
		"oidc":          s.oidc.Enabled(),
	}
	if ident != nil {
		resp["username"] = ident.Username
		resp["role"] = string(ident.Role)
		resp["site"] = ident.Site
		resp["kind"] = ident.Kind
	}
	writeJSON(w, resp)
}
