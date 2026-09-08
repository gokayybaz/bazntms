package report

// Kurumsal rapor PDF çıktısı (Faz 22 S22.20) — pdf.go'daki pdfRenderer
// yardımcılarını kullanır. SLA / kapasite-banding / top listeler.

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// RenderEnterprisePDF, kurumsal raporu PDF belgesi olarak üretir.
func (d *EnterpriseData) RenderEnterprisePDF() ([]byte, error) {
	p := &pdfRenderer{fpdf.New("P", "mm", "A4", "")}
	p.SetMargins(margin, 14, margin)
	p.SetAutoPageBreak(true, 18)
	p.AliasNbPages("")
	p.AddUTF8FontFromBytes("go", "", goregular.TTF)
	p.AddUTF8FontFromBytes("go", "B", gobold.TTF)
	p.SetFooterFunc(func() {
		p.SetY(-14)
		p.SetFont("go", "", 8)
		p.SetTextColor(120, 130, 145)
		p.CellFormat(0, 6, "bazNTMS - kurumsal rapor (SLA/kapasite)", "", 0, "L", false, 0, "")
		p.CellFormat(0, 6, fmt.Sprintf("sayfa %d/%d", p.PageNo(), p.PageCount()), "", 0, "R", false, 0, "")
	})
	p.AddPage()

	p.SetFont("go", "B", 20)
	p.SetTextColor(30, 41, 59)
	p.CellFormat(0, 10, "bazNTMS — Kurumsal Rapor", "", 2, "L", false, 0, "")
	p.SetFont("go", "", 9.5)
	p.SetTextColor(100, 116, 139)
	scope := "Filo geneli"
	if d.Site != "" {
		scope = "Saha: " + d.Site
	}
	p.CellFormat(0, 5, fmt.Sprintf("%s · son %d gün · Üretim: %s", scope, d.Days, d.GeneratedAt.Format("02.01.2006 15:04")), "", 2, "L", false, 0, "")
	p.setRule()
	p.Ln(3)

	if d.Empty {
		p.SetFont("go", "", 9.5)
		p.SetTextColor(146, 64, 14)
		p.MultiCell(0, 5, "Bu dönemde filo trafiği/telemetri kaydı bulunamadı.", "", "L", false)
		p.SetTextColor(30, 41, 59)
		p.Ln(2)
	}

	// SLA
	p.section("SLA")
	p.kvLine("Agent uptime", fmt.Sprintf("%.1f%% (%d/%d)", d.AgentUptime, d.AgentOnline, d.AgentTotal))
	p.kvLine("Cihaz sağlığı", fmt.Sprintf("%.1f%% (%d/%d)", d.DeviceHealth, d.DeviceOK, d.DeviceTotal))
	p.kvLine("Arayüz iskarta / hata", fmt.Sprintf("%d / %d", d.IfaceDiscards, d.IfaceErrors))
	total := 0
	for _, v := range d.AlertCounts {
		total += v
	}
	p.kvLine("Uyarı sayısı", fmt.Sprintf("%d", total))
	if d.Target.Set() {
		p.Ln(1)
		sc := []float64{60, 35, 35, 0}
		p.tableHeader(sc, []string{"SLA Hedefi", "Hedef", "Gerçek", "Durum"})
		st := func(breach bool) string {
			if breach {
				return "İHLAL"
			}
			return "karşılandı"
		}
		if d.Target.AgentUptimePct > 0 {
			p.tableRow(sc, []string{"Agent uptime", fmt.Sprintf(">= %.1f%%", d.Target.AgentUptimePct), fmt.Sprintf("%.1f%%", d.AgentUptime), st(d.UptimeBreach)}, false)
		}
		if d.Target.DeviceHealthPct > 0 {
			p.tableRow(sc, []string{"Cihaz sağlığı", fmt.Sprintf(">= %.1f%%", d.Target.DeviceHealthPct), fmt.Sprintf("%.1f%%", d.DeviceHealth), st(d.DeviceBreach)}, true)
		}
		if d.Target.IfaceErrCeiling > 0 {
			p.tableRow(sc, []string{"Arayüz iskarta+hata 24s", fmt.Sprintf("<= %d", d.Target.IfaceErrCeiling), fmt.Sprintf("%d", d.IfaceDiscards+d.IfaceErrors), st(d.IfaceBreach)}, false)
		}
	}

	// Kapasite ve banding
	p.section("Kapasite ve Banding")
	p.kvLine("Toplam trafik", fmt.Sprintf("%.1f GB", d.TotalGB))
	if d.PrevGB > 0 {
		p.kvLine("Büyüme (önceki dönem)", fmt.Sprintf("%+.1f%%", d.GrowthPct))
	}
	p.kvLine("Ortalama verim", bits(d.AvgBps))
	p.kvLine("Zirve verim", bits(d.PeakBps))
	p.Ln(2)
	cols := []float64{40, 60, 0}
	p.tableHeader(cols, []string{"Bant", "Toplam verim", "Not"})
	p.tableRow(cols, []string{"p50 (tipik)", bits(d.P50Bps), "günlük profil"}, false)
	p.tableRow(cols, []string{"p95 (zirve)", bits(d.P95Bps), "kalıcı kapasite"}, true)
	p.tableRow(cols, []string{"p99 (sıçrama)", bits(d.P99Bps), "burst planı"}, false)

	// En yoğun uç noktalar
	if len(d.TopEndpoints) > 0 {
		p.section("En Yoğun Uç Noktalar")
		ec := []float64{70, 55, 0}
		p.tableHeader(ec, []string{"IP", "Gelen", "Giden"})
		for i, e := range d.TopEndpoints {
			p.tableRow(ec, []string{e.IP, bytesFmt(e.BytesIn), bytesFmt(e.BytesOut)}, i%2 == 1)
		}
	}
	// Süreç bazlı trafik
	if len(d.TopProcesses) > 0 {
		p.section("Süreç Bazlı Trafik (Agentlar)")
		pc := []float64{70, 45, 45}
		p.tableHeader(pc, []string{"Süreç", "İndirilen", "Gönderilen"})
		for i, pr := range d.TopProcesses {
			p.tableRow(pc, []string{pr.Process, bytesFmt(pr.BytesIn), bytesFmt(pr.BytesOut)}, i%2 == 1)
		}
	}

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
