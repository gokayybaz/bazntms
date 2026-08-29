package report

// Kurumsal raporlar (Faz 6.4): SLA / kapasite / banding. Mevcut trafik
// raporunun cok kaynakli sürümü — agent filosu, cihaz envanteri ve
// Timescale continuous aggregate'larindan beslenir. HTML olarak üretilir.

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

type EnterpriseData struct {
	GeneratedAt time.Time `json:"generated_at"`
	Days        int       `json:"days"`

	// SLA
	AgentTotal   int     `json:"agent_total"`
	AgentOnline  int     `json:"agent_online"`
	AgentUptime  float64 `json:"agent_uptime_pct"`
	DeviceTotal  int     `json:"device_total"`
	DeviceOK     int     `json:"device_ok"`
	DeviceHealth float64 `json:"device_health_pct"`
	DropPct      float64 `json:"drop_pct"`

	// kapasite / banding
	TotalGB   float64 `json:"total_gb"`
	AvgBps    float64 `json:"avg_bps"`
	PeakBps   float64 `json:"peak_bps"`
	P50Bps    float64 `json:"p50_bps"`
	P95Bps    float64 `json:"p95_bps"`
	P99Bps    float64 `json:"p99_bps"`
	PrevGB    float64 `json:"prev_gb"`
	GrowthPct float64 `json:"growth_pct"`

	TopEndpoints []store.EndpointDelta `json:"top_endpoints"`
	TopProcesses []store.ProcessUsage  `json:"top_processes"`
	AlertCounts  map[string]int        `json:"alert_counts"`
}

// BuildEnterprise, son `days` gun icin SLA/kapasite/banding modelini kurar.
func BuildEnterprise(st store.Store, days int) (*EnterpriseData, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	d := &EnterpriseData{
		GeneratedAt: time.Now(),
		Days:        days,
		AlertCounts: map[string]int{},
	}

	// SLA: agent online orani
	agents, err := st.ListAgents(2*time.Minute, "")
	if err != nil {
		return nil, fmt.Errorf("agent filosu: %w", err)
	}
	d.AgentTotal = len(agents)
	for _, a := range agents {
		if a.Online {
			d.AgentOnline++
		}
	}
	if d.AgentTotal > 0 {
		d.AgentUptime = 100 * float64(d.AgentOnline) / float64(d.AgentTotal)
	}

	// SLA: cihaz poll basarisi (son hatasi olmayan + guncel poll)
	devices, err := st.ListDevices()
	if err != nil {
		return nil, fmt.Errorf("cihazlar: %w", err)
	}
	d.DeviceTotal = len(devices)
	now := time.Now().Unix()
	for _, dev := range devices {
		fresh := dev.LastPoll > 0 && now-dev.LastPoll < int64(3*dev.PollSeconds)
		if dev.LastError == "" && (fresh || !dev.Enabled) && dev.Enabled {
			d.DeviceOK++
		}
	}
	if d.DeviceTotal > 0 {
		d.DeviceHealth = 100 * float64(d.DeviceOK) / float64(d.DeviceTotal)
	}

	// SLA: paket dusme orani
	if dropped, pps, err := st.DropStats(since); err == nil && pps > 0 {
		d.DropPct = 100 * float64(dropped) / float64(pps)
	}

	// kapasite: donem toplamlari + onceki donem karsilastirma
	totals, err := st.PeriodTotals(since)
	if err != nil {
		return nil, fmt.Errorf("donem ozeti: %w", err)
	}
	d.TotalGB = (totals.AvgBpsIn + totals.AvgBpsOut) * float64(totals.Seconds) / 8 / 1e9
	d.AvgBps = totals.AvgBpsIn + totals.AvgBpsOut
	d.PeakBps = totals.PeakBpsIn + totals.PeakBpsOut

	prevTotals, _ := st.PeriodTotals(since.Add(-time.Duration(days) * 24 * time.Hour))
	d.PrevGB = (prevTotals.AvgBpsIn + prevTotals.AvgBpsOut) * float64(prevTotals.Seconds) / 8 / 1e9
	if d.PrevGB > 0 {
		d.GrowthPct = 100 * (d.TotalGB - d.PrevGB) / d.PrevGB
	}

	// banding: 60 sn kovar uzerinden yuzdelikler (Timescale'de cagg uzerinden)
	buckets, err := st.TimeseriesBuckets(since)
	if err != nil {
		return nil, fmt.Errorf("banding kovalari: %w", err)
	}
	pcts := percentiles(buckets, 0.50, 0.95, 0.99)
	d.P50Bps, d.P95Bps, d.P99Bps = pcts[0], pcts[1], pcts[2]

	// top listeler
	if d.TopEndpoints, err = st.TopEndpointsSince(since, 10); err != nil {
		return nil, fmt.Errorf("hedefler: %w", err)
	}
	if d.TopProcesses, err = st.TopProcessesSince(since, 10); err != nil {
		return nil, fmt.Errorf("surecler: %w", err)
	}
	alerts, err := st.RecentAlertEvents(500)
	if err == nil {
		cutoff := since.Unix()
		for _, a := range alerts {
			if a.Ts >= cutoff {
				d.AlertCounts[a.Kind]++
			}
		}
	}
	return d, nil
}

// percentiles, kova toplamlarindan (bps_in + bps_out) verilen yuzdelikleri dondurur.
func percentiles(buckets []store.Bucket, ps ...float64) []float64 {
	if len(buckets) == 0 {
		return make([]float64, len(ps))
	}
	vals := make([]float64, 0, len(buckets))
	for _, b := range buckets {
		vals = append(vals, (b.In+b.Out)*8) // /8 donusumunu geri al -> bps
	}
	sort.Float64s(vals)
	out := make([]float64, len(ps))
	for i, p := range ps {
		idx := int(p * float64(len(vals)-1))
		out[i] = vals[idx]
	}
	return out
}

const enterpriseTpl = `<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<title>bazNTMS — Kurumsal Rapor (SLA/Kapasite) — {{.GeneratedAt.Format "02.01.2006 15:04"}}</title>
<style>
  :root { --ink:#1e293b; --muted:#64748b; --line:#e2e8f0; --accent:#0e7490; --soft:#f1f5f9; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, "Segoe UI", Roboto, sans-serif; color: var(--ink); margin: 0; background: #f8fafc; }
  .page { max-width: 860px; margin: 24px auto; background: #fff; padding: 40px 48px; border: 1px solid var(--line); }
  header { border-bottom: 3px solid var(--accent); padding-bottom: 14px; margin-bottom: 24px; }
  h1 { margin: 0; font-size: 22px; letter-spacing: .5px; }
  .sub { color: var(--muted); font-size: 12px; margin-top: 4px; }
  h2 { font-size: 14px; text-transform: uppercase; letter-spacing: 1px; color: var(--accent);
       border-bottom: 1px solid var(--line); padding-bottom: 6px; margin: 28px 0 10px; }
  .kpi { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin: 12px 0; }
  .kpi div { border: 1px solid var(--line); padding: 10px 12px; }
  .kpi b { display: block; font-size: 17px; margin-top: 2px; }
  .kpi span { font-size: 10.5px; color: var(--muted); text-transform: uppercase; letter-spacing: .5px; }
  table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
  th { text-align: left; background: var(--soft); padding: 6px 8px; border-bottom: 2px solid var(--line);
       font-size: 11px; text-transform: uppercase; letter-spacing: .5px; color: var(--muted); }
  td { padding: 5px 8px; border-bottom: 1px solid var(--line); }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
  footer { margin-top: 30px; color: var(--muted); font-size: 11px; text-align: center; }
</style>
</head>
<body>
<div class="page">
  <header>
    <h1>bazNTMS — Kurumsal Rapor</h1>
    <div class="sub">SLA · kapasite · banding — son {{.Days}} gün · üretilme {{.GeneratedAt.Format "02.01.2006 15:04"}}</div>
  </header>

  <h2>SLA</h2>
  <div class="kpi">
    <div><span>Agent Uptime</span><b>{{printf "%.1f" .AgentUptime}}% ({{.AgentOnline}}/{{.AgentTotal}})</b></div>
    <div><span>Cihaz Sağlığı</span><b>{{printf "%.1f" .DeviceHealth}}% ({{.DeviceOK}}/{{.DeviceTotal}})</b></div>
    <div><span>Paket Düşme</span><b>{{printf "%.3f" .DropPct}}%</b></div>
    <div><span>Uyarı Sayısı</span><b>{{totalAlerts .AlertCounts}}</b></div>
  </div>

  <h2>Kapasite ve Banding</h2>
  <div class="kpi">
    <div><span>Toplam Trafik</span><b>{{printf "%.1f" .TotalGB}} GB</b></div>
    <div><span>Büyüme (önceki dönem)</span><b>{{printf "%+.1f" .GrowthPct}}%</b></div>
    <div><span>Ortalama Verim</span><b>{{printf "%.1f" .AvgBps}} bps</b></div>
    <div><span>Zirve Verim</span><b>{{printf "%.1f" .PeakBps}} bps</b></div>
  </div>
  <table>
    <tr><th>Bant</th><th class="num">Toplam Verim (bps)</th><th class="num">Kapasite Planı Notu</th></tr>
    <tr><td>p50 (tipik)</td><td class="num">{{printf "%.0f" .P50Bps}}</td><td>günlük operasyon profili</td></tr>
    <tr><td>p95 (zirve profili)</td><td class="num">{{printf "%.0f" .P95Bps}}</td><td>kalıcı kapasite bu bandı karşılamalı</td></tr>
    <tr><td>p99 (sıçrama)</td><td class="num">{{printf "%.0f" .P99Bps}}</td><td>yedeklilik/burst planlaması</td></tr>
  </table>

  <h2>En Yoğun Uç Noktalar</h2>
  <table>
    <tr><th>IP</th><th>Hostname</th><th class="num">Gelen (MB)</th><th class="num">Giden (MB)</th></tr>
    {{$missing := true}}
    {{range .TopEndpoints}}{{$missing = false}}
    <tr><td>{{.IP}}</td><td>{{.Hostname}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesIn 1048576)}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesOut 1048576)}}</td></tr>
    {{end}}
    {{if $missing}}<tr><td colspan="4">kayıt yok</td></tr>{{end}}
  </table>

  <h2>En Aktif Süreçler (Agentlar)</h2>
  <table>
    <tr><th>Süreç</th><th class="num">Bağlantı</th><th class="num">Olay</th></tr>
    {{$missing2 := true}}
    {{range .TopProcesses}}{{$missing2 = false}}
    <tr><td>{{.Process}}</td><td class="num">{{.Connections}}</td><td class="num">{{.Events}}</td></tr>
    {{end}}
    {{if $missing2}}<tr><td colspan="3">kayıt yok</td></tr>{{end}}
  </table>

  <footer>bazNTMS kurumsal rapor motoru — Faz 6.4</footer>
</div>
</body>
</html>`

// RenderEnterpriseHTML, kurumsal raporu HTML uretir (PDF destegi ileri faz).
func (d *EnterpriseData) RenderEnterpriseHTML() ([]byte, error) {
	funcs := template.FuncMap{
		"divf": func(a any, b float64) float64 {
			switch v := a.(type) {
			case uint64:
				return float64(v) / b
			case int64:
				return float64(v) / b
			case int:
				return float64(v) / b
			}
			return 0
		},
		"totalAlerts": func(m map[string]int) int {
			n := 0
			for _, v := range m {
				n += v
			}
			return n
		},
	}
	t, err := template.New("enterprise").Funcs(funcs).Parse(enterpriseTpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
