package report

import (
	"fmt"
	"sort"
	"time"

	"github.com/gokayybaz/bazntms/internal/geoip"
	"github.com/gokayybaz/bazntms/internal/store"
)

// bucketSecs, filo trafik serisinin kova genisligi. 5 dk: 7 gunluk pencerede
// ~2000 kova — donem ozeti, gunluk toplam ve percentile'lar icin yeterli
// cozunurluk, tek tarama.
const bucketSecs = 300

// Data, raporun tamami; ayni modelden hem HTML hem PDF uretilir.
// Kaynak: agent arayuz telemetrisi + NetFlow + agent surec trafigi
// (hub yerel yakalamasi / `samples` tablosu DEGIL — bkz. store.Fleet*).
type Data struct {
	GeneratedAt time.Time `json:"generated_at"`
	Days        int       `json:"days"`

	Sources    int     `json:"sources"` // rapora veri katan agent sayisi
	TotalGB    float64 `json:"total_gb"`
	AvgInBps   float64 `json:"avg_in_bps"`
	AvgOutBps  float64 `json:"avg_out_bps"`
	PeakInBps  float64 `json:"peak_in_bps"`
	PeakOutBps float64 `json:"peak_out_bps"`

	Daily        []store.DayTotal            `json:"daily"`
	Agents       []store.AgentWithRates      `json:"agents"`
	TopEndpoints []store.EndpointDelta       `json:"top_endpoints"`
	TopProcesses []store.ProcessTrafficUsage `json:"top_processes"`
	TopDomains   []store.AgentDNSUsage       `json:"top_domains"` // agent DNS görünürlüğü (fleet)
	Protocols    []ProtoCount                `json:"protocols"`
	Alerts       []store.AlertEvent          `json:"alerts"`

	AlertCounts map[string]int `json:"alert_counts"`

	// Empty, dönemde hiç filo trafiği kaydı bulunmadığını gösterir — şablon
	// sıfır dolu tablolar yerine açıklayıcı bir uyarı bandı basar.
	Empty bool `json:"empty"`
}

type ProtoCount struct {
	Name  string `json:"name"`
	Count uint64 `json:"count"`
}

// Build, store'dan son `days` gunun filo verisini toplayip rapor modelini kurar.
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

	buckets, err := st.FleetTrafficBuckets(since, bucketSecs)
	if err != nil {
		return nil, fmt.Errorf("filo trafik serisi: %w", err)
	}
	summarize(d, buckets)
	d.Daily = rollupDaily(buckets)

	agents, err := st.ListAgents(2*time.Minute, "")
	if err != nil {
		return nil, fmt.Errorf("agent filosu: %w", err)
	}
	d.Agents = agents
	for _, a := range agents {
		if len(a.Rates) > 0 {
			d.Sources++
		}
	}

	if d.TopEndpoints, err = st.FleetTopEndpoints(since, 15); err != nil {
		return nil, fmt.Errorf("hedefler: %w", err)
	}
	if geo != nil && geo.Enabled() {
		for i := range d.TopEndpoints {
			info := geo.Lookup(d.TopEndpoints[i].IP)
			d.TopEndpoints[i].Country = info.Country
			d.TopEndpoints[i].ASN = info.ASN
		}
	}
	if d.TopProcesses, err = st.TopProcessTraffic(since, 0, 10); err != nil {
		return nil, fmt.Errorf("surecler: %w", err)
	}
	if d.TopDomains, err = st.TopAgentDNS(since, 0, 15); err != nil {
		return nil, fmt.Errorf("dns: %w", err)
	}

	protos, err := st.FleetProtocolTotals(since)
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

	d.Empty = len(buckets) == 0 && len(d.TopEndpoints) == 0 && len(d.TopProcesses) == 0

	return d, nil
}

// summarize, kova serisinden donem ozetini (ort/zirve bit/sn, toplam GB) doldurur.
// store.Bucket.In/Out bayt/sn'dir → bit/sn icin ×8.
func summarize(d *Data, buckets []store.Bucket) {
	var sumIn, sumOut, totalBytes float64
	for _, b := range buckets {
		inBps, outBps := b.In*8, b.Out*8
		sumIn += inBps
		sumOut += outBps
		if inBps > d.PeakInBps {
			d.PeakInBps = inBps
		}
		if outBps > d.PeakOutBps {
			d.PeakOutBps = outBps
		}
		totalBytes += (b.In + b.Out) * float64(bucketSecs)
	}
	if n := float64(len(buckets)); n > 0 {
		d.AvgInBps = sumIn / n
		d.AvgOutBps = sumOut / n
	}
	d.TotalGB = totalBytes / 1e9
}

// rollupDaily, 5 dk kovalari yerel gece yarisina hizali gunlere toplar.
// DayTotal.Samples = gunde kapsanan saniye sayisi (sablonun dailyGB
// yardimcisi bunu "saniye" olarak yorumlar).
func rollupDaily(buckets []store.Bucket) []store.DayTotal {
	_, offset := time.Now().Zone()
	type acc struct {
		sumIn, sumOut   float64
		peakIn, peakOut float64
		n               int64
	}
	byDay := map[int64]*acc{}
	for _, b := range buckets {
		day := ((b.Ts+int64(offset))/86400)*86400 - int64(offset)
		a := byDay[day]
		if a == nil {
			a = &acc{}
			byDay[day] = a
		}
		inBps, outBps := b.In*8, b.Out*8
		a.sumIn += inBps
		a.sumOut += outBps
		a.n++
		if inBps > a.peakIn {
			a.peakIn = inBps
		}
		if outBps > a.peakOut {
			a.peakOut = outBps
		}
	}
	days := make([]int64, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	out := make([]store.DayTotal, 0, len(days))
	for _, day := range days {
		a := byDay[day]
		dt := store.DayTotal{Day: day, Samples: a.n * int64(bucketSecs)}
		if a.n > 0 {
			dt.AvgBpsIn = a.sumIn / float64(a.n)
			dt.AvgBpsOut = a.sumOut / float64(a.n)
		}
		dt.PeakBpsIn = a.peakIn
		dt.PeakBpsOut = a.peakOut
		out = append(out, dt)
	}
	return out
}
