package report

import (
	"fmt"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/store"
)

// Data, raporun tamami; ayni modelden hem HTML hem PDF uretilir.
type Data struct {
	GeneratedAt time.Time `json:"generated_at"`
	Days        int       `json:"days"`

	Samples    int64   `json:"samples"`
	TotalGB    float64 `json:"total_gb"`
	AvgInBps   float64 `json:"avg_in_bps"`
	AvgOutBps  float64 `json:"avg_out_bps"`
	PeakInBps  float64 `json:"peak_in_bps"`
	PeakOutBps float64 `json:"peak_out_bps"`

	Daily        []store.DayTotal      `json:"daily"`
	TopEndpoints []store.EndpointDelta `json:"top_endpoints"`
	TopProcesses []store.ProcessUsage  `json:"top_processes"`
	TopDomains   []store.DNSDelta      `json:"top_domains"`
	Protocols    []ProtoCount          `json:"protocols"`
	Alerts       []store.AlertEvent    `json:"alerts"`

	AlertCounts map[string]int `json:"alert_counts"`
}

type ProtoCount struct {
	Name  string `json:"name"`
	Count uint64 `json:"count"`
}

// Build, store'dan son `days` gunun verisini toplayip rapor modelini kurar.
func Build(st store.Store, geo *geoip.Resolver, days int) (*Data, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	d := &Data{
		GeneratedAt: time.Now(),
		Days:        days,
		AlertCounts: map[string]int{},
	}

	totals, err := st.PeriodTotals(since)
	if err != nil {
		return nil, fmt.Errorf("donem ozeti: %w", err)
	}
	d.Samples = totals.Samples
	d.AvgInBps = totals.AvgBpsIn
	d.AvgOutBps = totals.AvgBpsOut
	d.PeakInBps = totals.PeakBpsIn
	d.PeakOutBps = totals.PeakBpsOut
	d.TotalGB = (totals.AvgBpsIn + totals.AvgBpsOut) * float64(totals.Seconds) / 8 / 1e9

	if d.Daily, err = st.DailyTotals(days); err != nil {
		return nil, fmt.Errorf("gunluk ozet: %w", err)
	}
	if d.TopEndpoints, err = st.TopEndpointsSince(since, 15); err != nil {
		return nil, fmt.Errorf("hedefler: %w", err)
	}
	if geo != nil && geo.Enabled() {
		for i := range d.TopEndpoints {
			info := geo.Lookup(d.TopEndpoints[i].IP)
			d.TopEndpoints[i].Country = info.Country
			d.TopEndpoints[i].ASN = info.ASN
		}
	}
	if d.TopProcesses, err = st.TopProcessesSince(since, 10); err != nil {
		return nil, fmt.Errorf("surecler: %w", err)
	}
	if d.TopDomains, err = st.TopDomainsSince(since, 10); err != nil {
		return nil, fmt.Errorf("dns: %w", err)
	}

	protos, err := st.ProtocolTotals(since)
	if err != nil {
		return nil, fmt.Errorf("protokoller: %w", err)
	}
	for name, count := range protos {
		d.Protocols = append(d.Protocols, ProtoCount{Name: name, Count: count})
	}
	sort.Slice(d.Protocols, func(i, j int) bool { return d.Protocols[i].Count > d.Protocols[j].Count })

	if d.Alerts, err = st.RecentAlertEvents(200); err != nil {
		return nil, fmt.Errorf("uyarilar: %w", err)
	}
	// donem disi uyarilari ele
	cutoff := since.Unix()
	filtered := d.Alerts[:0]
	for _, a := range d.Alerts {
		if a.Ts >= cutoff {
			filtered = append(filtered, a)
			d.AlertCounts[a.Kind]++
		}
	}
	d.Alerts = filtered

	return d, nil
}
