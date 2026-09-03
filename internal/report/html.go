package report

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

const htmlTpl = `<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<title>bazNTMS — Ağ Trafik Raporu — {{.GeneratedAt.Format "02.01.2006 15:04"}}</title>
<style>
  :root { --ink:#1e293b; --muted:#64748b; --line:#e2e8f0; --accent:#0e7490; --soft:#f1f5f9; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, "Segoe UI", Roboto, sans-serif; color: var(--ink);
         margin: 0; background: #f8fafc; }
  .page { max-width: 860px; margin: 24px auto; background: #fff; padding: 40px 48px;
          border: 1px solid var(--line); }
  header { border-bottom: 3px solid var(--accent); padding-bottom: 14px; margin-bottom: 24px; }
  h1 { margin: 0; font-size: 22px; letter-spacing: .5px; }
  .sub { color: var(--muted); font-size: 12px; margin-top: 4px; }
  h2 { font-size: 14px; text-transform: uppercase; letter-spacing: 1px; color: var(--accent);
       border-bottom: 1px solid var(--line); padding-bottom: 6px; margin: 28px 0 10px; }
  table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
  th { text-align: left; background: var(--soft); padding: 6px 8px; border-bottom: 2px solid var(--line);
       font-size: 11px; text-transform: uppercase; letter-spacing: .5px; color: var(--muted); }
  td { padding: 5px 8px; border-bottom: 1px solid var(--line); }
  tr:nth-child(even) td { background: #fcfdfe; }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
  .summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin: 12px 0; }
  .kpi { border: 1px solid var(--line); padding: 10px 12px; }
  .kpi b { display: block; font-size: 17px; margin-top: 2px; }
  .kpi span { font-size: 10.5px; color: var(--muted); text-transform: uppercase; letter-spacing: .5px; }
  .bar { height: 12px; background: #0891b2; min-width: 2px; display: inline-block; vertical-align: middle; }
  .bar.up { background: #8b5cf6; }
  .barwrap { background: var(--soft); display: inline-block; vertical-align: middle; }
  .mut { color: var(--muted); font-size: 11px; }
  .pct { font-variant-numeric: tabular-nums; }
  footer { margin-top: 30px; border-top: 1px solid var(--line); padding-top: 10px;
           color: var(--muted); font-size: 10.5px; display: flex; justify-content: space-between; }
  ul.klist { margin: 6px 0; padding-left: 18px; font-size: 12.5px; }
  @media print {
    body { background: #fff; }
    .page { border: none; margin: 0; padding: 10mm 12mm; max-width: none; }
    h2 { break-after: avoid; }
    table, .kpi { break-inside: avoid; }
  }
</style>
</head>
<body>
<div class="page">
  <header>
    <h1>bazNTMS — Ağ Trafik Raporu</h1>
    <div class="sub">Dönem: son {{.Days}} gün &middot; Üretim: {{.GeneratedAt.Format "02.01.2006 15:04"}} &middot; kaynak: agent filosu + NetFlow</div>
  </header>

  {{if .Empty}}<p class="mut" style="border:1px solid var(--line); background:#fffbeb; padding:10px 12px">
    Bu dönemde filo trafiği kaydı bulunamadı. Agent'ların telemetri gönderdiğini
    (Agentlar sayfası) ve/veya bir NetFlow exporter tanımlı olduğunu doğrulayın.
  </p>{{end}}

  <h2>Yönetici Özeti</h2>
  <div class="summary">
    <div class="kpi"><span>Toplam Transfer</span><b>{{printf "%.2f" .TotalGB}} GB</b></div>
    <div class="kpi"><span>Ort. İndirme</span><b>{{bits .AvgInBps}}</b></div>
    <div class="kpi"><span>Ort. Gönderme</span><b>{{bits .AvgOutBps}}</b></div>
    <div class="kpi"><span>Zirve (↓ / ↑)</span><b>{{bits .PeakInBps}} / {{bits .PeakOutBps}}</b></div>
  </div>
  <ul class="klist">
    <li>Kaynak: {{.Sources}} agent telemetri gönderiyor</li>
    <li>Uyarı olayları: {{len .Alerts}}{{if .AlertCounts}} — {{range $k, $v := .AlertCounts}}{{$k}}: {{$v}} {{end}}{{end}}</li>
    {{if .TopEndpoints}}{{with index .TopEndpoints 0}}<li>En yoğun hedef: {{.IP}} — {{bytes .BytesIn}} ↓ / {{bytes .BytesOut}} ↑</li>{{end}}{{end}}
    {{if .TopProcesses}}{{with index .TopProcesses 0}}<li>En yoğun süreç: {{.Process}} ({{bytes .Total}} toplam)</li>{{end}}{{end}}
  </ul>

  <h2>Günlük Trafik</h2>
  <table>
    <tr><th>Gün</th><th class="num">Ort. ↓</th><th class="num">Ort. ↑</th><th class="num">Zirve ↓</th><th class="num">Toplam</th><th>Göreli</th></tr>
    {{$max := .MaxDailyGB}}
    {{range .Daily}}
    <tr>
      <td>{{dayLabel .Day}}</td>
      <td class="num">{{bits .AvgBpsIn}}</td>
      <td class="num">{{bits .AvgBpsOut}}</td>
      <td class="num">{{bits .PeakBpsIn}}</td>
      <td class="num pct">{{printf "%.2f" (dailyGB .)}} GB</td>
      <td class="barwrap" style="width:200px"><span class="bar" style="width:{{printf "%.0f" (barPct (dailyGB .) $max)}}%"></span><span class="bar up" style="width:{{printf "%.0f" (barPct (dailyUpGB .) $max)}}%"></span></td>
    </tr>
    {{end}}
  </table>

  <h2>Agent Filosu</h2>
  {{if .Agents}}
  <table>
    <tr><th>Agent</th><th>Site</th><th class="num">Anlık ↓</th><th class="num">Anlık ↑</th><th class="num">Bağlantı</th><th>Son Görülme</th></tr>
    {{range .Agents}}
    <tr>
      <td>{{.Name}}{{if not .Online}} <span class="mut">(çevrimdışı)</span>{{end}}</td>
      <td>{{if .Site}}{{.Site}}{{else}}<span class="mut">—</span>{{end}}</td>
      <td class="num">{{bits (agentRx .Rates)}}</td>
      <td class="num">{{bits (agentTx .Rates)}}</td>
      <td class="num">{{.Conns}}</td>
      <td class="mut">{{ts .LastSeen}}</td>
    </tr>
    {{end}}
  </table>
  {{else}}<p class="mut">Kayıtlı agent yok.</p>{{end}}

  <h2>En Yoğun Hedefler</h2>
  <p class="mut">NetFlow kayıtlarından (yoksa agent süreç trafiğinden) uzak uç nokta bazında toplam hacim.</p>
  <table>
    <tr><th>#</th><th>Uç Nokta</th><th>Ülke / ASN</th><th class="num">Gelen</th><th class="num">Giden</th><th class="num">Paket</th></tr>
    {{range $i, $e := .TopEndpoints}}
    <tr>
      <td>{{$i}}</td>
      <td>{{.IP}}</td>
      <td>{{if .Country}}{{.Country}}{{end}}{{if .ASN}} <span class="mut">{{.ASN}}</span>{{end}}{{if not (or .Country .ASN)}}<span class="mut">—</span>{{end}}</td>
      <td class="num">{{bytes .BytesIn}}</td>
      <td class="num">{{bytes .BytesOut}}</td>
      <td class="num">{{.Packets}}</td>
    </tr>
    {{end}}
  </table>

  <h2>Süreç Bazlı Trafik</h2>
  <table>
    <tr><th>#</th><th>Süreç</th><th class="num">İndirilen</th><th class="num">Gönderilen</th><th class="num">Agent</th></tr>
    {{range $i, $p := .TopProcesses}}
    <tr><td>{{$i}}</td><td>{{.Process}}</td><td class="num">{{bytes .BytesIn}}</td><td class="num">{{bytes .BytesOut}}</td><td class="num">{{.AgentCnt}}</td></tr>
    {{end}}
  </table>

  {{if .TopDomains}}
  <h2>DNS Görünürlüğü</h2>
  <p class="mut">Agent'ların gözlemlediği (UDP/53) en çok sorulan alan adları, süreç ile.</p>
  <table>
    <tr><th>#</th><th>Alan Adı</th><th>Süreç</th><th class="num">Sorgu</th><th class="num">Yanıt</th><th class="num">Agent</th></tr>
    {{range $i, $x := .TopDomains}}
    <tr><td>{{$i}}</td><td>{{.Domain}}</td><td>{{.Process}}</td><td class="num">{{.Queries}}</td><td class="num">{{.Responses}}</td><td class="num">{{.AgentCnt}}</td></tr>
    {{end}}
  </table>
  {{end}}

  <h2>Protokol Dağılımı</h2>
  <p class="mut">NetFlow octet toplamı (proto alanına göre).</p>
  <table>
    <tr><th>Protokol</th><th class="num">Trafik</th><th>Göreli</th></tr>
    {{$pt := .ProtoTotal}}
    {{range .Protocols}}
    <tr>
      <td>{{.Name}}</td>
      <td class="num">{{bytes .Count}}</td>
      <td class="barwrap" style="width:260px"><span class="bar" style="width:{{printf "%.0f" (barPct (toFloat .Count) (toFloat $pt))}}%"></span></td>
    </tr>
    {{end}}
  </table>

  <h2>Uyarı Olayları</h2>
  {{if .Alerts}}
  <table>
    <tr><th>Zaman</th><th>Tür</th><th>Olay</th></tr>
    {{range .Alerts}}
    <tr><td class="pct">{{ts .Ts}}</td><td>{{.Key}}</td><td>{{.Message}}</td></tr>
    {{end}}
  </table>
  {{else}}<p class="mut">Bu dönemde uyarı olayı oluşmadı.</p>{{end}}

  <footer>
    <span>bazNTMS · otomatik üretilmiş rapor</span>
    <span>sayfa: yazdırırken tarayıcı başlığını kullanın</span>
  </footer>
</div>
</body>
</html>`

// RenderHTML, raporu bagimsiz bir HTML belgesi olarak uretir.
func (d *Data) RenderHTML() ([]byte, error) {
	funcs := template.FuncMap{
		"bits": func(v float64) string {
			units := []string{"bit/s", "Kbit/s", "Mbit/s", "Gbit/s"}
			i := 0
			for v >= 1000 && i < len(units)-1 {
				v /= 1000
				i++
			}
			return fmt.Sprintf("%.1f %s", v, units[i])
		},
		"bytes": func(v uint64) string {
			f := float64(v)
			units := []string{"B", "KB", "MB", "GB", "TB"}
			i := 0
			for f >= 1024 && i < len(units)-1 {
				f /= 1024
				i++
			}
			return fmt.Sprintf("%.1f %s", f, units[i])
		},
		"ts": func(unix int64) string {
			return time.Unix(unix, 0).Format("02.01 15:04")
		},
		"dayLabel": func(unix int64) string {
			return time.Unix(unix, 0).Format("02.01.2006")
		},
		"toFloat":   func(v uint64) float64 { return float64(v) },
		"dailyGB":   dailyGB,
		"dailyUpGB": dailyUpGB,
		"agentRx":   func(r []store.AgentRate) float64 { return agentRateSum(r, true) },
		"agentTx":   func(r []store.AgentRate) float64 { return agentRateSum(r, false) },
		"barPct": func(v, max float64) float64 {
			if max <= 0 {
				return 0
			}
			return v / max * 100
		},
	}
	t, err := template.New("report").Funcs(funcs).Parse(htmlTpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d.templateData()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func dailyGB(dt store.DayTotal) float64 {
	return ((dt.AvgBpsIn + dt.AvgBpsOut) / 8) * float64(dt.Samples) / 1e9
}

func dailyUpGB(dt store.DayTotal) float64 {
	return (dt.AvgBpsOut / 8) * float64(dt.Samples) / 1e9
}

// agentRateSum, bir agent'in tum arayuzlerinin anlik hizini toplar (bit/sn).
// AgentRate.RxBps/TxBps bayt/sn'dir → ×8.
func agentRateSum(rates []store.AgentRate, rx bool) float64 {
	var sum float64
	for _, r := range rates {
		if rx {
			sum += r.RxBps
		} else {
			sum += r.TxBps
		}
	}
	return sum * 8
}

// templateData, sablon icindeki turetilmis alanlari hesaplar.
func (d *Data) templateData() map[string]any {
	maxGB := 0.0
	for _, day := range d.Daily {
		if g := dailyGB(day); g > maxGB {
			maxGB = g
		}
	}
	protoTotal := uint64(0)
	for _, p := range d.Protocols {
		protoTotal += p.Count
	}
	return map[string]any{
		"GeneratedAt":  d.GeneratedAt,
		"Days":         d.Days,
		"Sources":      d.Sources,
		"Empty":        d.Empty,
		"TotalGB":      d.TotalGB,
		"AvgInBps":     d.AvgInBps,
		"AvgOutBps":    d.AvgOutBps,
		"PeakInBps":    d.PeakInBps,
		"PeakOutBps":   d.PeakOutBps,
		"Daily":        d.Daily,
		"Agents":       d.Agents,
		"TopEndpoints": d.TopEndpoints,
		"TopProcesses": d.TopProcesses,
		"TopDomains":   d.TopDomains,
		"Protocols":    d.Protocols,
		"Alerts":       d.Alerts,
		"AlertCounts":  d.AlertCounts,
		"MaxDailyGB":   maxGB,
		"ProtoTotal":   protoTotal,
	}
}
