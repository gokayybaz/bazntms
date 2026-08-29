package server

// Cihaz yonetimi ve ag cihazi verileri (Faz 3). UI auth ile korunur.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type deviceRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Kind        string `json:"kind"`
	SNMPVersion int    `json:"snmp_version"`
	Community   string `json:"community"`
	V3User      string `json:"v3_user"`
	V3AuthProto string `json:"v3_auth_proto"`
	V3AuthPass  string `json:"v3_auth_pass"`
	V3PrivProto string `json:"v3_priv_proto"`
	V3PrivPass  string `json:"v3_priv_pass"`
	PollSeconds int    `json:"poll_seconds"`
}

func (s *Server) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// kimlik bilgilerini UI'ya gonderme; sadece dolum durumu
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
	if req.SNMPVersion != 3 {
		req.SNMPVersion = 2
	}
	if req.PollSeconds <= 0 {
		req.PollSeconds = 60
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
	if err != nil {
		http.Error(w, "şifreleme: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := s.store.AddDevice(store.Device{
		Name: req.Name, Host: req.Host, Kind: req.Kind,
		SNMPVersion: req.SNMPVersion, Community: req.Community,
		V3User: req.V3User, V3AuthProto: req.V3AuthProto, V3AuthPass: req.V3AuthPass,
		V3PrivProto: req.V3PrivProto, V3PrivPass: req.V3PrivPass,
		PollSeconds: req.PollSeconds, Enabled: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("cihaz eklendi", "id", id, "name", req.Name, "host", req.Host, "kind", req.Kind)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleDeviceDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteDevice(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("cihaz silindi", "device_id", id)
	writeJSON(w, map[string]any{"ok": true})
}

// handleDeviceIfaces, cihazin son arayuz orneklerini verimlerle dondurur.
func (s *Server) handleDeviceIfaces(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
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
	flows, err := s.store.TopFlows(time.Now().Add(-time.Duration(minutes)*time.Minute), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, flows)
}

func (s *Server) handleSyslogEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.RecentSyslog(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}
