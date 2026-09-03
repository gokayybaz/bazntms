package store

import (
	"log"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/sysmon"
)

// Collector, canli yakalama verisini periyodik olarak SQLite'a yazar:
//   - saniyelik: trafik örnekleri (bps, pps, protokoller)
//   - dakikalik: uç nokta farkları (delta) + bağlantı olayları
//
// Eski kayitlarin temizligi (Prune) buradan KALDIRILDI — Collector yalnizca
// -capture=true iken calisir, oysa temizlik capture'dan bagimsiz gerekli
// (coklu-hub'da tum hub'lar -capture=false). Artik store.Maintainer yapar.
type Collector struct {
	engine *capture.Engine
	store  Store
	dbPath string
	stopCh chan struct{}
	doneCh chan struct{}

	mu        sync.Mutex
	lastCumul map[string]*endpointCumul // key: device|ip
	lastDNS   map[string]*dnsCumul      // key: domain
}

type endpointCumul struct {
	In, Out, Packets uint64
}

type dnsCumul struct {
	Queries, Responses uint64
}

func NewCollector(engine *capture.Engine, st Store, dbPath string) *Collector {
	return &Collector{
		engine:    engine,
		store:     st,
		dbPath:    dbPath,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		lastCumul: map[string]*endpointCumul{},
		lastDNS:   map[string]*dnsCumul{},
	}
}

func (c *Collector) Start() {
	go c.run()
}

func (c *Collector) Stop() {
	close(c.stopCh)
	<-c.doneCh
}

func (c *Collector) run() {
	defer close(c.doneCh)

	sec := time.NewTicker(time.Second)
	min := time.NewTicker(time.Minute)
	defer sec.Stop()
	defer min.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-sec.C:
			c.sampleOnce()
		case <-min.C:
			c.flushMinute()
		}
	}
}

func (c *Collector) sampleOnce() {
	snap := c.engine.Snapshot()
	if !snap.Running {
		return
	}
	sm := Sample{
		Ts:        time.Now().Unix(),
		Device:    snap.Device,
		BpsIn:     snap.BpsIn,
		BpsOut:    snap.BpsOut,
		BpsLocal:  snap.BpsLocal,
		Pps:       snap.Pps,
		Dropped:   uint64(snap.Dropped),
		Protocols: snap.Protocols,
	}
	if err := c.store.InsertSample(sm); err != nil {
		log.Printf("ornek kaydi hatasi: %v", err)
	}
}

func (c *Collector) flushMinute() {
	snap := c.engine.Snapshot()
	now := time.Now().Unix()

	// uc nokta farklari
	deltas := make([]EndpointDelta, 0, len(snap.TopEndpoints))
	for _, e := range snap.TopEndpoints {
		if e.Total == 0 {
			continue
		}
		key := snap.Device + "|" + e.IP
		cur := &endpointCumul{In: e.In, Out: e.Out, Packets: e.Packets}

		c.mu.Lock()
		last, ok := c.lastCumul[key]
		c.lastCumul[key] = cur
		c.mu.Unlock()

		if !ok {
			// ilk kez gorulen uc nokta: motor yeni baslamissa deger zaten fark,
			// yeniden baslatma tespit edilemiyorsa toplami yazmak en iyi tahmin
			deltas = append(deltas, EndpointDelta{
				Ts: now, Device: snap.Device, IP: e.IP, Hostname: e.Hostname,
				BytesIn: e.In, BytesOut: e.Out, Packets: e.Packets,
			})
			continue
		}
		dIn := sub(cur.In, last.In)
		dOut := sub(cur.Out, last.Out)
		dPk := sub(cur.Packets, last.Packets)
		if dIn+dOut+dPk == 0 {
			continue
		}
		deltas = append(deltas, EndpointDelta{
			Ts: now, Device: snap.Device, IP: e.IP, Hostname: e.Hostname,
			BytesIn: dIn, BytesOut: dOut, Packets: dPk,
		})
	}
	if err := c.store.InsertEndpointDeltas(deltas); err != nil {
		log.Printf("uç nokta kaydi hatasi: %v", err)
	}

	// DNS sorgu farklari
	dnsDeltas := make([]DNSDelta, 0, len(snap.TopDomains))
	for _, d := range snap.TopDomains {
		if d.Queries+d.Responses == 0 {
			continue
		}
		cur := &dnsCumul{Queries: d.Queries, Responses: d.Responses}

		c.mu.Lock()
		last, ok := c.lastDNS[d.Domain]
		c.lastDNS[d.Domain] = cur
		c.mu.Unlock()

		var dQ, dR uint64
		if !ok {
			// ilk gorulis: fark sifirlanmis olabilir, mevcut toplami yaz
			dQ, dR = d.Queries, d.Responses
		} else {
			dQ = subUint(d.Queries, last.Queries)
			dR = subUint(d.Responses, last.Responses)
		}
		if dQ+dR == 0 {
			continue
		}
		dnsDeltas = append(dnsDeltas, DNSDelta{Ts: now, Domain: d.Domain, Queries: dQ, Responses: dR})
	}
	if err := c.store.InsertDNSDeltas(dnsDeltas); err != nil {
		log.Printf("dns kaydi hatasi: %v", err)
	}

	// baglanti olaylari
	cons := sysmon.ListConnections()
	events := make([]ConnectionEvent, 0, len(cons))
	for _, cn := range cons {
		if cn.RemoteAddr == "" && cn.Status == "LISTEN" {
			continue // dinleme soketlerini atla
		}
		events = append(events, ConnectionEvent{
			Ts: now, Proto: cn.Proto, LocalAddr: cn.LocalAddr,
			RemoteAddr: cn.RemoteAddr, Status: cn.Status,
			PID: cn.PID, Process: cn.Process, Count: cn.Count,
		})
	}
	if err := c.store.InsertConnectionEvents(events); err != nil {
		log.Printf("baglanti kaydi hatasi: %v", err)
	}
}

func sub(a, b uint64) uint64 {
	if a < b {
		return a // motor sifirlandi: yeni degeri fark olarak yaz
	}
	return a - b
}

func subUint(a, b uint64) uint64 { return sub(a, b) }
