// bazntms-loadgen — Faz 4.5 yuk testi: sentetik agent filosu.
//
// Hub'a N adet sanal agent enroll eder ve gercek agent ritmiyle
// (varsayilan 30 sn batch) telemetri gonderir. Kapasite hedefleri
// (docs/enterprise-plan.html): 5000 agent @ 30 sn ≈ ≥170 ist/sn surekli.
//
// Ornek:
//
//	bazntms-loadgen -hub http://localhost:8080 -token <enroll-token> \
//	  -agents 5000 -interval 30 -duration 10m
//
// Istatistikler 5 saniyede bir ve kapanista yazilir (rps, p50/p95/p99).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

type stats struct {
	sent     atomic.Int64
	failed   atomic.Int64
	enrolled atomic.Int64
	hist     [9]atomic.Int64 // ms siniflari: 5,10,25,50,100,250,500,1000,1000+
	start    time.Time
}

var buckets = []int64{5, 10, 25, 50, 100, 250, 500, 1000}

func (s *stats) observe(d time.Duration) {
	ms := d.Milliseconds()
	i := 0
	for i < len(buckets) && ms > buckets[i] {
		i++
	}
	s.hist[i].Add(1)
}

func (s *stats) percentile(p float64) int64 {
	total := int64(0)
	for i := range s.hist {
		total += s.hist[i].Load()
	}
	if total == 0 {
		return 0
	}
	target := int64(p * float64(total))
	cum := int64(0)
	for i := range s.hist {
		cum += s.hist[i].Load()
		if cum >= target {
			if i == len(buckets) {
				return buckets[len(buckets)-1] * 2
			}
			return buckets[i]
		}
	}
	return 0
}

func (s *stats) report(elapsed time.Duration) {
	sent := s.sent.Load()
	rps := float64(sent) / elapsed.Seconds()
	fmt.Printf("[%6s] gonderilen=%d hata=%d rps=%.1f p50<=%dms p95<=%dms p99<=%dms\n",
		elapsed.Round(time.Second), sent, s.failed.Load(), rps,
		s.percentile(0.50), s.percentile(0.95), s.percentile(0.99))
}

var (
	processNames = []string{"chrome", "slack", "zoom", "firefox", "spotify", "code", "node", "python", "backup", "update"}
	remoteHosts  = []string{"1.1.1.1", "8.8.8.8", "10.0.20.5", "172.16.4.9", "142.250.74.14", "151.101.1.69", "104.16.132.229"}
	ifaceNames   = []string{"eth0", "eth1", "en0", "wlan0"}
	l7Hosts      = []string{"api.github.com", "www.google.com", "slack.com", "zoom.us", "cdn.jsdelivr.net", "s3.amazonaws.com", "login.microsoftonline.com", "update.example.com"}
)

func main() {
	fl := flag.NewFlagSet("bazntms-loadgen", flag.ExitOnError)
	hub := fl.String("hub", "http://localhost:8080", "hub adresi")
	enroll := fl.String("token", "", "enrollment token'i (zorunlu)")
	agents := fl.Int("agents", 100, "sanal agent sayisi")
	interval := fl.Int("interval", 30, "telemetri araligi (saniye)")
	duration := fl.Duration("duration", 0, "test suresi (0 = Ctrl+C'ye kadar)")
	spread := fl.Duration("spread", 30*time.Second, "agent baslangic rampasi (thundering herd onleme)")
	site := fl.String("site", "loadgen", "sanal agent site adi")
	fl.Parse(os.Args[1:])

	if *enroll == "" {
		fmt.Fprintln(os.Stderr, "-token zorunlu (hub'in logladigi enroll token)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		IdleConnTimeout:     90 * time.Second,
	}}

	st := &stats{start: time.Now()}
	fmt.Printf("loadgen basladi: %d agent, %v aralik, %v rampa → %s\n",
		*agents, time.Duration(*interval)*time.Second, *spread, *hub)

	var wg sync.WaitGroup
	for i := 0; i < *agents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// rampa: agent'lari yayarak baslat
			delay := time.Duration(float64(idx) / float64(*agents) * float64(*spread))
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			runAgent(ctx, client, *hub, *enroll, *site, idx, *interval, st)
		}(i)
	}

	// periyodik rapor
	ticker := time.NewTicker(5 * time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	for {
		select {
		case <-ticker.C:
			st.report(time.Since(st.start))
		case <-done:
			fmt.Println("tum agent'lar tamamlandi")
			st.report(time.Since(st.start))
			return
		case <-ctx.Done():
			fmt.Println("durduruluyor...")
			wg.Wait()
			st.report(time.Since(st.start))
			return
		}
	}
}

// runAgent, tek sanal agent'in omrungu: enroll → telemetri dongusu.
func runAgent(ctx context.Context, client *http.Client, hub, enrollToken, site string, idx, intervalSec int, st *stats) {
	name := fmt.Sprintf("loadgen-%04d", idx)
	hello := telemetry.AgentHello{
		Name: name, Site: site, Version: "loadgen-1.0",
		ProtocolVersion: 1, OS: runtime.GOOS, Arch: runtime.GOARCH,
		Capabilities: []string{"interfaces", "connections", "process_traffic", "l7", "dns"},
	}
	hb, _ := json.Marshal(hello)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hub+"/api/v1/agent/hello", bytes.NewReader(hb))
	if err != nil {
		return
	}
	req.Header.Set("X-Enroll-Token", enrollToken)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		st.failed.Add(1)
		return
	}
	defer resp.Body.Close()
	st.observe(time.Since(start))
	if resp.StatusCode != http.StatusOK {
		st.failed.Add(1)
		return
	}
	var reply telemetry.HubReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil || !reply.Accepted {
		st.failed.Add(1)
		return
	}
	st.enrolled.Add(1)
	token := reply.AgentToken

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(idx)<<20))
	nIfaces := 1 + rng.Intn(3)
	type counter struct {
		rx, tx, rxp, txp uint64
	}
	counters := make([]counter, nIfaces)

	tick := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		batch := telemetry.TelemetryBatch{TS: time.Now().Unix()}
		for j := 0; j < nIfaces; j++ {
			counters[j].rx += uint64(200_000 + rng.Intn(2_000_000))
			counters[j].tx += uint64(100_000 + rng.Intn(1_000_000))
			counters[j].rxp += uint64(200 + rng.Intn(2000))
			counters[j].txp += uint64(100 + rng.Intn(1000))
			batch.Interfaces = append(batch.Interfaces, telemetry.InterfaceSample{
				Name:    ifaceNames[j%len(ifaceNames)],
				RxBytes: counters[j].rx, TxBytes: counters[j].tx,
				RxPackets: counters[j].rxp, TxPackets: counters[j].txp,
			})
		}
		for j := 0; j < 3+rng.Intn(10); j++ {
			batch.Connections = append(batch.Connections, telemetry.ConnectionSample{
				Proto:      "tcp",
				LocalAddr:  fmt.Sprintf("10.%d.%d.%d:%d", idx%250, rng.Intn(250), rng.Intn(250), 40000+rng.Intn(20000)),
				RemoteAddr: fmt.Sprintf("%s:%d", remoteHosts[rng.Intn(len(remoteHosts))], 443),
				Status:     "ESTABLISHED", PID: int32(1000 + rng.Intn(5000)),
				Process: processNames[rng.Intn(len(processNames))],
			})
		}
		if rng.Intn(3) == 0 {
			for j := 0; j < 1+rng.Intn(5); j++ {
				in := uint64(rng.Intn(500_000))
				out := uint64(rng.Intn(200_000))
				if in+out == 0 {
					continue
				}
				batch.ProcessTraffic = append(batch.ProcessTraffic, telemetry.ProcessTrafficSample{
					PID:     int32(1000 + rng.Intn(5000)),
					Process: processNames[rng.Intn(len(processNames))],
					Proto:   "tcp", RemoteIP: remoteHosts[rng.Intn(len(remoteHosts))],
					Port: uint16(443), BytesIn: in, BytesOut: out,
				})
			}
		}
		// sentetik L7 (SNI/Host) + DNS — depolama/API/rapor/UI yolunu uctan uca
		// besler (gercek pcap yakalama parser'lari birim testli).
		if rng.Intn(2) == 0 {
			for j := 0; j < 1+rng.Intn(4); j++ {
				proc := processNames[rng.Intn(len(processNames))]
				host := l7Hosts[rng.Intn(len(l7Hosts))]
				pid := int32(1000 + rng.Intn(5000))
				kind := "tls"
				if rng.Intn(4) == 0 {
					kind = "http"
				}
				batch.L7 = append(batch.L7, telemetry.L7Sample{
					PID: pid, Process: proc, Kind: kind, Host: host,
					RemoteIP: remoteHosts[rng.Intn(len(remoteHosts))],
					Bytes:    uint64(200 + rng.Intn(4000)), Count: uint64(1 + rng.Intn(6)),
				})
				q := uint64(1 + rng.Intn(8))
				batch.DNS = append(batch.DNS, telemetry.DNSSample{
					PID: pid, Process: proc, Domain: host, Queries: q, Responses: q,
				})
			}
		}

		bb, _ := json.Marshal(batch)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, hub+"/api/v1/agent/telemetry", bytes.NewReader(bb))
		if err != nil {
			st.failed.Add(1)
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			st.failed.Add(1)
			continue
		}
		resp.Body.Close()
		st.observe(time.Since(start))
		if resp.StatusCode == http.StatusOK {
			st.sent.Add(1)
		} else {
			st.failed.Add(1)
		}
	}
}
