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

	"github.com/gokayybaz/bazntms/internal/health"

	"github.com/gokayybaz/bazntms/internal/store"
)

type EnterpriseData struct {
	GeneratedAt time.Time `json:"generated_at"`
	Days        int       `json:"days"`
	Site        string    `json:"site,omitempty"` // "" = filo geneli (S22.20)

	// SLA
	AgentTotal    int     `json:"agent_total"`
	AgentOnline   int     `json:"agent_online"`
	AgentUptime   float64 `json:"agent_uptime_pct"`
	DeviceTotal   int     `json:"device_total"`
	DeviceOK      int     `json:"device_ok"`
	DeviceHealth  float64 `json:"device_health_pct"`
	IfaceDiscards uint64  `json:"iface_discards"` // SNMP cihaz arayuz iskarta sayaci artisi (donem)
	IfaceErrors   uint64  `json:"iface_errors"`   // SNMP cihaz arayuz hata sayaci artisi (donem)

	// kapasite / banding
	TotalGB   float64 `json:"total_gb"`
	AvgBps    float64 `json:"avg_bps"`
	PeakBps   float64 `json:"peak_bps"`
	P50Bps    float64 `json:"p50_bps"`
	P95Bps    float64 `json:"p95_bps"`
	P99Bps    float64 `json:"p99_bps"`
	PrevGB    float64 `json:"prev_gb"`
	GrowthPct float64 `json:"growth_pct"`

	// DataWindowDays, filo ham tablolarinda gercekte veri bulunan pencere
	// (retention nedeniyle secilen `Days`'ten kucuk olabilir).
	DataWindowDays float64 `json:"data_window_days"`
	Empty          bool    `json:"empty"`

	// SLA hedefleri (S22.21) — Target.Set()=false ise hedef tanımlı değil
	Target       store.SLATarget `json:"sla_target"`
	UptimeBreach bool            `json:"uptime_breach"`
	DeviceBreach bool            `json:"device_breach"`
	IfaceBreach  bool            `json:"iface_breach"`

	// Ağ sağlık skoru (Faz 25-A) — deterministik, her kesinti açıklanabilir.
	HealthScore      int                `json:"health_score"`
	HealthDeductions []health.Deduction `json:"health_deductions,omitempty"`

	TopEndpoints     []store.EndpointDelta       `json:"top_endpoints"`
	TopProcesses     []store.ProcessTrafficUsage `json:"top_processes"`
	TopConversations []store.FlowConversation    `json:"top_conversations,omitempty"` // Faz 25-B (23-B)
	TopDNS           []store.AgentDNSUsage       `json:"top_dns,omitempty"`           // Faz 25-B — DNS görünürlüğü
	TopL7            []store.L7Usage             `json:"top_l7,omitempty"`            // Faz 25-B — uygulama görünürlüğü
	OpenIncidents    []store.Incident            `json:"open_incidents,omitempty"`    // Faz 25-B (24-B)
	Recommendations  []string                    `json:"recommendations,omitempty"`   // Faz 25-B — deterministik şablon
	AlertCounts      map[string]int              `json:"alert_counts"`

	// Sites, filo-geneli raporda (site=="") saha kırılımı (S22.22).
	Sites []SiteBreakdown `json:"sites,omitempty"`
}

type SiteBreakdown struct {
	Site        string  `json:"site"`
	AgentTotal  int     `json:"agent_total"`
	AgentOnline int     `json:"agent_online"`
	DeviceTotal int     `json:"device_total"`
	DeviceOK    int     `json:"device_ok"`
	UptimePct   float64 `json:"uptime_pct"`
	HealthPct   float64 `json:"health_pct"`
}

// BuildEnterprise, son `days` gun icin SLA/kapasite/banding modelini kurar.
// site boş değilse rapor o sahaya kırpılır (S22.20).
func BuildEnterprise(st store.Store, days int, site string) (*EnterpriseData, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	d := &EnterpriseData{
		GeneratedAt: time.Now(),
		Days:        days,
		Site:        site,
		AlertCounts: map[string]int{},
	}

	// SLA: agent online orani
	agents, err := st.ListAgents(2*time.Minute, site)
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
	devices, err := st.ListDevices(site)
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

	// SLA: SNMP cihaz arayuz iskarta/hata sayaclari (eski `samples.dropped`
	// karsiligi — coklu-hub'da hicbir hub paket yakalamadigi icin)
	if disc, errs, err := st.FleetIfaceHealth(since, site); err == nil {
		d.IfaceDiscards, d.IfaceErrors = disc, errs
	}

	// ağ sağlık skoru (Faz 25-A) — mevcut sinyaller + açık kritik uyarı + olay
	hi := health.Inputs{
		AgentsTotal: d.AgentTotal, AgentsOnline: d.AgentOnline,
		DevicesTotal: d.DeviceTotal, DevicesOnline: d.DeviceOK,
		IfaceErrors: d.IfaceErrors, IfaceDiscards: d.IfaceDiscards,
	}
	if evs, _, err := st.QueryAlertEvents(store.AlertEventFilter{Severity: "crit", State: "firing", Site: site, Limit: 200}); err == nil {
		hi.CritAlertsOpen = len(evs)
	}
	if incs, err := st.RecentOpenIncidents(time.Now().Add(-24 * time.Hour)); err == nil {
		for _, in := range incs {
			if site != "" && in.Site != site {
				continue
			}
			hi.OpenIncidents++
			if in.RiskScore > hi.MaxIncidentRisk {
				hi.MaxIncidentRisk = in.RiskScore
			}
		}
	}
	hs := health.Compute(hi)
	d.HealthScore = hs.Score
	d.HealthDeductions = hs.Deductions

	// kapasite/banding: filo trafik serisi (agent arayuz telemetrisi).
	// 60 sn kova: percentile'lar icin yeterli cozunurluk.
	buckets, err := st.FleetTrafficBuckets(since, 60, site)
	if err != nil {
		return nil, fmt.Errorf("filo trafik serisi: %w", err)
	}
	var sumBps, totalBytes float64
	var t0, t1 int64
	for i, b := range buckets {
		tot := (b.In + b.Out) * 8 // bit/sn
		sumBps += tot
		if tot > d.PeakBps {
			d.PeakBps = tot
		}
		totalBytes += (b.In + b.Out) * 60
		if i == 0 {
			t0 = b.Ts
		}
		t1 = b.Ts
	}
	if n := float64(len(buckets)); n > 0 {
		d.AvgBps = sumBps / n
		d.TotalGB = totalBytes / 1e9
		d.DataWindowDays = float64(t1-t0) / 86400
	}

	// onceki donem [since-period, since) — buyume karsilastirmasi
	prevAll, _ := st.FleetTrafficBuckets(since.Add(-time.Duration(days)*24*time.Hour), 60, site)
	sinceUnix := since.Unix()
	var prevBytes float64
	for _, b := range prevAll {
		if b.Ts < sinceUnix {
			prevBytes += (b.In + b.Out) * 60
		}
	}
	d.PrevGB = prevBytes / 1e9
	if d.PrevGB > 0 {
		d.GrowthPct = 100 * (d.TotalGB - d.PrevGB) / d.PrevGB
	}

	pcts := percentiles(buckets, 0.50, 0.95, 0.99)
	d.P50Bps, d.P95Bps, d.P99Bps = pcts[0], pcts[1], pcts[2]

	// top listeler
	if d.TopEndpoints, err = st.FleetTopEndpoints(since, 10, site); err != nil {
		return nil, fmt.Errorf("hedefler: %w", err)
	}
	if d.TopProcesses, err = st.TopProcessTraffic(since, 0, 10, site); err != nil {
		return nil, fmt.Errorf("surecler: %w", err)
	}
	// Faz 25-B: ek görünürlük bölümleri — hepsi best-effort (eksikse bölüm
	// zarif düşer, rapor 500 vermez).
	d.TopConversations, _ = st.FlowConversations(since, "pair", "octets", 10, site)
	d.TopDNS, _ = st.TopAgentDNS(since, 0, 10, site)
	d.TopL7, _ = st.TopL7(since, 0, 10, site)
	if incs, err := st.RecentOpenIncidents(since); err == nil {
		for _, in := range incs {
			if site == "" || in.Site == site {
				d.OpenIncidents = append(d.OpenIncidents, in)
			}
		}
	}

	d.Empty = len(buckets) == 0 && len(d.TopEndpoints) == 0 && d.AgentTotal == 0

	// saha kırılımı (S22.22) — yalnız filo-geneli raporda
	if site == "" {
		bySite := map[string]*SiteBreakdown{}
		get := func(s string) *SiteBreakdown {
			if bySite[s] == nil {
				bySite[s] = &SiteBreakdown{Site: s}
			}
			return bySite[s]
		}
		for _, a := range agents {
			if a.Site == "" {
				continue
			}
			sb := get(a.Site)
			sb.AgentTotal++
			if a.Online {
				sb.AgentOnline++
			}
		}
		for _, dev := range devices {
			if dev.Site == "" || !dev.Enabled {
				continue
			}
			sb := get(dev.Site)
			sb.DeviceTotal++
			if dev.LastError == "" && dev.LastPoll > 0 && now-dev.LastPoll < int64(3*dev.PollSeconds) {
				sb.DeviceOK++
			}
		}
		for _, sb := range bySite {
			if sb.AgentTotal > 0 {
				sb.UptimePct = 100 * float64(sb.AgentOnline) / float64(sb.AgentTotal)
			}
			if sb.DeviceTotal > 0 {
				sb.HealthPct = 100 * float64(sb.DeviceOK) / float64(sb.DeviceTotal)
			}
			d.Sites = append(d.Sites, *sb)
		}
		sort.Slice(d.Sites, func(i, j int) bool { return d.Sites[i].Site < d.Sites[j].Site })
	}

	// SLA hedefleri (S22.21): saha-özel varsa o, yoksa global
	if tgt, terr := st.SLATargetFor(site); terr == nil && tgt.Set() {
		d.Target = tgt
		d.UptimeBreach = tgt.AgentUptimePct > 0 && d.AgentTotal > 0 && d.AgentUptime < tgt.AgentUptimePct
		d.DeviceBreach = tgt.DeviceHealthPct > 0 && d.DeviceTotal > 0 && d.DeviceHealth < tgt.DeviceHealthPct
		d.IfaceBreach = tgt.IfaceErrCeiling > 0 && int64(d.IfaceDiscards+d.IfaceErrors) > tgt.IfaceErrCeiling
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

	d.Recommendations = recommend(d) // Faz 25-B — deterministik şablon, LLM yok
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
    <div class="sub">SLA · kapasite · banding — {{if .Site}}saha: {{.Site}} · {{end}}son {{.Days}} gün · üretilme {{.GeneratedAt.Format "02.01.2006 15:04"}}
      {{if gt .DataWindowDays 0.0}}· veri penceresi ~{{printf "%.1f" .DataWindowDays}} gün{{end}}</div>
  </header>

  {{if .Empty}}<p style="border:1px solid var(--line); background:#fffbeb; padding:10px 12px; font-size:12px; color:var(--muted)">
    Filoda trafik/telemetri kaydı yok — agent'lar ve/veya SNMP cihazları veri gönderiyor mu kontrol edin.
  </p>{{end}}

  <h2>Yönetici Özeti</h2>
  <div class="kpi">
    <div><span>Ağ Sağlığı</span><b style="color:{{if ge .HealthScore 85}}#15803d{{else if ge .HealthScore 60}}#b45309{{else}}#b91c1c{{end}}">{{.HealthScore}} / 100</b></div>
    <div><span>Erişilebilirlik</span><b>{{printf "%.1f" .AgentUptime}}% agent</b></div>
    <div><span>Açık Kritik Olay</span><b>{{len .OpenIncidents}}</b></div>
    <div><span>Uyarı (dönem)</span><b>{{totalAlerts .AlertCounts}}</b></div>
    <div><span>Kapasite Riski</span><b>{{if .IfaceBreach}}var{{else if gt .GrowthPct 25.0}}büyüme{{else}}—{{end}}</b></div>
    <div><span>En Yoğun Uç</span><b>{{if .TopEndpoints}}{{(index .TopEndpoints 0).IP}}{{else}}—{{end}}</b></div>
    <div><span>En Yoğun Süreç</span><b>{{if .TopProcesses}}{{(index .TopProcesses 0).Process}}{{else}}—{{end}}</b></div>
    <div><span>Dönem</span><b>{{.Days}} gün</b></div>
  </div>

  <h2>Ağ Sağlık Skoru</h2>
  <div class="kpi">
    <div><span>Skor</span><b style="color:{{if ge .HealthScore 85}}#15803d{{else if ge .HealthScore 60}}#b45309{{else}}#b91c1c{{end}}">{{.HealthScore}} / 100</b></div>
  </div>
  {{if .HealthDeductions}}<table>
    <tr><th>Kesinti</th><th class="num">Puan</th></tr>
    {{range .HealthDeductions}}<tr><td>{{.Reason}}</td><td class="num">−{{.Points}}</td></tr>{{end}}
  </table>{{else}}<p style="font-size:11px;color:#15803d">Tespit edilen sorun yok.</p>{{end}}
  <p style="font-size:10.5px;color:var(--muted)">Deterministik ağırlıklı — opak AI skoru değil; her kesinti gerekçelidir.</p>

  <h2>SLA</h2>
  <div class="kpi">
    <div><span>Agent Uptime</span><b>{{printf "%.1f" .AgentUptime}}% ({{.AgentOnline}}/{{.AgentTotal}})</b></div>
    <div><span>Cihaz Sağlığı</span><b>{{printf "%.1f" .DeviceHealth}}% ({{.DeviceOK}}/{{.DeviceTotal}})</b></div>
    <div><span>Arayüz İskarta / Hata</span><b>{{.IfaceDiscards}} / {{.IfaceErrors}}</b></div>
    <div><span>Uyarı Sayısı</span><b>{{totalAlerts .AlertCounts}}</b></div>
  </div>
  <p style="font-size:10.5px;color:var(--muted)">İskarta/hata: SNMP izlenen cihaz arayüzlerinin dönem içi ifIn/OutDiscards + ifIn/OutErrors sayaç artışı.</p>

  {{if .Target.Set}}
  <table style="margin-top:8px">
    <tr><th>SLA Hedefi</th><th class="num">Hedef</th><th class="num">Gerçek</th><th class="num">Durum</th></tr>
    {{if gt .Target.AgentUptimePct 0.0}}<tr><td>Agent uptime</td><td class="num">≥ {{printf "%.1f" .Target.AgentUptimePct}}%</td><td class="num">{{printf "%.1f" .AgentUptime}}%</td><td class="num" style="color:{{if .UptimeBreach}}#b91c1c{{else}}#15803d{{end}}">{{if .UptimeBreach}}İHLAL{{else}}karşılandı{{end}}</td></tr>{{end}}
    {{if gt .Target.DeviceHealthPct 0.0}}<tr><td>Cihaz sağlığı</td><td class="num">≥ {{printf "%.1f" .Target.DeviceHealthPct}}%</td><td class="num">{{printf "%.1f" .DeviceHealth}}%</td><td class="num" style="color:{{if .DeviceBreach}}#b91c1c{{else}}#15803d{{end}}">{{if .DeviceBreach}}İHLAL{{else}}karşılandı{{end}}</td></tr>{{end}}
    {{if gt .Target.IfaceErrCeiling 0}}<tr><td>Arayüz iskarta+hata (24s)</td><td class="num">≤ {{.Target.IfaceErrCeiling}}</td><td class="num">{{add64 .IfaceDiscards .IfaceErrors}}</td><td class="num" style="color:{{if .IfaceBreach}}#b91c1c{{else}}#15803d{{end}}">{{if .IfaceBreach}}İHLAL{{else}}karşılandı{{end}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Sites}}
  <h2>Saha Kırılımı</h2>
  <table>
    <tr><th>Saha</th><th class="num">Agent (online/toplam)</th><th class="num">Uptime</th><th class="num">Cihaz (ok/toplam)</th><th class="num">Sağlık</th></tr>
    {{range .Sites}}
    <tr><td>{{.Site}}</td>
        <td class="num">{{.AgentOnline}} / {{.AgentTotal}}</td>
        <td class="num">{{printf "%.1f" .UptimePct}}%</td>
        <td class="num">{{.DeviceOK}} / {{.DeviceTotal}}</td>
        <td class="num">{{printf "%.1f" .HealthPct}}%</td></tr>
    {{end}}
  </table>
  {{end}}

  <h2>Kapasite ve Banding</h2>
  <div class="kpi">
    <div><span>Toplam Trafik</span><b>{{printf "%.1f" .TotalGB}} GB</b></div>
    <div><span>Büyüme (önceki dönem)</span><b>{{if gt .PrevGB 0.0}}{{printf "%+.1f" .GrowthPct}}%{{else}}—{{end}}</b></div>
    <div><span>Ortalama Verim</span><b>{{bits .AvgBps}}</b></div>
    <div><span>Zirve Verim</span><b>{{bits .PeakBps}}</b></div>
  </div>
  <table>
    <tr><th>Bant</th><th class="num">Toplam Verim</th><th class="num">Kapasite Planı Notu</th></tr>
    <tr><td>p50 (tipik)</td><td class="num">{{bits .P50Bps}}</td><td>günlük operasyon profili</td></tr>
    <tr><td>p95 (zirve profili)</td><td class="num">{{bits .P95Bps}}</td><td>kalıcı kapasite bu bandı karşılamalı</td></tr>
    <tr><td>p99 (sıçrama)</td><td class="num">{{bits .P99Bps}}</td><td>yedeklilik/burst planlaması</td></tr>
  </table>
  <p style="font-size:10.5px;color:var(--muted)">Verim = tüm agent arayüzlerinin toplam gelen+giden hızı (60 sn kova).</p>

  <h2>En Yoğun Uç Noktalar</h2>
  <table>
    <tr><th>IP</th><th class="num">Gelen (MB)</th><th class="num">Giden (MB)</th></tr>
    {{$missing := true}}
    {{range .TopEndpoints}}{{$missing = false}}
    <tr><td>{{.IP}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesIn 1048576)}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesOut 1048576)}}</td></tr>
    {{end}}
    {{if $missing}}<tr><td colspan="3">kayıt yok</td></tr>{{end}}
  </table>

  <h2>Süreç Bazlı Trafik (Agentlar)</h2>
  <table>
    <tr><th>Süreç</th><th class="num">İndirilen (MB)</th><th class="num">Gönderilen (MB)</th><th class="num">Agent</th></tr>
    {{$missing2 := true}}
    {{range .TopProcesses}}{{$missing2 = false}}
    <tr><td>{{.Process}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesIn 1048576)}}</td>
        <td class="num">{{printf "%.1f" (divf .BytesOut 1048576)}}</td>
        <td class="num">{{.AgentCnt}}</td></tr>
    {{end}}
    {{if $missing2}}<tr><td colspan="4">kayıt yok</td></tr>{{end}}
  </table>

  <h2>Top Konuşmalar (NetFlow)</h2>
  <table>
    <tr><th>Uç A</th><th>Uç B</th><th class="num">Akış</th><th class="num">Paket</th><th class="num">Veri (MB)</th></tr>
    {{range .TopConversations}}
    <tr><td>{{.Src}}</td><td>{{.Dst}}</td>
        <td class="num">{{.Flows}}</td><td class="num">{{.Packets}}</td>
        <td class="num">{{printf "%.1f" (divf .Octets 1048576)}}</td></tr>
    {{else}}<tr><td colspan="5">NetFlow verisi yok</td></tr>{{end}}
  </table>

  <h2>DNS / Uygulama Görünürlüğü</h2>
  <table>
    <tr><th>Alan Adı (DNS)</th><th class="num">Sorgu</th><th class="num">Agent</th></tr>
    {{range .TopDNS}}<tr><td>{{.Domain}}</td><td class="num">{{.Queries}}</td><td class="num">{{.AgentCnt}}</td></tr>
    {{else}}<tr><td colspan="3">süreç-atıflı DNS verisi yok (pcap gerekir)</td></tr>{{end}}
  </table>
  <table style="margin-top:8px">
    <tr><th>Alan Adı (TLS SNI / HTTP Host)</th><th>Tür</th><th class="num">Gözlem</th></tr>
    {{range .TopL7}}<tr><td>{{.Host}}</td><td>{{.Kind}}</td><td class="num">{{.Hits}}</td></tr>
    {{else}}<tr><td colspan="3">L7 görünürlüğü yok</td></tr>{{end}}
  </table>

  <h2>Açık Olaylar (Incident)</h2>
  <table>
    <tr><th>#</th><th>Önem</th><th>Başlık</th><th class="num">Risk</th><th>Sebep</th></tr>
    {{range .OpenIncidents}}
    <tr><td>{{.ID}}</td><td>{{.Severity}}</td><td>{{.Title}}</td>
        <td class="num">{{.RiskScore}}</td><td style="font-size:11px">{{.CorrelationReason}}</td></tr>
    {{else}}<tr><td colspan="5">açık olay yok</td></tr>{{end}}
  </table>

  <h2>Öneriler</h2>
  <ol style="font-size:12px; padding-left:18px; margin:8px 0">
    {{range .Recommendations}}<li style="margin:4px 0">{{.}}</li>{{end}}
  </ol>
  <p style="font-size:10.5px;color:var(--muted)">Öneriler deterministik şablonlardır (eşik-tabanlı) — LLM kullanılmaz.</p>

  <footer>bazNTMS kurumsal rapor motoru · kaynak: agent filosu + NetFlow + SNMP + korelasyon</footer>
</div>
</body>
</html>`

// RenderEnterpriseHTML, kurumsal raporu HTML olarak uretir.
func (d *EnterpriseData) RenderEnterpriseHTML() ([]byte, error) {
	funcs := template.FuncMap{
		"add64": func(a, b uint64) uint64 { return a + b },
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
		"bits": bits, // pdf.go'daki paket-duzeyi yardimci (bit/sn formatlayici)
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
