package server

// RBAC (Faz 5.1, çoklu-saha S14.B2): roller, yetkiler ve kimlik denetimi.
//
// Roller ve yetki matrisi:
//
//               view  operate  analyze  devices  agents  admin  global-admin
//   admin        ✓       ✓        ✓        ✓       ✓      ✓         ✓
//   site-admin   ✓       ✓        ✓        ✓       ✓      ✓         ✗
//   netops       ✓       ✓        ✓        ✓       ✗      ✗         ✗
//   analyst      ✓       ✗        ✓        ✗       ✗      ✗         ✗
//   viewer       ✓       ✗        ✗        ✗       ✗      ✗         ✗
//
// "view" salt-okuma GET; "operate" yakalama/kayit; "devices/agents" silme/
// olusturma; "admin" kullanici/token/enroll-token/agent yonetimi (kapsam ici);
// "global-admin" saha-ustu: ISMS, 5651 compliance, uyari config, audit zinciri
// dogrulama.
//
// Site scope (Identity.Site): bos = global (tum sahalar). Dolu = o sahaya
// kilitli — filo sorgulari + yonetim uclari o site ile sinirli (inSiteScope).
// `site-admin` her zaman dolu bir Site tasir; `admin` her zaman bos.
// Bkz. docs/DEPLOYMENT-MODEL.md.

import (
	"context"
	"encoding/json"
	"net/http"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleSiteAdmin Role = "site-admin"
	RoleNetOps    Role = "netops"
	RoleAnalyst   Role = "analyst"
	RoleViewer    Role = "viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleSiteAdmin, RoleNetOps, RoleAnalyst, RoleViewer:
		return true
	}
	return false
}

type Permission string

const (
	PermView          Permission = "view"
	PermOperate       Permission = "operate" // yakalama baslat/durdur, PCAP kaydi
	PermAnalyze       Permission = "analyze" // AI analizi, rapor
	PermManageDevices Permission = "devices" // cihaz ekle/sil
	PermManageAgents  Permission = "agents"  // agent sil
	PermAdmin         Permission = "admin"   // kullanici/token/enroll-token/agent yonetimi (kapsam ici)
	PermGlobalAdmin   Permission = "global-admin"
)

var rolePermissions = map[Role]map[Permission]bool{
	RoleAdmin: {
		PermView: true, PermOperate: true, PermAnalyze: true,
		PermManageDevices: true, PermManageAgents: true, PermAdmin: true, PermGlobalAdmin: true,
	},
	RoleSiteAdmin: {
		PermView: true, PermOperate: true, PermAnalyze: true,
		PermManageDevices: true, PermManageAgents: true, PermAdmin: true, PermGlobalAdmin: false,
	},
	RoleNetOps: {
		PermView: true, PermOperate: true, PermAnalyze: true,
		PermManageDevices: true, PermManageAgents: false, PermAdmin: false,
	},
	RoleAnalyst: {
		PermView: true, PermOperate: false, PermAnalyze: true,
		PermManageDevices: false, PermManageAgents: false, PermAdmin: false,
	},
	RoleViewer: {
		PermView: true, PermOperate: false, PermAnalyze: false,
		PermManageDevices: false, PermManageAgents: false, PermAdmin: false,
	},
}

// Allows, rolun istenen yetkisi var mi?
func (r Role) Allows(p Permission) bool {
	if !r.Valid() {
		return false
	}
	return rolePermissions[r][p]
}

// Identity, oturum/api-token/oidc arkasindaki kimliktir.
type Identity struct {
	Username string `json:"username"`
	Role     Role   `json:"role"`
	Site     string `json:"site"` // bos = tum siteler
	Kind     string `json:"kind"` // user | legacy | token | oidc
}

const identityCtxKey ctxKey = "identity"

func identityFromCtx(r *http.Request) *Identity {
	if v, ok := r.Context().Value(identityCtxKey).(*Identity); ok {
		return v
	}
	return nil
}

func contextWithIdentity(r *http.Request, id *Identity) context.Context {
	return context.WithValue(r.Context(), identityCtxKey, id)
}

// requirePerm, kimligin rolunu yetki matrisine gore denetler; oturum
// middleware'inden SONRA calismalidir. Kimlik dogrulama kapaliysa
// (dev modu) her seye izin verilir.
func (s *Server) requirePerm(p Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		id := identityFromCtx(r)
		if id == nil {
			unauthorized(w, "oturum gerekli")
			return
		}
		if !id.Role.Allows(p) {
			s.audit(r, id, "denied", string(r.Method)+" "+r.URL.Path, "yetki reddi: "+string(p))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"error": "yetkiniz yok", "required": string(p)})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SiteScope, site-sinirli kimlik icin site filtresini dondurur:
// bos donerse tum siteler goruntulenebilir.
func SiteScope(id *Identity) string {
	if id == nil {
		return ""
	}
	return id.Site
}

// sanitizeIdentity, tutarsiz kimlikleri guvenli tarafa cevirir (savunma
// derinligi — create/update zaten roleSiteConsistent uygular):
//   - site-admin ama Site bos → fiilen global olurdu; viewer'a dusur
//   - admin ama Site dolu     → global admin bir siteye baglanamaz; Site'i sil
func sanitizeIdentity(id *Identity) *Identity {
	if id == nil {
		return nil
	}
	switch id.Role {
	case RoleSiteAdmin:
		if id.Site == "" {
			id.Role = RoleViewer
		}
	case RoleAdmin:
		id.Site = ""
	}
	return id
}

// inSiteScope, kimligin verilen kaynak-site'ina erisip erisemeyecegini soyler:
// global kimlik (Site=="") her seye, saha-kisitli kimlik yalnizca kendi
// sahasina. Bir yonetim islemi (kullanici/token/enroll-token/agent CRUD) bu
// denetimden gecmelidir (S14.B2).
func inSiteScope(id *Identity, resourceSite string) bool {
	scope := SiteScope(id)
	return scope == "" || scope == resourceSite
}

// forbidden, 403 + JSON gerekce dondurur (yetki reddi loglama cagirana birakilir).
func forbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
