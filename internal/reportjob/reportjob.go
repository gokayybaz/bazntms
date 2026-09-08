// Package reportjob, zamanlanmış (ve elle tetiklenen) kurumsal rapor
// üretimi + arşivleme + e-posta teslimidir (Faz 22 S22.19).
//
// Rapor içeriği dosya sistemine yazılır (<data>/reports/), yol + meta
// store.ReportArchive'a; DB PDF blob'larıyla şişmez (ADR 0008 Karar 4).
package reportjob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/report"
	"github.com/gokayybaz/bazntms/internal/scheduler"
	"github.com/gokayybaz/bazntms/internal/store"
)

// Payload, scheduled_jobs.payload_json (kind="report").
type Payload struct {
	Type   string   `json:"type"`   // enterprise | compliance | traffic
	Days   int      `json:"days"`   // rapor penceresi
	Site   string   `json:"site"`   // enterprise: saha kırılımı (S22.20)
	Format string   `json:"format"` // html | pdf
	Email  []string `json:"email"`  // teslim alıcıları (SMTP alert config'ten)
}

// MailFn, üretilen raporu e-postalar. nil → e-posta atlanır.
type MailFn func(to []string, subject string, htmlBody []byte) error

// Handler, scheduler'a kaydedilecek "report" iş türü işleyicisi.
func Handler(st store.Store, geo *geoip.Resolver, dir string, mail MailFn) scheduler.Handler {
	return func(ctx context.Context, payloadJSON string) error {
		var p Payload
		if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
			return fmt.Errorf("payload: %w", err)
		}
		_, err := Generate(st, geo, dir, 0, p, mail)
		return err
	}
}

// Generate, tek bir raporu üretir, dosyaya yazar, arşivler ve (alıcı + mail
// varsa) e-postalar. jobID 0 → elle tetiklenmiş.
func Generate(st store.Store, geo *geoip.Resolver, dir string, jobID int64, p Payload, mail MailFn) (*store.ReportArchive, error) {
	if p.Days <= 0 {
		p.Days = 30
	}
	body, format, empty, err := render(st, geo, p)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	status := "ok"
	if empty {
		status = "boş dönem — teslim edilmedi"
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s.%s", safe(p.Type), now.Format("20060102-150405"), format)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}

	delivered := ""
	if !empty && mail != nil && len(p.Email) > 0 {
		subj := fmt.Sprintf("bazNTMS %s raporu — %s", p.Type, now.Format("02.01.2006"))
		if err := mail(p.Email, subj, body); err != nil {
			status = "e-posta hatası: " + err.Error()
		} else {
			delivered = strings.Join(p.Email, ",")
		}
	}

	a := store.ReportArchive{
		Kind: p.Type, Site: p.Site, Days: p.Days, Format: format, Path: path,
		Size: int64(len(body)), GeneratedTs: now.Unix(), DeliveredTo: delivered,
		Status: status, JobID: jobID,
	}
	id, err := st.InsertReportArchive(a)
	if err != nil {
		return nil, err
	}
	a.ID = id
	return &a, nil
}

func render(st store.Store, geo *geoip.Resolver, p Payload) (body []byte, format string, empty bool, err error) {
	format = "html"
	switch p.Type {
	case "enterprise":
		d, e := report.BuildEnterprise(st, p.Days, p.Site)
		if e != nil {
			return nil, "", false, e
		}
		if p.Format == "pdf" {
			b, pe := d.RenderEnterprisePDF()
			return b, "pdf", d.Empty, pe
		}
		b, re := d.RenderEnterpriseHTML()
		return b, "html", d.Empty, re
	case "compliance":
		d, e := report.BuildComplianceData(st)
		if e != nil {
			return nil, "", false, e
		}
		b, re := report.RenderComplianceHTML(d)
		return b, "html", false, re
	case "traffic", "":
		d, e := report.Build(st, geo, p.Days)
		if e != nil {
			return nil, "", false, e
		}
		if p.Format == "pdf" {
			b, pe := d.RenderPDF()
			return b, "pdf", d.Empty, pe
		}
		b, re := d.RenderHTML()
		return b, "html", d.Empty, re
	}
	return nil, "", false, fmt.Errorf("bilinmeyen rapor türü: %q", p.Type)
}

func safe(s string) string {
	if s == "" {
		return "traffic"
	}
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, strings.ToLower(s))
}
