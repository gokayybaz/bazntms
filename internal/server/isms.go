package server

// ISMS yönetişim uçları (Faz 10): varlık envanteri, risk defteri, SoA,
// politika yönetimi, iç denetim + CAPA, yönetim incelemesi, tedarikçi
// güvenliği, süreklilik testleri ve denetçi paketi.
//
// Yetki modeli: okuma oturum yeterlidir (cihaz uçları gibi); yazma işlemleri
// PermAdmin ister — yönetişim kayıtları kanıt niteliğinde olduğundan
// değişiklikler Faz 5 audit zincirine işlenir. Denetçi paketi dışa aktarımı
// da admin yetkisi ister.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/isms"
	"github.com/gokayybaz/bazntms/internal/store"
)

var (
	ismsCriticalities   = map[string]bool{"dusuk": true, "orta": true, "yuksek": true, "kritik": true}
	ismsTreatments      = map[string]bool{"mitigate": true, "accept": true, "transfer": true, "avoid": true}
	ismsRiskStatuses    = map[string]bool{"open": true, "in_progress": true, "closed": true}
	ismsSoaStatuses     = map[string]bool{"planned": true, "implemented": true, "verified": true}
	ismsPolicyFlow      = []string{"draft", "in_review", "approved", "published", "archived"}
	ismsAuditStatuses   = map[string]bool{"planned": true, "done": true, "closed": true}
	ismsFindingStatuses = map[string]bool{"open": true, "in_progress": true, "verified": true, "closed": true}
	ismsSeverities      = map[string]bool{"dusuk": true, "orta": true, "yuksek": true}
	ismsContinuityKinds = map[string]bool{"restore": true, "failover": true, "backup_check": true, "tabletop": true}
)

func ismsBad(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func ismsPathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// handleIsmsSummary, yönetişim paneli için olgunluk sayıları.
func (s *Server) handleIsmsSummary(w http.ResponseWriter, r *http.Request) {
	total, applicable, implemented, verified, excluded, err := s.store.IsmsSoaCounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	risks, _ := s.store.ListIsmsRisks()
	var high, medium, low, open int
	for _, rk := range risks {
		switch {
		case rk.Score >= 15:
			high++
		case rk.Score >= 8:
			medium++
		default:
			low++
		}
		if rk.Status != "closed" {
			open++
		}
	}
	findings, _ := s.store.ListIsmsFindings(0)
	var openFindings int
	for _, f := range findings {
		if f.Status != "verified" && f.Status != "closed" {
			openFindings++
		}
	}
	policies, _ := s.store.ListIsmsPolicies()
	published := 0
	for _, p := range policies {
		if p.Status == "published" {
			published++
		}
	}
	assets, _ := s.store.ListIsmsAssets()
	suppliers, _ := s.store.ListIsmsSuppliers()
	now := time.Now().Unix()
	dueSuppliers := 0
	for _, sp := range suppliers {
		if sp.NextReview > 0 && sp.NextReview <= now {
			dueSuppliers++
		}
	}
	continuity, _ := s.store.ListIsmsContinuityTests(1)
	out := map[string]any{
		"soa": map[string]int{
			"total": total, "applicable": applicable,
			"implemented": implemented, "verified": verified, "excluded": excluded,
		},
		"risks": map[string]any{
			"total": len(risks), "open": open, "high": high, "medium": medium, "low": low,
		},
		"open_findings":      openFindings,
		"policies_published": published,
		"assets":             len(assets),
		"suppliers_due":      dueSuppliers,
	}
	if len(continuity) > 0 {
		out["last_continuity_test"] = continuity[0]
	}
	writeJSON(w, out)
}

// --- varlık envanteri ---

func (s *Server) handleIsmsAssetsList(w http.ResponseWriter, r *http.Request) {
	assets, err := s.store.ListIsmsAssets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, assets)
}

// handleIsmsAssetsSync, filoyu varlık envanterine yansıtır (10.1).
func (s *Server) handleIsmsAssetsSync(w http.ResponseWriter, r *http.Request) {
	added, err := s.store.SyncIsmsAssetsFromFleet()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.asset.sync", "", fmt.Sprintf("%d yeni varlık", added))
	writeJSON(w, map[string]any{"ok": true, "added": added})
}

func (s *Server) handleIsmsAssetUpdate(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	var req struct {
		Owner       string `json:"owner"`
		Criticality string `json:"criticality"`
		Notes       string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if !ismsCriticalities[req.Criticality] {
		ismsBad(w, "criticality dusuk|orta|yuksek|kritik olmalı")
		return
	}
	if err := s.store.UpdateIsmsAsset(store.IsmsAsset{ID: id, Owner: req.Owner, Criticality: req.Criticality, Notes: req.Notes}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.asset.update", strconv.FormatInt(id, 10), req.Criticality)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleIsmsAssetDelete(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	if err := s.store.DeleteIsmsAsset(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.asset.delete", strconv.FormatInt(id, 10), "")
	writeJSON(w, map[string]any{"ok": true})
}

// --- risk defteri ---

func (s *Server) handleIsmsRisksList(w http.ResponseWriter, r *http.Request) {
	risks, err := s.store.ListIsmsRisks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, risks)
}

func decodeIsmsRisk(w http.ResponseWriter, r *http.Request) (store.IsmsRisk, bool) {
	var rk store.IsmsRisk
	if err := json.NewDecoder(r.Body).Decode(&rk); err != nil {
		ismsBad(w, err.Error())
		return rk, false
	}
	if strings.TrimSpace(rk.Threat) == "" {
		ismsBad(w, "threat zorunlu")
		return rk, false
	}
	if rk.Treatment == "" {
		rk.Treatment = "mitigate"
	}
	if !ismsTreatments[rk.Treatment] {
		ismsBad(w, "treatment mitigate|accept|transfer|avoid olmalı")
		return rk, false
	}
	if rk.Status == "" {
		rk.Status = "open"
	}
	if !ismsRiskStatuses[rk.Status] {
		ismsBad(w, "status open|in_progress|closed olmalı")
		return rk, false
	}
	return rk, true
}

func (s *Server) handleIsmsRiskAdd(w http.ResponseWriter, r *http.Request) {
	rk, ok := decodeIsmsRisk(w, r)
	if !ok {
		return
	}
	id, err := s.store.AddIsmsRisk(rk)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.risk.add", strconv.FormatInt(id, 10), rk.Threat)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleIsmsRiskUpdate(w http.ResponseWriter, r *http.Request) {
	rk, ok := decodeIsmsRisk(w, r)
	if !ok {
		return
	}
	rk.ID = ismsPathID(r)
	if rk.ReviewTs == 0 {
		rk.ReviewTs = time.Now().Unix()
	}
	if err := s.store.UpdateIsmsRisk(rk); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.risk.update", strconv.FormatInt(rk.ID, 10), rk.Threat)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleIsmsRiskDelete(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	if err := s.store.DeleteIsmsRisk(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.risk.delete", strconv.FormatInt(id, 10), "")
	writeJSON(w, map[string]any{"ok": true})
}

// --- SoA ---

func (s *Server) handleIsmsSoaList(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListIsmsSoa()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}

// handleIsmsSoaUpdate, tek kontrolün kararını günceller (uygula/hariç,
// gerekçe, durum, kanıt, sahip).
func (s *Server) handleIsmsSoaUpdate(w http.ResponseWriter, r *http.Request) {
	controlID := r.PathValue("control")
	var req struct {
		Applicable    bool   `json:"applicable"`
		Justification string `json:"justification"`
		Status        string `json:"status"`
		Evidence      string `json:"evidence"`
		Owner         string `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if req.Status == "" {
		req.Status = "planned"
	}
	if !ismsSoaStatuses[req.Status] {
		ismsBad(w, "status planned|implemented|verified olmalı")
		return
	}
	if !req.Applicable && strings.TrimSpace(req.Justification) == "" {
		ismsBad(w, "hariç karar için gerekçe zorunludur (SoA denetim gereği)")
		return
	}
	if err := s.store.UpdateIsmsSoa(store.IsmsSoaItem{
		ControlID: controlID, Applicable: req.Applicable, Justification: req.Justification,
		Status: req.Status, Evidence: req.Evidence, Owner: req.Owner,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.soa.update", controlID, req.Status)
	writeJSON(w, map[string]any{"ok": true})
}

// --- politika yönetimi ---

func (s *Server) handleIsmsPoliciesList(w http.ResponseWriter, r *http.Request) {
	policies, err := s.store.ListIsmsPolicies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, policies)
}

// handleIsmsPolicyAdd, yeni politika; referans otomatik üretilir (POL-001…).
func (s *Server) handleIsmsPolicyAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title      string `json:"title"`
		Owner      string `json:"owner"`
		Content    string `json:"content"`
		NextReview int64  `json:"next_review"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		ismsBad(w, "title zorunlu")
		return
	}
	existing, err := s.store.ListIsmsPolicies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	used := map[string]bool{}
	for _, p := range existing {
		used[p.Ref] = true
	}
	ref := ""
	for i := 1; i < 10000; i++ {
		cand := fmt.Sprintf("POL-%03d", i)
		if !used[cand] {
			ref = cand
			break
		}
	}
	id, err := s.store.AddIsmsPolicy(store.IsmsPolicy{
		Ref: ref, Title: req.Title, Owner: req.Owner, Status: "draft",
		Version: "1.0", NextReview: req.NextReview,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(req.Content) != "" {
		if _, err := s.store.AddIsmsPolicyVersion(store.IsmsPolicyVersion{
			PolicyID: id, Version: "1.0", Content: req.Content, ChangeNote: "ilk sürüm",
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.audit(r, identityFromCtx(r), "isms.policy.add", ref, req.Title)
	writeJSON(w, map[string]any{"ok": true, "id": id, "ref": ref})
}

func ismsLoadPolicy(w http.ResponseWriter, r *http.Request) *store.IsmsPolicy {
	// tek politika güncelleme gövdesi
	var p store.IsmsPolicy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		ismsBad(w, err.Error())
		return nil
	}
	p.ID = ismsPathID(r)
	switch {
	case p.Status == "approved" && p.ApprovedBy == "":
		ismsBad(w, "onay için approved_by zorunlu")
		return nil
	case p.ApprovedAt == 0 && p.Status == "approved":
		p.ApprovedAt = time.Now().Unix()
	case p.PublishedAt == 0 && p.Status == "published":
		p.PublishedAt = time.Now().Unix()
	}
	return &p
}

func (s *Server) handleIsmsPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	p := ismsLoadPolicy(w, r)
	if p == nil {
		return
	}
	if err := s.store.UpdateIsmsPolicy(*p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.policy.update", p.Ref, p.Status)
	writeJSON(w, map[string]any{"ok": true})
}

// handleIsmsPolicyTransition, durum makinesini denetimli ilerletir:
// draft → in_review → approved → published → archived (geri adım yok).
func (s *Server) handleIsmsPolicyTransition(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ismsBad(w, err.Error())
		return
	}
	policies, err := s.store.ListIsmsPolicies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var cur *store.IsmsPolicy
	for i := range policies {
		if policies[i].ID == id {
			cur = &policies[i]
		}
	}
	if cur == nil {
		http.Error(w, "politika bulunamadı", http.StatusNotFound)
		return
	}
	from, to := -1, -1
	for i, st := range ismsPolicyFlow {
		if st == cur.Status {
			from = i
		}
		if st == req.Status {
			to = i
		}
	}
	if to < 0 || to != from+1 {
		ismsBad(w, "geçersiz geçiş: "+cur.Status+" → "+req.Status+
			" (sıra: "+strings.Join(ismsPolicyFlow, " → ")+")")
		return
	}
	cur.Status = req.Status
	if req.Status == "approved" {
		iden := identityFromCtx(r)
		if iden == nil {
			ismsBad(w, "onay için kimlik gerekli")
			return
		}
		cur.ApprovedBy = iden.Username
		cur.ApprovedAt = time.Now().Unix()
	}
	if req.Status == "published" {
		cur.PublishedAt = time.Now().Unix()
		if cur.NextReview == 0 {
			cur.NextReview = time.Now().AddDate(1, 0, 0).Unix() // yıllık inceleme varsayılanı
		}
	}
	if err := s.store.UpdateIsmsPolicy(*cur); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.policy.transition", cur.Ref, cur.Status)
	writeJSON(w, map[string]any{"ok": true, "status": cur.Status})
}

func (s *Server) handleIsmsPolicyVersionsList(w http.ResponseWriter, r *http.Request) {
	versions, err := s.store.ListIsmsPolicyVersions(ismsPathID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, versions)
}

func (s *Server) handleIsmsPolicyVersionAdd(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	var req struct {
		Version    string `json:"version"`
		Content    string `json:"content"`
		ChangeNote string `json:"change_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if req.Version == "" || req.Content == "" {
		ismsBad(w, "version ve content zorunlu")
		return
	}
	iden := identityFromCtx(r)
	createdBy := ""
	if iden != nil {
		createdBy = iden.Username
	}
	vid, err := s.store.AddIsmsPolicyVersion(store.IsmsPolicyVersion{
		PolicyID: id, Version: req.Version, Content: req.Content,
		ChangeNote: req.ChangeNote, CreatedBy: createdBy,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, iden, "isms.policy.version", strconv.FormatInt(id, 10), req.Version)
	writeJSON(w, map[string]any{"ok": true, "id": vid})
}

// --- iç denetim + CAPA ---

func (s *Server) handleIsmsAuditsList(w http.ResponseWriter, r *http.Request) {
	audits, err := s.store.ListIsmsAudits()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, audits)
}

func (s *Server) handleIsmsAuditAdd(w http.ResponseWriter, r *http.Request) {
	var a store.IsmsAudit
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if strings.TrimSpace(a.Title) == "" {
		ismsBad(w, "title zorunlu")
		return
	}
	if a.Status == "" {
		a.Status = "planned"
	}
	if !ismsAuditStatuses[a.Status] {
		ismsBad(w, "status planned|done|closed olmalı")
		return
	}
	id, err := s.store.AddIsmsAudit(a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.audit.add", strconv.FormatInt(id, 10), a.Title)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleIsmsAuditUpdate(w http.ResponseWriter, r *http.Request) {
	var a store.IsmsAudit
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		ismsBad(w, err.Error())
		return
	}
	a.ID = ismsPathID(r)
	if a.Status == "done" && a.PerformedAt == 0 {
		a.PerformedAt = time.Now().Unix()
	}
	if !ismsAuditStatuses[a.Status] {
		ismsBad(w, "status planned|done|closed olmalı")
		return
	}
	if err := s.store.UpdateIsmsAudit(a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.audit.update", strconv.FormatInt(a.ID, 10), a.Status)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleIsmsFindingsList(w http.ResponseWriter, r *http.Request) {
	findings, err := s.store.ListIsmsFindings(ismsPathID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, findings)
}

func (s *Server) handleIsmsFindingAdd(w http.ResponseWriter, r *http.Request) {
	var f store.IsmsFinding
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		ismsBad(w, err.Error())
		return
	}
	f.AuditID = ismsPathID(r)
	if strings.TrimSpace(f.Description) == "" {
		ismsBad(w, "description zorunlu")
		return
	}
	if f.Severity == "" {
		f.Severity = "orta"
	}
	if !ismsSeverities[f.Severity] {
		ismsBad(w, "severity dusuk|orta|yuksek olmalı")
		return
	}
	if f.Status == "" {
		f.Status = "open"
	}
	// bulgu referansı audit içinde numaralanır (AUD-<id>-F<n>)
	existing, err := s.store.ListIsmsFindings(f.AuditID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f.Ref = fmt.Sprintf("AUD-%d-F%d", f.AuditID, len(existing)+1)
	id, err := s.store.AddIsmsFinding(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.finding.add", f.Ref, f.Severity)
	writeJSON(w, map[string]any{"ok": true, "id": id, "ref": f.Ref})
}

// handleIsmsFindingUpdate, CAPA güncelleme + kapanış doğrulaması. "closed"
// durumuna geçiş ancak verified üzerinden ve doğrulayan kaydedilerek olur.
func (s *Server) handleIsmsFindingUpdate(w http.ResponseWriter, r *http.Request) {
	var f store.IsmsFinding
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		ismsBad(w, err.Error())
		return
	}
	f.ID = ismsPathID(r)
	if !ismsFindingStatuses[f.Status] {
		ismsBad(w, "status open|in_progress|verified|closed olmalı")
		return
	}
	if f.Status == "verified" || f.Status == "closed" {
		iden := identityFromCtx(r)
		if iden == nil {
			ismsBad(w, "doğrulama için kimlik gerekli")
			return
		}
		if f.VerifiedBy == "" {
			f.VerifiedBy = iden.Username
		}
	}
	if err := s.store.UpdateIsmsFinding(f); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.finding.update", f.Ref, f.Status)
	writeJSON(w, map[string]any{"ok": true})
}

// --- yönetim incelemesi / tedarikçi / süreklilik ---

func (s *Server) handleIsmsMgmtReviewsList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reviews, err := s.store.ListIsmsMgmtReviews(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, reviews)
}

func (s *Server) handleIsmsMgmtReviewAdd(w http.ResponseWriter, r *http.Request) {
	var rev store.IsmsMgmtReview
	if err := json.NewDecoder(r.Body).Decode(&rev); err != nil {
		ismsBad(w, err.Error())
		return
	}
	iden := identityFromCtx(r)
	if iden != nil {
		rev.CreatedBy = iden.Username
	}
	id, err := s.store.AddIsmsMgmtReview(rev)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, iden, "isms.mgmt_review.add", strconv.FormatInt(id, 10), rev.Period)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleIsmsSuppliersList(w http.ResponseWriter, r *http.Request) {
	suppliers, err := s.store.ListIsmsSuppliers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, suppliers)
}

func decodeIsmsSupplier(w http.ResponseWriter, r *http.Request) (store.IsmsSupplier, bool) {
	var sp store.IsmsSupplier
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		ismsBad(w, err.Error())
		return sp, false
	}
	if strings.TrimSpace(sp.Name) == "" {
		ismsBad(w, "name zorunlu")
		return sp, false
	}
	if sp.Criticality == "" {
		sp.Criticality = "orta"
	}
	if !ismsCriticalities[sp.Criticality] {
		ismsBad(w, "criticality dusuk|orta|yuksek|kritik olmalı")
		return sp, false
	}
	return sp, true
}

func (s *Server) handleIsmsSupplierAdd(w http.ResponseWriter, r *http.Request) {
	sp, ok := decodeIsmsSupplier(w, r)
	if !ok {
		return
	}
	id, err := s.store.AddIsmsSupplier(sp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.supplier.add", strconv.FormatInt(id, 10), sp.Name)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleIsmsSupplierUpdate(w http.ResponseWriter, r *http.Request) {
	sp, ok := decodeIsmsSupplier(w, r)
	if !ok {
		return
	}
	sp.ID = ismsPathID(r)
	if err := s.store.UpdateIsmsSupplier(sp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.supplier.update", strconv.FormatInt(sp.ID, 10), sp.Name)
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleIsmsSupplierDelete(w http.ResponseWriter, r *http.Request) {
	id := ismsPathID(r)
	if err := s.store.DeleteIsmsSupplier(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.supplier.delete", strconv.FormatInt(id, 10), "")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleIsmsContinuityList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tests, err := s.store.ListIsmsContinuityTests(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tests)
}

func (s *Server) handleIsmsContinuityAdd(w http.ResponseWriter, r *http.Request) {
	var t store.IsmsContinuityTest
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		ismsBad(w, err.Error())
		return
	}
	if strings.TrimSpace(t.Title) == "" {
		ismsBad(w, "title zorunlu")
		return
	}
	if !ismsContinuityKinds[t.Kind] {
		ismsBad(w, "kind restore|failover|backup_check|tabletop olmalı")
		return
	}
	iden := identityFromCtx(r)
	if iden != nil {
		t.CreatedBy = iden.Username
	}
	id, err := s.store.AddIsmsContinuityTest(t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, iden, "isms.continuity.add", strconv.FormatInt(id, 10), t.Kind+":"+t.Result)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

// --- denetçi paketi (10.8) ---

// handleIsmsAuditorPackage, SoA + risk defteri + kanıtlar + inceleme
// kayıtlarını tek dışa aktarımda verir. format=json (varsayılan, imza
// doğrulamalı Faz 9 zinciriyle birlikte makine-okur) | html (denetçi raporu).
func (s *Server) handleIsmsAuditorPackage(w http.ResponseWriter, r *http.Request) {
	p, err := isms.BuildAuditorPackage(s.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "isms.auditor_package", "",
		fmt.Sprintf("soa=%d risk=%d denetim=%d", len(p.SOA), len(p.Risks), len(p.Audits)))
	stamp := time.Now().Format("20060102-1504")
	if strings.EqualFold(r.URL.Query().Get("format"), "html") {
		htmlBytes, err := p.RenderAuditorHTML()
		if err != nil {
			http.Error(w, "HTML üretilemedi: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=bazntms-isms-denetci-"+stamp+".html")
		w.Write(htmlBytes)
		return
	}
	data, err := p.ToJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=bazntms-isms-denetci-"+stamp+".json")
	w.Write(data)
}
