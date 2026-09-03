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
	writeJSON(w, list)
}

func maskNonEmpty(s string) string {
	if s == "" {
		return ""
	}
	return "•••"
}

func (s *Server) handleDeviceAdd(w http.ResponseWriter, r *http.Request) {
	var req deviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Host == "" {
		http.Error(w, "name ve host zorunlu", http.StatusBadRequest)
		return
	}
	if req.Kind == "" {
		req.Kind = "other"
	}
	if req.Vendor == "" {
		req.Vendor = "snmp"
	}
	if req.Vendor != "snmp" && req.Vendor != "fortigate" {
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
		APIVerifyTLS: req.APIVerifyTLS, VDOM: req.VDOM,
		PollSeconds: req.PollSeconds, Enabled: true,
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
	if minutes <= 0 || minutes > 60*24*7 {
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

func (s *Server) handleSyslogEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.RecentSyslog(limit, SiteScope(identityFromCtx(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}
