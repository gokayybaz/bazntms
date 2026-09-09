package server

// FortiGate "Bağlantıyı Sına" (probe) + dar FortiGate düzenleme (Faz 27).
//
// FortiOS monitor API'si şema vermez; probe her veri ucuna gerçek istek atıp
// (bağlandı mı / HTTP kodu / parser anladı mı / kaç kayıt) rapor döndürür ve
// yanıt zarfından FortiOS sürümünü + VDOM modunu çıkarır. Sürüm cihaz satırına
// yazılır; kullanıcı raporu görüp "Profil"i elle sabitleyebilir.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/fortigate"
)

type probeRequest struct {
	DeviceID     int64   `json:"device_id"`      // >0 → mevcut cihazı yeniden sına
	APIURL       string  `json:"api_url"`        // yeni cihaz sınaması
	APIToken     string  `json:"api_token"`      // yeni cihaz sınaması (düz metin)
	APIVerifyTLS bool    `json:"api_verify_tls"` // yeni cihaz sınaması
	VDOM         string  `json:"vdom"`
	Profile      *string `json:"profile"` // verilirse cihaza pinlenir (mevcut cihaz)
}

// handleDeviceProbe, POST /api/v1/devices/probe.
func (s *Server) handleDeviceProbe(w http.ResponseWriter, r *http.Request) {
	var req probeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Profile != nil && !fortigate.ValidProfileID(*req.Profile) {
		http.Error(w, "geçersiz profil", http.StatusBadRequest)
		return
	}

	opts := fortigate.Options{VerifyTLS: req.APIVerifyTLS, VDOM: strings.TrimSpace(req.VDOM)}
	var existing bool
	var deviceID int64
	auditTarget := "device:new"

	if req.DeviceID > 0 {
		existing = true
		deviceID = req.DeviceID
		if !s.deviceInScope(r, deviceID) {
			http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
			return
		}
		d, err := s.store.DeviceByID(deviceID)
		if err != nil || d == nil {
			http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
			return
		}
		if d.Vendor != "fortigate" {
			http.Error(w, "cihaz FortiGate değil", http.StatusBadRequest)
			return
		}
		token, err := s.vault.Decrypt(d.APIToken)
		if err != nil || token == "" {
			http.Error(w, "kayıtlı API token çözülemedi", http.StatusInternalServerError)
			return
		}
		opts.BaseURL = d.APIURL
		opts.Token = token
		opts.VerifyTLS = d.APIVerifyTLS
		if opts.VDOM == "" {
			opts.VDOM = d.VDOM
		}
		pinned := d.APIProfile
		if req.Profile != nil {
			pinned = strings.TrimSpace(*req.Profile)
		}
		opts.Profile = fortigate.ResolveProfile(pinned, d.APIVersion)
		auditTarget = fmt.Sprintf("device:%d", deviceID)
	} else {
		if !strings.HasPrefix(req.APIURL, "https://") && !strings.HasPrefix(req.APIURL, "http://") {
			http.Error(w, "api_url https:// ile başlamalı", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.APIToken) == "" {
			http.Error(w, "api_token zorunlu", http.StatusBadRequest)
			return
		}
		opts.BaseURL = req.APIURL
		opts.Token = req.APIToken
		if req.Profile != nil {
			if p, ok := fortigate.ProfileByID(*req.Profile); ok {
				opts.Profile = p
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	report := fortigate.Probe(ctx, opts)

	// mevcut cihaz: tespit edilen sürüm + uç yetenek özetini kalıcılaştır
	if existing {
		caps, _ := json.Marshal(report.Caps)
		if err := s.store.UpdateDeviceFortiMeta(deviceID, report.Version, string(caps)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if req.Profile != nil {
			if err := s.store.SetDeviceFortiProfile(deviceID, strings.TrimSpace(*req.Profile)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	detail := report.Version
	if report.Error != "" {
		detail = "hata: " + report.Error
	}
	s.audit(r, identityFromCtx(r), "device.probe", auditTarget, detail)
	writeJSON(w, report)
}

type fortiConfigRequest struct {
	VDOM    *string `json:"vdom"`
	Profile *string `json:"profile"`
}

// handleDeviceFortiConfig, PUT /api/v1/devices/{id}/fortigate — VDOM / sürüm
// profili dar düzenleme (genel cihaz-düzenleme endpoint'i yok).
func (s *Server) handleDeviceFortiConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if !s.deviceInScope(r, id) {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	d, err := s.store.DeviceByID(id)
	if err != nil || d == nil {
		http.Error(w, "cihaz bulunamadı", http.StatusNotFound)
		return
	}
	if d.Vendor != "fortigate" {
		http.Error(w, "cihaz FortiGate değil", http.StatusBadRequest)
		return
	}
	var req fortiConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	before := map[string]any{"vdom": d.VDOM, "profile": d.APIProfile}
	if req.Profile != nil {
		p := strings.TrimSpace(*req.Profile)
		if !fortigate.ValidProfileID(p) {
			http.Error(w, "geçersiz profil", http.StatusBadRequest)
			return
		}
		if err := s.store.SetDeviceFortiProfile(id, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if req.VDOM != nil {
		if err := s.store.SetDeviceVDOM(id, strings.TrimSpace(*req.VDOM)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	after, _ := s.store.DeviceByID(id)
	afterMap := map[string]any{}
	if after != nil {
		afterMap = map[string]any{"vdom": after.VDOM, "profile": after.APIProfile}
	}
	s.auditDiff(r, identityFromCtx(r), "device.fortigate", fmt.Sprintf("device:%d", id), "", before, afterMap)
	writeJSON(w, map[string]any{"ok": true})
}
