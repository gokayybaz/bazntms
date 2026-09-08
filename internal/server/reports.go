package server

// Zamanlanmış rapor teslimi + arşiv uçları (Faz 22 S22.19). Zamanlamalar
// scheduled_jobs (kind="report"); üretilen dosyalar <data>/reports/.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/reportjob"
	"github.com/gokayybaz/bazntms/internal/scheduler"
	"github.com/gokayybaz/bazntms/internal/store"
)

// SetReportsDir, zamanlanmış rapor dosyalarının yazıldığı dizini bağlar.
func (s *Server) SetReportsDir(dir string) { s.reportsDir = dir }

func (s *Server) reportMail() reportjob.MailFn {
	return func(to []string, subj string, html []byte) error {
		return alert.SendHTMLMail(s.alerts.Config().Notifiers, to, subj, html)
	}
}

func (s *Server) handleReportArchiveList(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListReportArchive(SiteScope(identityFromCtx(r)), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// dosya yolunu istemciye sızdırma — indirme id üzerinden
	for i := range list {
		list[i].Path = ""
	}
	writeJSON(w, map[string]any{"archive": list})
}

func (s *Server) handleReportArchiveGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	a, err := s.store.ReportArchiveByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if a == nil {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return
	}
	if scope := SiteScope(identityFromCtx(r)); scope != "" && a.Site != scope {
		http.Error(w, "bulunamadı", http.StatusNotFound)
		return
	}
	// path traversal koruması: dosya reportsDir altında olmalı
	clean := filepath.Clean(a.Path)
	if s.reportsDir == "" || !strings.HasPrefix(clean, filepath.Clean(s.reportsDir)+string(filepath.Separator)) {
		http.Error(w, "erişilemez", http.StatusForbidden)
		return
	}
	body, err := os.ReadFile(clean)
	if err != nil {
		http.Error(w, "dosya yok", http.StatusNotFound)
		return
	}
	ct := "text/html; charset=utf-8"
	if a.Format == "pdf" {
		ct = "application/pdf"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename="+filepath.Base(clean))
	_, _ = w.Write(body)
}

// --- zamanlamalar (scheduled_jobs kind="report") ---

func (s *Server) handleReportSchedulesList(w http.ResponseWriter, r *http.Request) {
	all, err := s.store.ListScheduledJobs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	scope := SiteScope(identityFromCtx(r))
	out := []map[string]any{}
	for _, j := range all {
		if j.Kind != "report" {
			continue
		}
		var p reportjob.Payload
		_ = json.Unmarshal([]byte(j.Payload), &p)
		if scope != "" && p.Site != scope {
			continue
		}
		out = append(out, map[string]any{
			"id": j.ID, "spec": j.Spec, "payload": p, "enabled": j.Enabled,
			"last_run_ts": j.LastRunTs, "next_run_ts": j.NextRunTs, "last_status": j.LastStatus,
		})
	}
	writeJSON(w, map[string]any{"schedules": out})
}

func (s *Server) handleReportSchedulePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Spec    string            `json:"spec"`
		Payload reportjob.Payload `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	next, err := scheduler.NextRun(req.Spec, time.Now())
	if err != nil {
		http.Error(w, "geçersiz zamanlama: "+err.Error(), http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	if scope := SiteScope(ident); scope != "" {
		req.Payload.Site = scope
	}
	pj, _ := json.Marshal(req.Payload)
	by := ""
	if ident != nil {
		by = ident.Username
	}
	id, err := s.store.CreateScheduledJob(store.ScheduledJob{
		Kind: "report", Spec: req.Spec, Payload: string(pj), Enabled: true,
		NextRunTs: next.Unix(), CreatedBy: by, CreatedTs: time.Now().Unix(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, ident, "report.schedule.create", fmt.Sprintf("job:%d", id), req.Spec)
	writeJSON(w, map[string]any{"id": id, "next_run_ts": next.Unix()})
}

func (s *Server) handleReportScheduleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteScheduledJob(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, identityFromCtx(r), "report.schedule.delete", fmt.Sprintf("job:%d", id), "")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleReportGenerate(w http.ResponseWriter, r *http.Request) {
	var p reportjob.Payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "geçersiz gövde", http.StatusBadRequest)
		return
	}
	ident := identityFromCtx(r)
	if scope := SiteScope(ident); scope != "" {
		p.Site = scope
	}
	a, err := reportjob.Generate(s.store, s.geo, s.reportsDir, 0, p, s.reportMail())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.audit(r, ident, "report.generate", fmt.Sprintf("archive:%d", a.ID), p.Type)
	a.Path = ""
	writeJSON(w, map[string]any{"archive": a})
}
