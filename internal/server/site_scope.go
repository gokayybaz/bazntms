package server

// Çoklu-saha (senaryo B, S14.B2) yönetim kapsamı yardımcıları.
//
// site sert bir yetki sınırıdır: saha-kısıtlı bir kimlik (Identity.Site dolu)
// yalnızca kendi sahasının kullanıcı / API token / enroll token / agent /
// cihazlarını görebilir ve yönetebilir; global admin (Site boş) her şeyi.
// Bkz. docs/DEPLOYMENT-MODEL.md.

import "net/http"

// roleSiteConsistent, rol ↔ site tutarlılığını doğrular:
//   - admin       → site BOŞ olmalı (global yönetici)
//   - site-admin  → site DOLU olmalı (yoksa fiilen global olurdu)
//   - diğerleri   → serbest
func roleSiteConsistent(role Role, site string) (ok bool, msg string) {
	switch role {
	case RoleAdmin:
		if site != "" {
			return false, "global admin bir siteye bağlanamaz — site alanını boş bırakın"
		}
	case RoleSiteAdmin:
		if site == "" {
			return false, "site-admin rolü için site zorunlu"
		}
	}
	return true, ""
}

// scopedManageAllowed, saha-kısıtlı bir yöneticinin verilen site'lı bir yönetim
// kaynağına dokunmasına izin var mı. Yoksa 403 yazıp döner (ok=false).
func (s *Server) scopedManageAllowed(w http.ResponseWriter, r *http.Request, resourceSite string) bool {
	id := identityFromCtx(r)
	if inSiteScope(id, resourceSite) {
		return true
	}
	s.audit(r, id, "denied", r.Method+" "+r.URL.Path, "kapsam dışı site: "+resourceSite)
	forbidden(w, "bu kaynak sizin sahanızın dışında")
	return false
}

// enforceCreateScope, saha-kısıtlı bir yöneticinin oluşturduğu yönetim
// kaynağının site'ını kendi sahasına sabitler ve global (admin) rol atamasını
// reddeder. want* değerleri caller tarafından güncellenir. reddedilirse 403
// yazıp false döner.
func (s *Server) enforceCreateScope(w http.ResponseWriter, r *http.Request, site *string, role Role) bool {
	scope := SiteScope(identityFromCtx(r))
	if scope == "" {
		return true // global admin
	}
	*site = scope
	if role == RoleAdmin {
		forbidden(w, "saha yöneticisi global admin oluşturamaz")
		return false
	}
	return true
}
