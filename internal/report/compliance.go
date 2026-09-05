package report

// ISO 27001 kontrol haritası + uyum durumu raporu (Faz 9.5).
// Annex A kontrolü → bazNTMS karşılığı → otomatik kanıt bağlantıları.
// GET /api/report?type=compliance ile HTML üretilir (denetçiye sunulur).

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// Control, haritadaki tek satır.
type Control struct {
	ID       string `json:"id"`       // ör. A.8.15
	Title    string `json:"title"`    // kontrol adı
	Covered  string `json:"covered"`  // bazNTMS karşılığı
	Evidence string `json:"evidence"` // kanıt bağlantısı (uç/tablo/komut)
}

// ComplianceData, rapor modeli.
type ComplianceData struct {
	GeneratedAt time.Time `json:"generated_at"`
	Controls    []Control `json:"controls"`

	// 5651 motoru durumu (store'dan)
	LogTotal   int64  `json:"log_total"`
	LastLogTs  int64  `json:"last_log_ts"`
	LastHourly string `json:"last_hourly"` // "yok" veya root özeti
	LastDaily  string `json:"last_daily"`  // "yok" veya durum özeti
	ChainNote  string `json:"chain_note"`
}

// controlMap, ISO 27001:2022 Annex A (uyumla ilgili kontrol alt kümesi).
var controlMap = []Control{
	{"A.8.15", "Loglama", "Tüm syslog/agent/cihaz olayları hash-zincirli compliance_logs tablosuna yazılır; saatlik Merkle checkpoint'leri bütünlüğü sağlar", "compliance_logs + log_checkpoints · GET /api/v1/compliance/status"},
	{"A.8.16", "İzleme faaliyetleri", "Anomali motoru (z-skoru baseline), eşik uyarıları, VPN/SD-WAN izleme; olaylar bildirim kanallarına dağıtılır", "alert_events · GET /api/alerts/events"},
	{"A.8.17", "Saat senkronizasyonu", "Zaman sapması alarmı: checkpoint saatleri sistem saatinden ileri saptığında uyarı üretilir; NTP senkronu operasyon gereği", "time_drift uyarıları · alert_events"},
	{"A.5.28", "Delil toplama", "Tarih aralıklı delil paketi: loglar + Merkle zinciri + RFC 3161 zaman damgası + manifest imzası + doğrulama raporu", "GET /api/v1/compliance/evidence · bazntmsctl verify"},
	{"A.5.33", "Kayıtların korunması", "Günlük imzalı paketler WORM dizinine yazılır; ham loglar 730 gün saklanır; kayıtlar append-only zincirlidir", "worm_dir paketleri · log_checkpoints"},
	{"A.5.34 / A.8.11", "KIŞV gizliliği / veri maskeleme", "Delil paketinde opsiyonel PII maskeleme (IP/MAC/kullanıcı); maskeleme politikası paket içinde beyan edilir", "evidence.masked alanı"},
	{"A.8.2", "Ayrıcalıklı erişim", "RBAC rolleri (admin/netops/analyst/viewer + site scope), OIDC SSO, API token'ları; tüm yönetim işlemleri hash-zincirli audit_events'te", "users · api_tokens · audit_events · GET /api/v1/audit/verify"},
	{"A.8.15 / A.5.25", "Log incelemesi ve olay değerlendirmesi", "İmzalı log inceleme tutanakları; inceleme kayıtları audit zincirine yazılır", "POST /api/v1/compliance/reviews · compliance_reviews"},
	{"A.8.2 / A.5.18", "Erişim hakları gözden geçirme", "Periyodik erişim incelemesi tutanakları (onaylı)", "compliance_reviews (kind=access)"},
	{"A.5.29 / A.5.30", "Kesinti ve süreklilik", "DR runbook, pg backup/restore script'leri, agent çoklu-hub failover + offline kuyruk", "docs/DR-RUNBOOK.md · deploy/scripts"},
	{"A.8.8", "Teknik güvenlik açıkları", "CI'da govulncheck, trivy taraması, syft SBOM, cosign imzalı artefaktlar", ".github/workflows · release imzaları"},
	{"A.8.9", "Yapılandırma yönetimi", "Cihaz/uygulama yapılandırma değişiklikleri audit zincirinde; sürüm bilgisi /api/v1/version", "audit_events (action=device.*/alerts.*)"},
}

// BuildComplianceData, raporu store durumuyla doldurur.
func BuildComplianceData(st store.Store) (*ComplianceData, error) {
	d := &ComplianceData{
		GeneratedAt: time.Now(),
		Controls:    controlMap,
	}
	total, lastTs, err := st.ComplianceStats()
	if err == nil {
		d.LogTotal, d.LastLogTs = total, lastTs
	}
	if cp, _ := st.LatestLogCheckpoint("hourly"); cp != nil {
		d.LastHourly = fmt.Sprintf("%s · kök %s… · %d kayıt",
			time.Unix(cp.BucketStart, 0).Format("02.01 15:04"), cp.Root[:12], cp.RecordCount)
	} else {
		d.LastHourly = "yok"
	}
	if cp, _ := st.LatestLogCheckpoint("daily"); cp != nil {
		d.LastDaily = fmt.Sprintf("%s · TSA=%s · imza=%v",
			time.Unix(cp.BucketStart, 0).Format("02.01.2006"), cp.TSAStatus, cp.Signature != "")
	} else {
		d.LastDaily = "yok"
	}
	if ok, broken, checked, err := st.VerifyAuditChain(); err == nil {
		if ok {
			d.ChainNote = fmt.Sprintf("audit zinciri sağlam (%d kayıt)", checked)
		} else {
			d.ChainNote = fmt.Sprintf("audit zinciri BOZUK: kayıt #%d", broken)
		}
	}
	return d, nil
}

const complianceTpl = `<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<title>bazNTMS — Uyumluluk Raporu (5651 + ISO 27001)</title>
<style>
  body { font-family: -apple-system, "Segoe UI", Roboto, sans-serif; color: #1e293b; margin: 0; background: #f8fafc; }
  .page { max-width: 900px; margin: 24px auto; background: #fff; padding: 40px 48px; border: 1px solid #e2e8f0; }
  header { border-bottom: 3px solid #0e7490; padding-bottom: 14px; margin-bottom: 20px; }
  h1 { margin: 0; font-size: 22px; }
  .sub { color: #64748b; font-size: 12px; margin-top: 4px; }
  h2 { font-size: 14px; text-transform: uppercase; letter-spacing: 1px; color: #0e7490; border-bottom: 1px solid #e2e8f0; padding-bottom: 6px; }
  table { width: 100%; border-collapse: collapse; font-size: 12px; }
  th { text-align: left; background: #f1f5f9; padding: 6px 8px; border-bottom: 2px solid #e2e8f0; font-size: 10.5px; text-transform: uppercase; color: #64748b; }
  td { padding: 6px 8px; border-bottom: 1px solid #e2e8f0; vertical-align: top; }
  code { font-family: monospace; font-size: 11px; color: #0e7490; background: #f1f5f9; padding: 1px 4px; border-radius: 3px; }
  .kpi { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; margin: 12px 0; }
  .kpi div { border: 1px solid #e2e8f0; padding: 10px 12px; }
  .kpi b { display: block; font-size: 15px; margin-top: 2px; }
  .kpi span { font-size: 10.5px; color: #64748b; text-transform: uppercase; }
  footer { margin-top: 26px; color: #64748b; font-size: 11px; }
</style>
</head>
<body>
<div class="page">
  <header>
    <h1>bazNTMS — Uyumluluk Raporu</h1>
    <div class="sub">5651 log imzalama durumu + ISO 27001:2022 Annex A kontrol haritası · {{.GeneratedAt.Format "02.01.2006 15:04"}}</div>
  </header>

  <h2>5651 Log İmzalama Durumu</h2>
  <div class="kpi">
    <div><span>İmzalı Kayıt</span><b>{{.LogTotal}}</b></div>
    <div><span>Son Saatlik Checkpoint</span><b>{{.LastHourly}}</b></div>
    <div><span>Son Günlük Mühür</span><b>{{.LastDaily}}</b></div>
  </div>
  <p style="font-size:12px">{{.ChainNote}} · son kayıt zamanı: {{if gt .LastLogTs 0}}{{unix .LastLogTs}}{{else}}—{{end}}</p>
  <p style="font-size:11px;color:#64748b">Hukuki zaman referansı RFC 3161 nitelikli zaman damgasıdır (TSA). Bu rapor teknik kanıt sunar; uyumluluk beyanı için hukuki danışmanlık gereklidir.</p>

  <h2>ISO 27001 Kontrol Haritası</h2>
  <table>
    <tr><th style="width:90px">Kontrol</th><th style="width:170px">Başlık</th><th>bazNTMS Karşılığı</th><th style="width:250px">Kanıt</th></tr>
    {{range .Controls}}
    <tr><td><b>{{.ID}}</b></td><td>{{.Title}}</td><td>{{.Covered}}</td><td><code>{{.Evidence}}</code></td></tr>
    {{end}}
  </table>

  <footer>bazNTMS uyum raporu · detaylı delil için /api/v1/compliance/evidence</footer>
</div>
</body>
</html>`

// RenderComplianceHTML, raporu HTML üretir.
func RenderComplianceHTML(d *ComplianceData) ([]byte, error) {
	funcs := template.FuncMap{
		"unix": func(ts int64) string { return time.Unix(ts, 0).Format("02.01.2006 15:04:05") },
	}
	t, err := template.New("compliance").Funcs(funcs).Parse(complianceTpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
