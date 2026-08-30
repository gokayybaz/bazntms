package server

// 5651 uyumluluk uçları (Faz 9): durum, delil paketi, inceleme tutanakları.
// Delil paketi admin yetkisi ister (PermAdmin); inceleme yazımı da admin'dir
// (netops okuyabilir).

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/compliance"
	"github.com/gokayybaz/bazntms/internal/store"
)

// ComplianceInfo, hub'dan panele taşınan yapılandırma bilgisidir.
type ComplianceInfo struct {
	Enabled       bool   `json:"enabled"`
	TSAURL        string `json:"tsa_url"`
	SignKey       bool   `json:"sign_key"`
	WormDir       string `json:"worm_dir"`
	MaskPII       bool   `json:"mask_pii"`
	RetentionDays int    `json:"retention_days"`
}

// SetCompliance, sealer yapılandırmasını panele taşır (main'de çağrılır).
func (s *Server) SetCompliance(info ComplianceInfo) { s.compliance = info }

// handleComplianceStatus, uyum paneli durumu.
func (s *Server) handleComplianceStatus(w http.ResponseWriter, r *http.Request) {
	total, lastTs, err := s.store.ComplianceStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := map[string]any{
		"config":         s.compliance,
		"records":        total,
		"last_record_ts": lastTs,
	}
	if cp, err := s.store.LatestLogCheckpoint("hourly"); err == nil && cp != nil {
		out["last_hourly"] = map[string]any{
			"bucket_start": cp.BucketStart, "root": cp.Root, "record_count": cp.RecordCount,
		}
	}
	if cp, err := s.store.LatestLogCheckpoint("daily"); err == nil && cp != nil {
		out["last_daily"] = map[string]any{
			"day":  time.Unix(cp.BucketStart, 0).Format("2006-01-02"),
			"root": cp.Root, "tsa_status": cp.TSAStatus,
			"tsa_time": cp.TSATime, "signed_at": cp.SignedAt,
			"signed": cp.Signature != "", "record_count": cp.RecordCount,
		}
	}
	writeJSON(w, out)
}

// handleComplianceEvidence, delil paketi üretir ve JSON olarak indirir (A.5.28).
// Örnek: /api/v1/compliance/evidence?from=2026-08-01&to=2026-09-01&mask=true
func (s *Server) handleComplianceEvidence(w http.ResponseWriter, r *http.Request) {
	from, err := parseDay(r.URL.Query().Get("from"), time.Now().AddDate(0, 0, -30))
	if err != nil {
		http.Error(w, "from: "+err.Error(), http.StatusBadRequest)
		return
	}
	to, err := parseDay(r.URL.Query().Get("to"), time.Now())
	if err != nil {
		http.Error(w, "to: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("to") != "" {
		to = to.AddDate(0, 0, 1) // bitiş günü kapsayıcı (gün sonu)
	}
	if !to.After(from) {
		http.Error(w, "to, from'dan sonra olmalı", http.StatusBadRequest)
		return
	}
	mask := strings.EqualFold(r.URL.Query().Get("mask"), "true") || s.compliance.MaskPII

	bundle, err := compliance.BuildEvidence(s.store, from.Unix(), to.Unix(), mask)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "compliance.evidence", "",
		"delil paketi: "+from.Format("2006-01-02")+".."+to.Format("2006-01-02")+
			" mask="+strconv.FormatBool(mask))
	data, err := compliance.BundleToJSON(bundle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition",
		"attachment; filename=bazntms-delil-"+from.Format("20060102")+"-"+to.Format("20060102")+".json")
	w.Write(data)
}

func parseDay(s string, def time.Time) (time.Time, error) {
	if s == "" {
		return def, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// handleComplianceReviewsGet, inceleme tutanakları.
func (s *Server) handleComplianceReviewsGet(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reviews, err := s.store.RecentComplianceReviews(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, reviews)
}

// handleComplianceReviewAdd, tutanak yazımı (log inceleme A.8.15 veya
// erişim incelemesi A.8.2) — tutanak audit zincire de işlenir.
func (s *Server) handleComplianceReviewAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind    string `json:"kind"` // log | access
		Period  string `json:"period"`
		Notes   string `json:"notes"`
		Finding string `json:"finding"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Kind != "log" && req.Kind != "access" {
		http.Error(w, "kind log veya access olmalı", http.StatusBadRequest)
		return
	}
	id := identityFromCtx(r)
	username := ""
	if id != nil {
		username = id.Username
	}
	rid, err := s.store.SaveComplianceReview(store.ComplianceReview{
		Ts: time.Now().Unix(), Username: username, Kind: req.Kind,
		Period: req.Period, Notes: req.Notes, Finding: req.Finding,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, id, "compliance.review", req.Kind+":"+strconv.FormatInt(rid, 10),
		req.Period+" bulgu="+strconv.FormatBool(req.Finding != ""))
	writeJSON(w, map[string]any{"ok": true, "id": rid})
}
