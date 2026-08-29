package server

// OIDC SSO (Faz 5.2): Entra ID, Keycloak, Google vb. uyumlu saglayici ile
// giris. Grup/rol claim'i → RBAC rol eslemesi config'ten gelir:
//
//	oidc:
//	  issuer: https://sso.example.com/realms/bazntms
//	  client_id: bazntms
//	  client_secret: ...
//	  group_roles:
//	    bazntms-admin: admin
//	    bazntms-netops: netops
//	  default_role: viewer
//
// Akis: /api/auth/oidc/login → saglayici → /api/auth/oidc/callback →
// id_token dogrulama → oturum (kind=oidc) → "/" yonlendirmesi.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const oidcStateCookie = "nm_oidc_state"

// OIDCOptions, SSO saglayici yapilandirmasidir (config'ten gelir).
type OIDCOptions struct {
	Issuer       string            `koanf:"issuer"`
	ClientID     string            `koanf:"client_id"`
	ClientSecret string            `koanf:"client_secret"`
	RedirectURL  string            `koanf:"redirect_url"` // bos: <scheme>://<host>/api/auth/oidc/callback
	GroupRoles   map[string]string `koanf:"group_roles"`  // grup → rol
	DefaultRole  string            `koanf:"default_role"`
}

// OIDCManager, provider discovery + oauth2 akisini yonetir.
type OIDCManager struct {
	opts     OIDCOptions
	provider *oidc.Provider
	oauth2   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewOIDCManager(opts OIDCOptions) *OIDCManager {
	if opts.Issuer == "" || opts.ClientID == "" {
		return nil
	}
	if opts.DefaultRole == "" {
		opts.DefaultRole = string(RoleViewer)
	}
	return &OIDCManager{opts: opts}
}

// init, provider discovery'yi ilk kullanimda (veya baslangicta) yapar.
func (o *OIDCManager) init() error {
	if o.provider != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, o.opts.Issuer)
	if err != nil {
		return fmt.Errorf("oidc discovery: %w", err)
	}
	o.provider = provider
	o.verifier = provider.Verifier(&oidc.Config{ClientID: o.opts.ClientID})
	o.oauth2 = &oauth2.Config{
		ClientID:     o.opts.ClientID,
		ClientSecret: o.opts.ClientSecret,
		RedirectURL:  o.opts.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		Endpoint:     provider.Endpoint(),
	}
	slog.Info("oidc saglayici hazir", "issuer", o.opts.Issuer, "client_id", o.opts.ClientID)
	return nil
}

// Enabled, SSO yapilandirilmis mi?
func (o *OIDCManager) Enabled() bool { return o != nil }

func newRandomState() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

// handleOIDCLogin, saglayiciya yonlendirir; state cookie CSRF korumasi saglar.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !s.oidc.Enabled() {
		http.Error(w, "OIDC yapilandirilmamis", http.StatusNotFound)
		return
	}
	if err := s.oidc.init(); err != nil {
		slog.Error("oidc saglayici erisilemedi", "err", err)
		http.Error(w, "OIDC saglayiciya erisilemedi", http.StatusBadGateway)
		return
	}
	state := newRandomState()
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookie, Value: state, Path: "/api/auth/oidc/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	http.Redirect(w, r, s.oidc.oauth2.AuthCodeURL(state), http.StatusFound)
}

// handleOIDCCallback, kodu token'a cevirir, id_token'i dogrular, oturum acar.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !s.oidc.Enabled() {
		http.Error(w, "OIDC yapilandirilmamis", http.StatusNotFound)
		return
	}
	if err := s.oidc.init(); err != nil {
		slog.Error("oidc saglayici erisilemedi", "err", err)
		http.Error(w, "OIDC saglayiciya erisilemedi", http.StatusBadGateway)
		return
	}
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil || stateCookie.Value == "" || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "geçersiz OIDC state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, Value: "", Path: "/api/auth/oidc/", MaxAge: -1})

	oauth2Token, err := s.oidc.oauth2.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("oidc token degisimi basarisiz", "err", err)
		http.Error(w, "OIDC token degisimi basarisiz", http.StatusUnauthorized)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "id_token yok", http.StatusUnauthorized)
		return
	}
	idToken, err := s.oidc.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		slog.Error("id_token dogrulanamadi", "err", err)
		http.Error(w, "id_token dogrulanamadi", http.StatusUnauthorized)
		return
	}

	var claims struct {
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Groups            []string `json:"groups"`
		Roles             []string `json:"roles"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "claim okunamadi", http.StatusUnauthorized)
		return
	}

	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	role := Role(s.oidc.opts.DefaultRole)
	for _, g := range append(claims.Groups, claims.Roles...) {
		if mapped, ok := s.oidc.opts.GroupRoles[g]; ok {
			role = Role(mapped)
			break
		}
	}
	if !role.Valid() {
		role = RoleViewer
	}

	ident := &Identity{Username: username, Role: role, Kind: "oidc"}
	token := newSessionToken()
	s.auth.mu.Lock()
	s.auth.sessions[token] = &session{ident: *ident, exp: time.Now().Add(sessionTTL)}
	s.auth.mu.Unlock()

	s.audit(r, ident, "login.oidc", "user:"+username, "SSO girisi: "+string(role))
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusFound)
}
