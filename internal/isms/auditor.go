package isms

// ISMS yönetişim ürün yüzeyi (Faz 10). Bu paket, Faz 9 kanıt omurgası ile
// store'daki yönetişim kayıtlarını denetçiye sunulabilir tek pakette birleştirir
// (10.8): SoA + risk defteri + varlık envanteri + politikalar + iç denetim +
// CAPA + yönetim incelemeleri + tedarikçiler + süreklilik testleri + uyum
// durumu. HTML çıktısı yazdırma dostu denetçi raporudur.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/version"
)

// AuditorPackage, tek tıkla dışa aktarılan denetçi paketi.
type AuditorPackage struct {
	GeneratedAt int64                      `json:"generated_at"`
	Hub         map[string]any             `json:"hub"` // sürüm bilgisi
	SOACounts   map[string]int             `json:"soa_counts"`
	SOA         []store.IsmsSoaItem        `json:"soa"`
	Assets      []store.IsmsAsset          `json:"assets"`
	Risks       []store.IsmsRisk           `json:"risks"`
	Policies    []store.IsmsPolicy         `json:"policies"`
	Audits      []store.IsmsAudit          `json:"audits"`
	Findings    []store.IsmsFinding        `json:"findings"`
	MgmtReviews []store.IsmsMgmtReview     `json:"mgmt_reviews"`
	Suppliers   []store.IsmsSupplier       `json:"suppliers"`
	Continuity  []store.IsmsContinuityTest `json:"continuity_tests"`
	Compliance  ComplianceSummary          `json:"compliance"`
}

// ComplianceSummary, Faz 9 kanıt omurgasının paket anındaki durumu.
type ComplianceSummary struct {
	LogRecords    int64  `json:"log_records"`
	LastRecordTs  int64  `json:"last_record_ts"`
	LastDailyRoot string `json:"last_daily_root"`
	LastDailyDay  string `json:"last_daily_day"`
	LastDailyTSA  string `json:"last_daily_tsa"`
	ChainVerified bool   `json:"chain_verified"`
	ChainBrokenAt int64  `json:"chain_broken_at"`
	ChainChecked  int    `json:"chain_checked"`
}

// BuildAuditorPackage, tüm yönetişim verilerini store'dan toplar.
func BuildAuditorPackage(st store.Store) (*AuditorPackage, error) {
	p := &AuditorPackage{GeneratedAt: time.Now().Unix(), Hub: version.Info()}

	total, applicable, implemented, verified, excluded, err := st.IsmsSoaCounts()
	if err != nil {
		return nil, err
	}
	p.SOACounts = map[string]int{
		"total": total, "applicable": applicable,
		"implemented": implemented, "verified": verified, "excluded": excluded,
	}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"soa", func() (err error) { p.SOA, err = st.ListIsmsSoa(); return }},
		{"assets", func() (err error) { p.Assets, err = st.ListIsmsAssets(); return }},
		{"risks", func() (err error) { p.Risks, err = st.ListIsmsRisks(); return }},
		{"policies", func() (err error) { p.Policies, err = st.ListIsmsPolicies(); return }},
		{"audits", func() (err error) { p.Audits, err = st.ListIsmsAudits(); return }},
		{"findings", func() (err error) { p.Findings, err = st.ListIsmsFindings(0); return }},
		{"mgmt_reviews", func() (err error) { p.MgmtReviews, err = st.ListIsmsMgmtReviews(200); return }},
		{"suppliers", func() (err error) { p.Suppliers, err = st.ListIsmsSuppliers(); return }},
		{"continuity_tests", func() (err error) { p.Continuity, err = st.ListIsmsContinuityTests(200); return }},
	}
	for _, step := range steps {
		if err := step.fn(); err != nil {
			return nil, fmt.Errorf("%s: %w", step.name, err)
		}
	}

	p.Compliance.LogRecords, p.Compliance.LastRecordTs, err = st.ComplianceStats()
	if err != nil {
		return nil, err
	}
	if cp, err := st.LatestLogCheckpoint("daily"); err != nil {
		return nil, err
	} else if cp != nil {
		p.Compliance.LastDailyRoot = cp.Root
		p.Compliance.LastDailyDay = time.Unix(cp.BucketStart, 0).Format("2006-01-02")
		p.Compliance.LastDailyTSA = cp.TSAStatus
	}
	ok, brokenAt, checked, err := st.VerifyAuditChain()
	if err != nil {
		return nil, err
	}
	p.Compliance.ChainVerified, p.Compliance.ChainBrokenAt, p.Compliance.ChainChecked = ok, brokenAt, checked
	return p, nil
}

// ToJSON, paketi dosyaya yazılabilir JSON'a çevirir.
func (p *AuditorPackage) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// --- denetçi HTML raporu ---

var tplFuncs = template.FuncMap{
	"ts": func(sec int64) string {
		if sec <= 0 {
			return "—"
		}
		return time.Unix(sec, 0).Format("02.01.2006")
	},
	"tsfull": func(sec int64) string {
		if sec <= 0 {
			return "—"
		}
		return time.Unix(sec, 0).Format("02.01.2006 15:04")
	},
}

var auditorTpl = template.Must(template.New("auditor").Funcs(tplFuncs).Parse(`<!DOCTYPE html>
<html lang="tr"><head><meta charset="utf-8">
<title>bazNTMS ISMS Denetçi Paketi</title>
<style>
 body{font-family:-apple-system,'Segoe UI',sans-serif;max-width:1100px;margin:24px auto;padding:0 16px;color:#1e293b;font-size:13px}
 h1{font-size:20px;border-bottom:2px solid #0891b2;padding-bottom:8px}
 h2{font-size:15px;margin-top:28px;color:#0e7490;border-bottom:1px solid #cbd5e1;padding-bottom:4px}
 table{border-collapse:collapse;width:100%;margin:8px 0}
 th,td{border:1px solid #cbd5e1;padding:4px 7px;text-align:left;vertical-align:top}
 th{background:#f1f5f9;font-size:11px;text-transform:uppercase;letter-spacing:.04em}
 td.num{text-align:right;font-family:ui-monospace,monospace}
 .ok{color:#059669;font-weight:600}.warn{color:#d97706;font-weight:600}.bad{color:#dc2626;font-weight:600}
 .meta{color:#64748b;font-size:11px}
 .pill{display:inline-block;border-radius:10px;padding:0 8px;font-size:10px;border:1px solid #cbd5e1;background:#f8fafc}
 @media print{h2{page-break-after:avoid}table{page-break-inside:auto;font-size:10px}}
</style></head><body>
<h1>bazNTMS — ISMS Denetçi Paketi (ISO 27001)</h1>
<p class="meta">Üretim: {{.GenTime}} · Hub sürümü: {{.HubVersion}} · Zincir doğrulaması:
{{if .P.Compliance.ChainVerified}}<span class="ok">SAĞLAM</span>{{else}}<span class="bad">BOZUK (seq {{.P.Compliance.ChainBrokenAt}})</span>{{end}}
({{.P.Compliance.ChainChecked}} kayıt) · İmzalı log: {{.P.Compliance.LogRecords}}</p>

<h2>Statement of Applicability</h2>
<p>Toplam {{.P.SOACounts.total}} kontrol · Uygulanacak {{.P.SOACounts.applicable}} ·
Uygulanan/doğrulanmış {{.P.SOACounts.implemented}} · Doğrulanmış {{.P.SOACounts.verified}} ·
Hariç {{.P.SOACounts.excluded}}</p>
<table><tr><th>Kontrol</th><th>Başlık</th><th>Karar</th><th>Durum</th><th>Kanıt / Gerekçe</th></tr>
{{range .P.SOA}}<tr><td><b>{{.ControlID}}</b></td><td>{{.Title}}</td>
<td>{{if .Applicable}}<span class="pill">uygulanacak</span>{{else}}<span class="pill warn">hariç</span>{{end}}</td>
<td>{{.Status}}</td><td>{{if .Evidence}}{{.Evidence}}{{end}}{{if .Justification}}<br><i>{{.Justification}}</i>{{end}}</td></tr>
{{end}}</table>

<h2>Risk Defteri</h2>
{{if .P.Risks}}<table><tr><th>#</th><th>Varlık</th><th>Tehdit</th><th>Zaafiyet</th><th>Etki×Olasılık</th><th>Muamele</th><th>Kalıntı</th><th>Durum</th></tr>
{{range .P.Risks}}<tr><td class="num">{{.ID}}</td><td>{{index $.AssetName .AssetID}}</td><td>{{.Threat}}</td><td>{{.Vulnerability}}</td>
<td class="num"><b>{{.Score}}</b> ({{.Impact}}×{{.Likelihood}})</td><td>{{.Treatment}}</td>
<td>{{if .ResScore}}<b>{{.ResScore}}</b> ({{.ResImpact}}×{{.ResLikelihood}}){{else}}—{{end}}</td><td>{{.Status}}</td></tr>
{{end}}</table>{{else}}<p class="meta">Risk kaydı yok.</p>{{end}}

<h2>Politikalar</h2>
{{if .P.Policies}}<table><tr><th>Ref</th><th>Başlık</th><th>Sürüm</th><th>Durum</th><th>Onay</th><th>Sonraki İnceleme</th></tr>
{{range .P.Policies}}<tr><td>{{.Ref}}</td><td>{{.Title}}</td><td>v{{.Version}}</td><td>{{.Status}}</td>
<td>{{if .ApprovedBy}}{{.ApprovedBy}} · {{ts .ApprovedAt}}{{else}}—{{end}}</td>
<td>{{ts .NextReview}}</td></tr>{{end}}</table>{{else}}<p class="meta">Politika kaydı yok.</p>{{end}}

<h2>İç Denetim ve Bulgular</h2>
{{if .P.Audits}}<table><tr><th>#</th><th>Başlık</th><th>Kapsam</th><th>Plan</th><th>Denetçi</th><th>Durum</th><th>Bulgu</th></tr>
{{range .P.Audits}}<tr><td class="num">{{.ID}}</td><td>{{.Title}}</td><td>{{.Scope}}</td><td>{{.PlannedDate}}</td><td>{{.Auditor}}</td><td>{{.Status}}</td>
<td>{{index $.FindingCount .ID}}</td></tr>{{end}}</table>{{else}}<p class="meta">Denetim kaydı yok.</p>{{end}}
{{if .P.Findings}}<table><tr><th>Ref</th><th>Bulgu</th><th>Şiddet</th><th>Kontrol</th><th>CAPA</th><th>Sorumlu</th><th>Vade</th><th>Durum</th></tr>
{{range .P.Findings}}<tr><td>{{.Ref}}</td><td>{{.Description}}</td><td>{{.Severity}}</td><td>{{.ControlID}}</td><td>{{.CAPA}}</td>
<td>{{.CAPAOwner}}</td><td>{{.CAPADue}}</td><td>{{.Status}}{{if .VerifiedBy}} ({{.VerifiedBy}}){{end}}</td></tr>{{end}}</table>{{end}}

<h2>Yönetim İncelemeleri</h2>
{{if .P.MgmtReviews}}<table><tr><th>Tarih</th><th>Dönem</th><th>Katılımcılar</th><th>Kararlar</th><th>Aksiyonlar</th></tr>
{{range .P.MgmtReviews}}<tr><td>{{tsfull .Ts}}</td><td>{{.Period}}</td><td>{{.Attendees}}</td><td>{{.Decisions}}</td><td>{{.Actions}}</td></tr>{{end}}</table>
{{else}}<p class="meta">Yönetim incelemesi kaydı yok.</p>{{end}}

<h2>Tedarikçiler (A.5.19-22)</h2>
{{if .P.Suppliers}}<table><tr><th>Tedarikçi</th><th>Hizmet</th><th>Kritiklik</th><th>Veri Erişimi</th><th>Risk</th><th>Sonraki İnceleme</th></tr>
{{range .P.Suppliers}}<tr><td>{{.Name}}</td><td>{{.Service}}</td><td>{{.Criticality}}</td><td>{{.DataAccess}}</td><td>{{.Risk}}</td>
<td>{{if .NextReview}}{{ts .NextReview}}{{else}}—{{end}}</td></tr>{{end}}</table>{{else}}<p class="meta">Tedarikçi kaydı yok.</p>{{end}}

<h2>Süreklilik Testleri (BCDR)</h2>
{{if .P.Continuity}}<table><tr><th>Tarih</th><th>Tür</th><th>Başlık</th><th>Sonuç</th><th>Kanıt</th></tr>
{{range .P.Continuity}}<tr><td>{{ts .PerformedAt}}</td><td>{{.Kind}}</td><td>{{.Title}}</td>
<td>{{if eq .Result "basarili"}}<span class="ok">{{.Result}}</span>{{else if eq .Result "basarisiz"}}<span class="bad">{{.Result}}</span>{{else}}{{.Result}}{{end}}</td>
<td>{{.Evidence}}</td></tr>{{end}}</table>{{else}}<p class="meta">Süreklilik testi kaydı yok.</p>{{end}}

<p class="meta">Varlık envanteri: {{len .P.Assets}} kayıt (cihaz/agent/site otomatik senkron + manuel).
Bu paket hub tarafından {{.GenTime}} anında üretildi; Faz 9 imzalı log zinciriyle birlikte kanıt olarak sunulur.</p>
</body></html>`))

// RenderAuditorHTML, paketi yazdırma dostu HTML raporuna çevirir.
func (p *AuditorPackage) RenderAuditorHTML() ([]byte, error) {
	assetNames := map[int64]string{}
	for _, a := range p.Assets {
		assetNames[a.ID] = a.Kind + ":" + a.Name
	}
	findingCounts := map[int64]int{}
	for _, f := range p.Findings {
		findingCounts[f.AuditID]++
	}
	data := struct {
		P            *AuditorPackage
		GenTime      string
		HubVersion   string
		AssetName    map[int64]string
		FindingCount map[int64]int
	}{
		P: p, GenTime: time.Unix(p.GeneratedAt, 0).Format("02.01.2006 15:04"),
		HubVersion: fmt.Sprint(p.Hub["version"]), AssetName: assetNames, FindingCount: findingCounts,
	}
	var buf bytes.Buffer
	if err := auditorTpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
