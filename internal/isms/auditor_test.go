package isms

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/store"
)

// TestBuildAuditorPackage, denetçi paketinin tüm yönetişim verilerini
// topladığını ve Faz 9 zincir doğrulamasını taşıdığını doğrular (10.8).
func TestBuildAuditorPackage(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store acilamadi: %v", err)
	}
	defer st.Close()

	now := int64(1725062400)
	if _, err := st.AddIsmsRisk(store.IsmsRisk{Threat: "test tehdidi", Impact: 4, Likelihood: 4}); err != nil {
		t.Fatalf("risk: %v", err)
	}
	if _, err := st.AddIsmsPolicy(store.IsmsPolicy{Ref: "POL-001", Title: "Test Politikası", Status: "published", PublishedAt: now}); err != nil {
		t.Fatalf("politika: %v", err)
	}
	aid, err := st.AddIsmsAudit(store.IsmsAudit{Title: "denetim", Status: "done"})
	if err != nil {
		t.Fatalf("denetim: %v", err)
	}
	if _, err := st.AddIsmsFinding(store.IsmsFinding{AuditID: aid, Description: "bulgu", Severity: "orta"}); err != nil {
		t.Fatalf("bulgu: %v", err)
	}
	if _, err := st.AddIsmsContinuityTest(store.IsmsContinuityTest{Kind: "restore", Title: "tatbikat", Result: "basarili"}); err != nil {
		t.Fatalf("sureklilik: %v", err)
	}
	if _, err := st.AppendComplianceLog(store.ComplianceLog{
		Ts: now, SourceType: "manual", Category: "event", Message: "paket test kaydı",
	}); err != nil {
		t.Fatalf("compliance log: %v", err)
	}

	p, err := BuildAuditorPackage(st)
	if err != nil {
		t.Fatalf("paket: %v", err)
	}
	if p.SOACounts["total"] != 93 {
		t.Fatalf("SoA 93 olmali: %d", p.SOACounts["total"])
	}
	if len(p.Risks) != 1 || len(p.Policies) != 1 || len(p.Audits) != 1 || len(p.Findings) != 1 || len(p.Continuity) != 1 {
		t.Fatalf("paket eksik: risk=%d politika=%d denetim=%d bulgu=%d sureklilik=%d",
			len(p.Risks), len(p.Policies), len(p.Audits), len(p.Findings), len(p.Continuity))
	}
	// Faz 5 audit zinciri boş olsa da sağlam raporlanır (checked=0 trivially ok)
	if !p.Compliance.ChainVerified {
		t.Fatalf("zincir dogrulamasi: %+v", p.Compliance)
	}
	if p.Compliance.LastDailyRoot != "" {
		t.Fatalf("daily checkpoint yokken root bos olmali")
	}

	// JSON çıktı yazılabilir olmalı
	data, err := p.ToJSON()
	if err != nil || !strings.Contains(string(data), `"soa_counts"`) {
		t.Fatalf("json cikti: %v", err)
	}

	// HTML raporu tüm bölümleri içermeli
	html, err := p.RenderAuditorHTML()
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	for _, bolum := range []string{"Statement of Applicability", "Risk Defteri", "Politikalar",
		"İç Denetim", "Yönetim İncelemeleri", "Tedarikçiler", "Süreklilik Testleri", "test tehdidi"} {
		if !strings.Contains(string(html), bolum) {
			t.Fatalf("html %q bolumunu icermiyor", bolum)
		}
	}
}
