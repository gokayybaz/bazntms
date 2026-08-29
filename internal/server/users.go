package server

// Kullanici, API token ve denetim kaydi yonetim uclari (Faz 5).
// Hepsi PermAdmin (admin rolü) ile korunur; her islem audit zincire yazilir.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"github.com/gokayybaz/bazntms/internal/store"
)

func newAPITokenValue() string {
	buf := make([]byte, 24)
	rand.Read(buf)
	return "bnt_" + hex.EncodeToString(buf)
}

// --- kullanicilar ---

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, users)
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Site     string `json:"site"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Username == "" || len(req.Password) < 8 {
		http.Error(w, "username zorunlu, sifre en az 8 karakter", http.StatusBadRequest)
		return
	}
	role := Role(req.Role)
	if !role.Valid() {
		role = RoleViewer
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, err := s.store.CreateUser(store.User{
		Username: req.Username, PasswordHash: string(hash),
		Role: string(role), Site: req.Site, Enabled: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "user.create", "user:"+req.Username, "rol: "+string(role)+" site: "+req.Site)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	u, err := s.store.UserByID(id)
	if err != nil {
		http.Error(w, "kullanıcı bulunamadı", http.StatusNotFound)
		return
	}
	var req struct {
		Role     *string `json:"role"`
		Site     *string `json:"site"`
		Enabled  *bool   `json:"enabled"`
		Password *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// son admin kilidi: admin rolunu/superuser'i etkisizlestirme korumasi
	if u.Role == string(RoleAdmin) && id == s.currentAdminID(r) {
		if req.Enabled != nil && !*req.Enabled {
			http.Error(w, "kendi hesabınızı devre dışı bırakamazsınız", http.StatusBadRequest)
			return
		}
	}
	if req.Role != nil {
		if !Role(*req.Role).Valid() {
			http.Error(w, "geçersiz rol", http.StatusBadRequest)
			return
		}
		u.Role = *req.Role
	}
	if req.Site != nil {
		u.Site = *req.Site
	}
	if req.Enabled != nil {
		u.Enabled = *req.Enabled
	}
	if err := s.store.UpdateUser(*u); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.Password != nil {
		if len(*req.Password) < 8 {
			http.Error(w, "sifre en az 8 karakter", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.store.UpdateUserPassword(id, string(hash)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.audit(r, identityFromCtx(r), "user.update", "user:"+u.Username, "rol: "+u.Role)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if id == s.currentAdminID(r) {
		http.Error(w, "kendi hesabınızı silemezsiniz", http.StatusBadRequest)
		return
	}
	u, err := s.store.UserByID(id)
	if err != nil {
		http.Error(w, "kullanıcı bulunamadı", http.StatusNotFound)
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "user.delete", "user:"+u.Username, "")
	writeJSON(w, map[string]any{"ok": true})
}

// currentAdminID, isteği yapan kullanıcının users tablosundaki ID'sini
// bulur (kendi hesabını kilitleme koruması için); legacy/oidc'de 0.
func (s *Server) currentAdminID(r *http.Request) int64 {
	id := identityFromCtx(r)
	if id == nil || id.Kind != "user" {
		return 0
	}
	if u, err := s.store.UserByName(id.Username); err == nil {
		return u.ID
	}
	return 0
}

// --- API token'lari ---

func (s *Server) handleTokensList(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListAPITokens()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokens)
}

func (s *Server) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
		Site string `json:"site"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name zorunlu", http.StatusBadRequest)
		return
	}
	role := Role(req.Role)
	if !role.Valid() {
		role = RoleViewer
	}
	plain := newAPITokenValue()
	id, err := s.store.CreateAPIToken(store.APIToken{
		Name: req.Name, TokenHash: TokenHashString(plain),
		Role: string(role), Site: req.Site,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "token.create", "token:"+req.Name, "rol: "+string(role))
	// duz token YALNIZCA bir kez doner (hash saklanir)
	writeJSON(w, map[string]any{"ok": true, "id": id, "token": plain})
}

func (s *Server) handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if err := s.store.RevokeAPIToken(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "token.revoke", "token:"+strconv.FormatInt(id, 10), "")
	writeJSON(w, map[string]any{"ok": true})
}

// --- denetim kaydi ---

func (s *Server) handleAuditList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.RecentAuditEvents(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}

func (s *Server) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	ok, brokenAt, checked, err := s.store.VerifyAuditChain()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": ok, "broken_at": brokenAt, "checked": checked})
}
