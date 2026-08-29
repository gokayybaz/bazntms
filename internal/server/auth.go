package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionTTL    = 7 * 24 * time.Hour
	sessionCookie = "nm_session"
	maxAttempts   = 5
	attemptWindow = time.Minute
)

// AuthManager, sifre tabanli oturum yonetimi. Sifre yalnizca bayrak/ortam
// degiskeninden gelir; DB'ye yazilmaz. Oturumlar bellekte tutulur (7 gun) —
// sunucu yeniden baslayinca tekrar giris gerekir.
type AuthManager struct {
	mu       sync.Mutex
	password string
	sessions map[string]time.Time // token -> son kullanma
	attempts map[string]*attemptLog
}

type attemptLog struct {
	count int
	reset time.Time
	block time.Time
}

func NewAuthManager(password string) *AuthManager {
	if password == "" {
		return nil // kimlik dogrulama kapali
	}
	return &AuthManager{
		password: password,
		sessions: map[string]time.Time{},
		attempts: map[string]*attemptLog{},
	}
}

// Enabled, kimlik dogrulama aktif mi?
func (a *AuthManager) Enabled() bool { return a != nil }

// Login, sifreyi dogrular; basariliysa yeni oturum token'i uretir.
// blocked=true, ayni IP cok fazla hatali deneme yaptigini gosterir.
func (a *AuthManager) Login(password, clientIP string) (token string, ok bool, blocked bool) {
	if a == nil {
		return "", true, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.allowAttempt(clientIP) {
		return "", false, true
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		a.recordFailure(clientIP)
		return "", false, false
	}
	delete(a.attempts, clientIP)

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, false
	}
	token = hex.EncodeToString(buf)
	a.sessions[token] = time.Now().Add(sessionTTL)
	a.pruneLocked()
	return token, true, false
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

// Valid, istekteki oturumun gecerli olup olmadigini kontrol eder.
// Tarayicilar cookie (WS handshake'i dahil), scriptler Bearer token kullanir.
func (a *AuthManager) Valid(r *http.Request) bool {
	if a == nil {
		return true
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
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(a.sessions, token)
		return false
	}
	return true
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

// pruneLocked, suresi gecmis oturumlari temizler (mu kilitliyken cagir).
func (a *AuthManager) pruneLocked() {
	now := time.Now()
	for t, exp := range a.sessions {
		if now.After(exp) {
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
// statik dosyalar (SPA kabugu) acik kalir. Login ucu her zaman aciktir.
func (a *AuthManager) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		// agent uclari UI auth'undan muaf: kendi Bearer agent auth'unu kullanir
		if path == "/api/login" || path == "/api/auth/status" ||
			strings.HasPrefix(path, "/api/v1/agent/") || !requiresAuth(path) {
			next.ServeHTTP(w, r)
			return
		}
		if a.Valid(r) {
			next.ServeHTTP(w, r)
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

// handleLogin, sifre ile oturum acar; cookie + token dondurur.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, ok, blocked := s.auth.Login(req.Password, clientIP(r))
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
	writeJSON(w, map[string]any{"ok": true, "token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.auth.Logout(c.Value)
	}
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
		s.auth.Logout(strings.TrimPrefix(ah, "Bearer "))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"required":      s.auth.Enabled(),
		"authenticated": s.auth.Valid(r),
	})
}
