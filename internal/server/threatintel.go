package server

// Tehdit istihbaratı ucu (Faz 24-E). Sağlayıcı-bağımsız; -ioc-file verildiyse
// localfile sağlayıcısıyla. Oto-blok yok — yalnızca görünürlük.

import (
	"net/http"

	"github.com/gokayybaz/bazntms/internal/threatintel"
)

// handleThreatIntel, GET /api/v1/threatintel?ip=&domain=
func (s *Server) handleThreatIntel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ip, domain := q.Get("ip"), q.Get("domain")
	if ip == "" && domain == "" {
		http.Error(w, "ip veya domain gerekli", http.StatusBadRequest)
		return
	}
	out := map[string]any{"providers": []string{}}
	if s.ti != nil {
		out["providers"] = s.ti.Providers()
	}
	if ip != "" {
		out["ip"] = s.tiLookup(ip, "ip")
	}
	if domain != "" {
		out["domain"] = s.tiLookup(domain, "domain")
	}
	writeJSON(w, out)
}

// tiLookup, servis nil ise "unknown" bir Indicator döndürür (uç her zaman
// tutarlı bir şema verir).
func (s *Server) tiLookup(indicator, typ string) threatintel.Indicator {
	if s.ti == nil {
		return threatintel.Indicator{Indicator: indicator, Type: typ, Reputation: threatintel.Unknown}
	}
	if typ == "ip" {
		return s.ti.IP(indicator)
	}
	return s.ti.Domain(indicator)
}
