package report

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	pageW  = 210.0 // A4 mm
	margin = 14.0

	accentR, accentG, accentB = 14, 116, 144 // #0e7490
)

type pdfRenderer struct {
	*fpdf.Fpdf
}

// RenderPDF, raporu Turkce karakter destekli bir PDF belgesi olarak uretir.
func (d *Data) RenderPDF() ([]byte, error) {
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
		p.CellFormat(0, 6, "bazNTMS - otomatik uretilmis rapor", "", 0, "L", false, 0, "")
		p.CellFormat(0, 6, fmt.Sprintf("sayfa %d/%d", p.PageNo(), p.PageCount()), "", 0, "R", false, 0, "")
	})

	p.AddPage()

	// baslik
	p.SetFont("go", "B", 20)
	p.SetTextColor(30, 41, 59)
	p.CellFormat(0, 10, "bazNTMS — Ağ Trafik Raporu", "", 2, "L", false, 0, "")
	p.SetFont("go", "", 9.5)
	p.SetTextColor(100, 116, 139)
	p.CellFormat(0, 5, fmt.Sprintf("Dönem: son %d gün · Üretim: %s", d.Days, d.GeneratedAt.Format("02.01.2006 15:04")), "", 2, "L", false, 0, "")
	p.setRule()
	p.Ln(4)

	p.execSummary(d)
	p.dailySection(d)
	p.endpointsSection(d)
	p.processesSection(d)
	p.dnsSection(d)
	p.protocolSection(d)
	p.alertSection(d)

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (p *pdfRenderer) setRule() {
	p.SetDrawColor(accentR, accentG, accentB)
	p.SetLineWidth(0.8)
	y := p.GetY()
	p.Line(margin, y, pageW-margin, y)
	p.SetDrawColor(226, 232, 240)
	p.SetLineWidth(0.2)
	p.Ln(3)
}

func (p *pdfRenderer) section(title string) {
	p.Ln(3)
	p.ensureSpace(22)
	p.SetFont("go", "B", 12.5)
	p.SetTextColor(accentR, accentG, accentB)
	p.CellFormat(0, 7, title, "", 2, "L", false, 0, "")
	p.SetDrawColor(226, 232, 240)
	y := p.GetY()
	p.Line(margin, y, pageW-margin, y)
	p.Ln(2.5)
	p.SetTextColor(30, 41, 59)
}

// ensureSpace, tablonun yarilmasini engellemek icin yeterli alan yoksa yeni sayfa acar.
func (p *pdfRenderer) ensureSpace(needMM float64) {
	_, pageH := p.GetPageSize()
	if p.GetY()+needMM > pageH-18 {
		p.AddPage()
	}
}

func (p *pdfRenderer) kvLine(k, v string) {
	p.SetFont("go", "", 10)
	p.SetTextColor(100, 116, 139)
	p.CellFormat(52, 6, k, "", 0, "L", false, 0, "")
	p.SetFont("go", "B", 10)
	p.SetTextColor(30, 41, 59)
	p.CellFormat(0, 6, v, "", 2, "L", false, 0, "")
}

func (p *pdfRenderer) execSummary(d *Data) {
	p.section("Yönetici Özeti")
	half := (pageW - 2*margin - 8) / 2
	y0 := p.GetY()
	p.kpi(half, "Toplam Transfer", fmt.Sprintf("%.2f GB", d.TotalGB))
	p.SetFont("go", "", 10)
	kpis := [][2]string{
		{"Ort. indirme", bits(d.AvgInBps)},
		{"Ort. gönderme", bits(d.AvgOutBps)},
		{"Zirve ↓ / ↑", bits(d.PeakInBps) + " / " + bits(d.PeakOutBps)},
		{"Örnek sayısı", fmt.Sprint(d.Samples)},
	}
	for i, kv := range kpis {
		p.SetXY(pageW-margin-half, y0+float64(i)*6.5)
		p.SetFont("go", "", 9)
		p.SetTextColor(100, 116, 139)
		p.CellFormat(28, 6, kv[0], "", 0, "L", false, 0, "")
		p.SetFont("go", "B", 9.5)
		p.SetTextColor(30, 41, 59)
		p.CellFormat(0, 6, kv[1], "", 2, "L", false, 0, "")
	}
	p.SetY(y0 + 27)

	p.Ln(1)
	if len(d.TopEndpoints) > 0 {
		e := d.TopEndpoints[0]
		name := e.IP
		if e.Hostname != "" {
			name = fmt.Sprintf("%s (%s)", e.Hostname, e.IP)
		}
		p.kvLine("En yoğun hedef", fmt.Sprintf("%s — %s ↓ / %s ↑", name, bytesFmt(e.BytesIn), bytesFmt(e.BytesOut)))
	}
	if len(d.TopProcesses) > 0 {
		p.kvLine("En aktif süreç", fmt.Sprintf("%s (%d bağlantı)", d.TopProcesses[0].Process, d.TopProcesses[0].Connections))
	}
	if len(d.TopDomains) > 0 {
		p.kvLine("En çok sorgulanan domain", fmt.Sprintf("%s (%d sorgu)", d.TopDomains[0].Domain, d.TopDomains[0].Queries))
	}
	alertText := fmt.Sprint(len(d.Alerts))
	if len(d.AlertCounts) > 0 {
		alertText += " ("
		first := true
		for k, v := range d.AlertCounts {
			if !first {
				alertText += ", "
			}
			alertText += fmt.Sprintf("%s: %d", k, v)
			first = false
		}
		alertText += ")"
	}
	p.kvLine("Uyarı olayları", alertText)
	p.Ln(2)
}

func (p *pdfRenderer) kpi(w float64, label, value string) {
	p.SetFont("go", "", 8)
	p.SetTextColor(100, 116, 139)
	p.CellFormat(w, 4.5, label, "", 2, "L", false, 0, "")
	p.SetFont("go", "B", 14)
	p.SetTextColor(30, 41, 59)
	p.CellFormat(w, 8, value, "", 2, "L", false, 0, "")
	p.SetX(margin)
}

// table baslik satiri + kolon genislikleriyle tekrarlanan tablo yardimcisi.
func (p *pdfRenderer) tableHeader(cols []float64, titles []string) {
	p.SetFont("go", "B", 8.5)
	p.SetFillColor(241, 245, 249)
	p.SetTextColor(100, 116, 139)
	for i, t := range titles {
		align := "L"
		if i > 1 {
			align = "R"
		}
		p.CellFormat(cols[i], 6.5, t, "B", 0, align, true, 0, "")
	}
	p.Ln(-1)
	p.SetTextColor(30, 41, 59)
}

func (p *pdfRenderer) tableRow(cols []float64, cells []string, fill bool) {
	p.SetFont("go", "", 8.8)
	for i, c := range cells {
		align := "L"
		if i > 1 {
			align = "R"
		}
		p.CellFormat(cols[i], 5.6, c, "", 0, align, fill, 0, "")
	}
	p.Ln(-1)
}

func (p *pdfRenderer) dailySection(d *Data) {
	if len(d.Daily) == 0 {
		return
	}
	p.section("Günlük Trafik")

	// 7 gunluk bar grafigi
	maxGB := 0.0
	gbs := make([]float64, len(d.Daily))
	for i, day := range d.Daily {
		gbs[i] = dailyGB(day)
		if gbs[i] > maxGB {
			maxGB = gbs[i]
		}
	}
	p.ensureSpace(45)
	chartH, chartW := 34.0, pageW-2*margin
	baseY := p.GetY()
	barW := chartW / float64(len(d.Daily)) * 0.55
	for i, g := range gbs {
		h := 0.0
		if maxGB > 0 {
			h = g / maxGB * chartH
		}
		x := margin + chartW/float64(len(d.Daily))*float64(i) + (chartW/float64(len(d.Daily))-barW)/2
		p.SetFillColor(8, 145, 178)
		p.RoundedRect(x, baseY+chartH-h, barW, h, 0.8, "1234", "F")
		p.SetFont("go", "", 6.5)
		p.SetTextColor(100, 116, 139)
		p.SetXY(x-2, baseY+chartH+1)
		p.CellFormat(barW+4, 4, time.Unix(d.Daily[i].Day, 0).Format("02.01"), "", 0, "C", false, 0, "")
	}
	p.SetY(baseY + chartH + 6)

	cols := []float64{26, 26, 26, 26, 26, 30, 26}
	p.tableHeader(cols, []string{"Gün", "Ort. ↓", "Ort. ↑", "Zirve ↓", "Zirve ↑", "Toplam", "Örnek"})
	for i, day := range d.Daily {
		p.ensureSpace(8)
		p.tableRow(cols, []string{
			time.Unix(day.Day, 0).Format("02.01.2006"),
			bits(day.AvgBpsIn), bits(day.AvgBpsOut),
			bits(day.PeakBpsIn), bits(day.PeakBpsOut),
			fmt.Sprintf("%.2f GB", gbs[i]),
			fmt.Sprint(day.Samples),
		}, i%2 == 1)
	}
}

func (p *pdfRenderer) endpointsSection(d *Data) {
	if len(d.TopEndpoints) == 0 {
		return
	}
	p.section("En Yoğun Hedefler")
	p.ensureSpace(20)
	cols := []float64{52, 22, 30, 24, 24, 18}
	p.tableHeader(cols, []string{"Hedef", "Ülke", "ASN", "İndirme", "Gönderme", "Paket"})
	for i, e := range d.TopEndpoints {
		p.ensureSpace(8)
		name := e.IP
		if e.Hostname != "" {
			name = trunc(e.Hostname, 46) + " (" + e.IP + ")"
		}
		asn := trunc(e.ASN, 20)
		if asn == "" {
			asn = "—"
		}
		if e.Country == "" {
			e.Country = "—"
		}
		p.tableRow(cols, []string{trunc(name, 52), e.Country, asn, bytesFmt(e.BytesIn), bytesFmt(e.BytesOut), fmt.Sprint(e.Packets)}, i%2 == 1)
	}
}

func (p *pdfRenderer) processesSection(d *Data) {
	if len(d.TopProcesses) == 0 {
		return
	}
	p.section("En Aktif Süreçler")
	p.ensureSpace(20)
	cols := []float64{90, 40, 40}
	p.tableHeader(cols, []string{"Süreç", "Bağlantı", "Olay"})
	for i, pr := range d.TopProcesses {
		p.ensureSpace(8)
		p.tableRow(cols, []string{pr.Process, fmt.Sprint(pr.Connections), fmt.Sprint(pr.Events)}, i%2 == 1)
	}
}

func (p *pdfRenderer) dnsSection(d *Data) {
	if len(d.TopDomains) == 0 {
		return
	}
	p.section("DNS Sorguları")
	p.ensureSpace(20)
	cols := []float64{90, 35, 35}
	p.tableHeader(cols, []string{"Domain", "Sorgu", "Yanıt"})
	for i, dom := range d.TopDomains {
		p.ensureSpace(8)
		p.tableRow(cols, []string{trunc(dom.Domain, 48), fmt.Sprint(dom.Queries), fmt.Sprint(dom.Responses)}, i%2 == 1)
	}
}

func (p *pdfRenderer) protocolSection(d *Data) {
	if len(d.Protocols) == 0 {
		return
	}
	p.section("Protokol Dağılımı")
	p.ensureSpace(20)
	cols := []float64{60, 45, 65}
	p.tableHeader(cols, []string{"Protokol", "Paket", "Pay"})
	total := uint64(0)
	for _, pr := range d.Protocols {
		total += pr.Count
	}
	for i, pr := range d.Protocols {
		p.ensureSpace(8)
		pct := 0.0
		if total > 0 {
			pct = float64(pr.Count) / float64(total) * 100
		}
		p.tableRow(cols, []string{pr.Name, fmt.Sprint(pr.Count), fmt.Sprintf("%%%.1f", pct)}, i%2 == 1)
	}
}

func (p *pdfRenderer) alertSection(d *Data) {
	p.section("Uyarı Olayları")
	if len(d.Alerts) == 0 {
		p.SetFont("go", "", 9)
		p.SetTextColor(100, 116, 139)
		p.CellFormat(0, 5, "Bu dönemde uyarı olayı oluşmadı.", "", 2, "L", false, 0, "")
		return
	}
	p.ensureSpace(20)
	cols := []float64{22, 26, 122}
	p.tableHeader(cols, []string{"Zaman", "Tür", "Olay"})
	for i, a := range d.Alerts {
		p.ensureSpace(8)
		p.tableRow(cols, []string{time.Unix(a.Ts, 0).Format("02.01 15:04"), a.Key, trunc(a.Message, 80)}, i%2 == 1)
	}
}

// --- yardimcilar ---

func bits(v float64) string {
	units := []string{"bit/s", "Kbit/s", "Mbit/s", "Gbit/s"}
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func bytesFmt(v uint64) string {
	f := float64(v)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// trunc, metni gosterilen genislige gore kisa keser (hücre tasmasini onler).
func trunc(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}
