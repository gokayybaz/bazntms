package server

// FortiGate veri uçları (Faz 8.6): kaynak kullanımı, VPN durumu, SD-WAN
// sağlık örnekleri ve politika hit trendleri. UI auth ile korunur (view).

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// parseMinutes, ?minutes= parametresini (varsayılan 60, üst sınır 7 gün) okur.
func parseMinutes(r *http.Request, def int) int {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 60*24*7 {
		return def
	}
	return minutes
}

// handleDeviceResources, cihazın kaynak kullanım zaman serisi (CPU/RAM/disk/oturum).
func (s *Server) handleDeviceResources(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	rows, err := s.store.LatestDeviceResources(id, parseMinutes(r, 60))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rows)
}

// handleDeviceVPN, cihazın güncel VPN tünel/kullanıcı durumları.
func (s *Server) handleDeviceVPN(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	rows, err := s.store.LatestFortiVPN(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rows)
}

// handleDeviceSDWAN, cihazın SD-WAN health-check örnekleri.
func (s *Server) handleDeviceSDWAN(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	rows, err := s.store.RecentFortiSDWANAll(
		time.Now().Add(-time.Duration(parseMinutes(r, 60)) * time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// cihaz filtresi
	filtered := []store.SDWANRow{}
	for _, row := range rows {
		if row.DeviceID == id {
			filtered = append(filtered, row)
		}
	}
	writeJSON(w, filtered)
}

// handleDevicePolicies, cihazın penceredeki en aktif politika hit'leri (delta).
func (s *Server) handleDevicePolicies(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.store.TopFortiPolicies(id,
		time.Now().Add(-time.Duration(parseMinutes(r, 60))*time.Minute), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rows)
}
