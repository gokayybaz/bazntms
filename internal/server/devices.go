package server

// Cihaz yonetimi ve ag cihazi verileri (Faz 3). UI auth ile korunur.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/fortigate"
	"github.com/gokayybaz/bazntms/internal/store"
)

type deviceRequest struct {
	Name         string `json:"name"`
	Host         string `json:"host"`
	Kind         string `json:"kind"`
	Site         string `json:"site"`   // RBAC site-scope
	Vendor       string `json:"vendor"` // snmp | fortigate (Faz 8)
	SNMPVersion  int    `json:"snmp_version"`
	Community    string `json:"community"`
	V3User       string `json:"v3_user"`
	V3AuthProto  string `json:"v3_auth_proto"`
	V3AuthPass   string `json:"v3_auth_pass"`
	V3PrivProto  string `json:"v3_priv_proto"`
	V3PrivPass   string `json:"v3_priv_pass"`
	APIURL       string `json:"api_url"`   // fortigate: https://host:port
	APIToken     string `json:"api_token"` // fortigate: düz metin → vault
	APIVerifyTLS bool   `json:"api_verify_tls"`
	VDOM         string `json:"vdom"`
	APIProfile   string `json:"profile"` // fortigate: FortiOS sürüm profili ("" / "auto" → oto)
	PollSeconds  int    `json:"poll_seconds"`
}

// deviceInScope, site-sinirli bir kimligin verilen cihaza erisip
// erisemeyecegini soyler. Kimlik site-sinirsizsa (SiteScope=="") her zaman
// true. Cihaz bulunamazsa false (handler 404 doner).
func (s *Server) deviceInScope(r *http.Request, id int64) bool {
	scope := SiteScope(identityFromCtx(r))
	if scope == "" {
		return true
	}
	d, err := s.store.DeviceByID(id)
	return err == nil && d.Site == scope
}

func (s *Server) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListDevices(SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// kimlik bilgilerini UI'ya gonderme; sadece dolum durumu
	// (APIToken zaten json:"-" ile hiçbir koşulda serileştirilmez)
	for i := range list {
		list[i].Community = maskNonEmpty(list[i].Community)
		list[i].V3AuthPass = maskNonEmpty(list[i].V3AuthPass)
		list[i].V3PrivPass = maskNonEmpty(list[i].V3PrivPass)
	}
	writeJSONETag(w, r, list) // D4: sık pollanır
}

// secretMask, dolu bir sırrın UI'ya gönderilen yer tutucusu. İstemci bu
// değeri geri PUT ederse sunucu alanı "değiştirilmedi" sayar.
const secretMask = "•••"

func maskNonEmpty(s string) string {
	if s == "" {
		return ""
	}
	return secretMask
}

func (s *Server) handleDeviceAdd(w http.ResponseWriter, r *http.Request) {
	var req deviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name zorunlu", http.StatusBadRequest)
		return
	}
	if req.Kind == "" {
		req.Kind = "other"
	}
	// host'suz kayıt = "yönetilmeyen" topoloji düğümü (Canlı Akış gruplama için
	// switch/AP). SNMP/API yok → poll edilmez (Enabled=false). Yalnızca ağ
	// donanımı türleri için.
	unmanaged := req.Host == ""
	if unmanaged {
		switch req.Kind {
		case "switch", "ap", "router", "firewall", "other":
		default:
			http.Error(w, "host'suz cihaz yalnızca switch/ap/router/firewall/other olabilir", http.StatusBadRequest)
			return
		}
		if req.Vendor == "fortigate" {
			http.Error(w, "fortigate için host zorunlu", http.StatusBadRequest)
			return
		}
	}
	if req.Vendor == "" {
		req.Vendor = "snmp"
	}
	if req.Vendor == "mock" && !s.mockDevices {
		http.Error(w, "vendor=mock yalnızca hub -mock-devices ile (ölçek testi)", http.StatusBadRequest)
		return
	}
	if req.Vendor != "snmp" && req.Vendor != "fortigate" && req.Vendor != "mock" {
		http.Error(w, "vendor snmp veya fortigate olmalı", http.StatusBadRequest)
		return
	}
	if req.SNMPVersion != 3 {
		req.SNMPVersion = 2
	}
	if req.PollSeconds <= 0 {
		req.PollSeconds = 60
	}
	// fortigate dogrulamasi: api_url + token zorunlu
	if req.Vendor == "fortigate" {
		if !strings.HasPrefix(req.APIURL, "https://") && !strings.HasPrefix(req.APIURL, "http://") {
			http.Error(w, "fortigate için api_url https:// ile başlamalı", http.StatusBadRequest)
			return
		}
		if req.APIToken == "" {
			http.Error(w, "fortigate için api_token zorunlu (REST API admin token'ı)", http.StatusBadRequest)
			return
		}
		if req.VDOM == "" {
			req.VDOM = "root"
		}
		if !fortigate.ValidProfileID(req.APIProfile) {
			http.Error(w, "geçersiz fortigate profili", http.StatusBadRequest)
			return
		}
		if req.APIProfile == "auto" {
			req.APIProfile = ""
		}
	}
	// hassas alanlari sifrele
	var err error
	mustEncrypt := func(in *string) {
		if err != nil {
			return
		}
		*in, err = s.vault.Encrypt(*in)
	}
	mustEncrypt(&req.Community)
	mustEncrypt(&req.V3AuthPass)
	mustEncrypt(&req.V3PrivPass)
	mustEncrypt(&req.APIToken)
	if err != nil {
		http.Error(w, "şifreleme: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// site-sinirli kimlik yalnizca kendi sitesine cihaz ekleyebilir
	if scope := SiteScope(identityFromCtx(r)); scope != "" {
		req.Site = scope
	}
	id, err := s.store.AddDevice(store.Device{
		Name: req.Name, Host: req.Host, Kind: req.Kind, Site: req.Site, Vendor: req.Vendor,
		SNMPVersion: req.SNMPVersion, Community: req.Community,
		V3User: req.V3User, V3AuthProto: req.V3AuthProto, V3AuthPass: req.V3AuthPass,
		V3PrivProto: req.V3PrivProto, V3PrivPass: req.V3PrivPass,
		APIURL: req.APIURL, APIToken: req.APIToken,
		APIVerifyTLS: req.APIVerifyTLS, VDOM: req.VDOM, APIProfile: req.APIProfile,
		PollSeconds: req.PollSeconds, Enabled: !unmanaged,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("cihaz eklendi", "id", id, "name", req.Name, "host", req.Host, "kind", req.Kind, "vendor", req.Vendor)
	s.audit(r, identityFromCtx(r), "device.add", fmt.Sprintf("device:%d", id), req.Name+" ("+req.Host+", "+req.Vendor+")")
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleDeviceDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	if err := s.store.DeleteDevice(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("cihaz silindi", "device_id", id)
	s.audit(r, identityFromCtx(r), "device.delete", fmt.Sprintf("device:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

// handleDeviceSetUplink, cihazın üst cihazını (switch → router zinciri) atar
// veya kaldırır. Gövde: {"device_id": <id>} veya {"device_id": null}.
func (s *Server) handleDeviceSetUplink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	var body struct {
		DeviceID *int64 `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}
	if body.DeviceID != nil {
		if *body.DeviceID == id {
			http.Error(w, "cihaz kendine bağlanamaz", http.StatusBadRequest)
			return
		}
		if !s.deviceInScope(r, *body.DeviceID) {
			http.Error(w, "üst cihaz bulunamadı", http.StatusBadRequest)
			return
		}
	}
	var prevUplink *int64
	if cur, err := s.store.DeviceByID(id); err == nil && cur != nil {
		prevUplink = cur.UplinkDeviceID
	}
	if err := s.store.SetDeviceUplink(id, body.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	detail := "kaldırıldı"
	if body.DeviceID != nil {
		detail = fmt.Sprintf("device:%d", *body.DeviceID)
	}
	// Faz 25-C: uplink ataması denetim farkı.
	s.auditDiff(r, identityFromCtx(r), "device.uplink", fmt.Sprintf("device:%d", id), detail,
		map[string]any{"uplink_device_id": prevUplink},
		map[string]any{"uplink_device_id": body.DeviceID})
	writeJSON(w, map[string]any{"ok": true})
}

// handleDeviceIfaces, cihazin son arayuz orneklerini verimlerle dondurur.
func (s *Server) handleDeviceIfaces(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	ifaces, err := s.store.LatestDeviceIfaces(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, ifaces)
}

func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	// "en yoğun akışlar" anlık bir görünüm — üst sınır 6 saat (ölçekte daha
	// geniş pencere milyonlarca satır sıralar; uzun dönem trendi rapor/flows_1h).
	if minutes <= 0 || minutes > 360 {
		minutes = 15
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	flows, err := s.store.TopFlows(time.Now().Add(-time.Duration(minutes)*time.Minute), limit, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, flows)
}

// flowConvoWindow, konuşma pencereleri: 15m (vars.) | 1h | 6h | 24h.
func flowConvoWindow(w string) time.Time {
	d := 15 * time.Minute
	switch w {
	case "1h":
		d = time.Hour
	case "6h":
		d = 6 * time.Hour
	case "24h":
		d = 24 * time.Hour
	}
	return time.Now().Add(-d)
}

// handleFlowConversations, ham NetFlow kayıtlarını konuşmalara toplar (Faz
// 23-B). ?window=15m|1h|6h|24h &by=5tuple|pair &sort=octets|packets|flows|last_seen.
func (s *Server) handleFlowConversations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	by := q.Get("by")
	if by != "5tuple" {
		by = "pair"
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	list, err := s.store.FlowConversations(flowConvoWindow(q.Get("window")), by, q.Get("sort"), limit, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// handleFlowConversationDetail, tek bir konuşmanın ham akışları + GeoIP/ASN +
// süreç korelasyonu. ?src=&dst=[&proto=&window=].
func (s *Server) handleFlowConversationDetail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	src, dst := q.Get("src"), q.Get("dst")
	if src == "" || dst == "" {
		http.Error(w, "src ve dst gerekli", http.StatusBadRequest)
		return
	}
	since := flowConvoWindow(q.Get("window"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	site := SiteScope(identityFromCtx(r))
	rows, err := s.store.FlowConversationDetail(since, src, dst, q.Get("proto"), limit, site)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	actors, err := s.store.FlowActorsForConversation(since, src, dst)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]any{
		"flows":    rows,
		"actors":   actors,
		"src_info": s.enrich.IP(src),
		"dst_info": s.enrich.IP(dst),
	}
	if s.ti != nil {
		resp["src_rep"] = s.ti.IP(src)
		resp["dst_rep"] = s.ti.IP(dst)
	}
	writeJSON(w, resp)
}

func (s *Server) handleSyslogEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.RecentSyslog(limit, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}
