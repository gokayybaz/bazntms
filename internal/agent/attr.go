package agent

import (
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/pkg/proctraffic"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// AttrEngine, yakalanan paketleri sureclere atfeder ve donemlik delta
// uretir (nethogs yontemi). Agent root/admin olarak calisirken tam
// kapsamli; izin yoksa atif kismi olur, telemetri aksamaz.
type AttrEngine struct {
	mu       sync.Mutex
	prov     proctraffic.Provider
	handle   *pcap.Handle
	localIPs map[string]struct{}
	totals   map[attrKey][2]uint64 // [in, out] kumulatif
	lastSent map[attrKey][2]uint64

	stopCh chan struct{}
	doneCh chan struct{}
}

type attrKey struct {
	pid      int32
	process  string
	proto    string
	remoteIP string
	port     uint16
}

type portKey struct {
	proto string
	lp    uint16
	rp    uint16
}

// NewAttrEngine, verilen arayuzde atf yakalamasini baslatir.
func NewAttrEngine(iface string) (*AttrEngine, error) {
	handle, err := pcap.OpenLive(iface, 128, false, time.Second)
	if err != nil {
		return nil, err
	}
	if err := handle.SetBPFFilter("ip or ip6"); err != nil {
		handle.Close()
		return nil, err
	}
	e := &AttrEngine{
		prov:     proctraffic.NewProvider(),
		handle:   handle,
		localIPs: proctraffic.LocalIPs(),
		totals:   map[attrKey][2]uint64{},
		lastSent: map[attrKey][2]uint64{},
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go e.loop()
	return e, nil
}

func (e *AttrEngine) Stop() {
	close(e.stopCh)
	<-e.doneCh
	e.handle.Close()
}

func (e *AttrEngine) loop() {
	defer close(e.doneCh)

	linkType := e.handle.LinkType()
	provTicker := time.NewTicker(3 * time.Second)
	defer provTicker.Stop()
	ipTicker := time.NewTicker(15 * time.Second)
	defer ipTicker.Stop()

	var index map[portKey]ProcInfoAlias = nil
	var full map[proctraffic.Key]ProcInfoAlias = nil
	refresh := func() {
		snap := e.prov.Snapshot()
		full = make(map[proctraffic.Key]ProcInfoAlias, len(snap))
		index = make(map[portKey]ProcInfoAlias, len(snap))
		for k, pi := range snap {
			full[proctraffic.Key(k)] = pi
			index[portKey{proto: k.Proto, lp: k.LocalPort, rp: k.RemotePort}] = pi
		}
	}
	refresh()

	src := gopacket.NewPacketSource(e.handle, linkType)
	src.Lazy = true
	src.NoCopy = true
	packets := src.Packets()

	for {
		select {
		case <-e.stopCh:
			return
		case <-provTicker.C:
			e.mu.Lock()
			refresh()
			e.mu.Unlock()
		case <-ipTicker.C:
			e.mu.Lock()
			e.localIPs = proctraffic.LocalIPs()
			e.mu.Unlock()
		case pkt := <-packets:
			if pkt == nil {
				continue
			}
			e.mu.Lock()
			e.attribute(pkt, full, index)
			e.mu.Unlock()
		}
	}
}

// ProcInfoAlias, proctraffic.ProcInfo ile ayni yapidir (import dongususuz kullanim).
type ProcInfoAlias = proctraffic.ProcInfo

func (e *AttrEngine) attribute(pkt gopacket.Packet, full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
	nl := pkt.NetworkLayer()
	if nl == nil {
		return
	}
	var srcIP, dstIP string
	switch l := nl.(type) {
	case *layers.IPv4:
		srcIP, dstIP = l.SrcIP.String(), l.DstIP.String()
	case *layers.IPv6:
		srcIP, dstIP = l.SrcIP.String(), l.DstIP.String()
	default:
		return
	}
	length := uint64(pkt.Metadata().Length)
	if length == 0 {
		length = uint64(pkt.Metadata().CaptureLength)
	}

	tl := pkt.TransportLayer()
	if tl == nil {
		return
	}
	var proto string
	var sport, dport uint16
	switch t := tl.(type) {
	case *layers.TCP:
		proto = "tcp"
		sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
	case *layers.UDP:
		proto = "udp"
		sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
	default:
		return
	}

	// once tam anahtarlar, sonra yalnizca port eslemesi
	var info ProcInfoAlias
	var ok bool
	if info, ok = full[proctraffic.Key{Proto: proto, LocalIP: srcIP, LocalPort: sport, RemoteIP: dstIP, RemotePort: dport}]; !ok {
		if info, ok = full[proctraffic.Key{Proto: proto, LocalIP: dstIP, LocalPort: dport, RemoteIP: srcIP, RemotePort: sport}]; !ok {
			if info, ok = index[portKey{proto: proto, lp: sport, rp: dport}]; !ok {
				if info, ok = index[portKey{proto: proto, lp: dport, rp: sport}]; !ok {
					return
				}
			}
		}
	}
	if info.Process == "" && info.PID == 0 {
		return
	}

	_, srcLocal := e.localIPs[srcIP]
	_, dstLocal := e.localIPs[dstIP]

	key := attrKey{pid: info.PID, process: info.Process, proto: proto, remoteIP: dstIP, port: dport}
	if srcLocal { // giden: uzak taraf dst
		tot := e.totals[key]
		tot[1] += length
		e.totals[key] = tot
		return
	}
	key = attrKey{pid: info.PID, process: info.Process, proto: proto, remoteIP: srcIP, port: sport}
	tot := e.totals[key]
	tot[0] += length
	e.totals[key] = tot
	_ = dstLocal
}

// Deltas, son gonderimden bu yana surec bazli trafik farklarini dondurur.
func (e *AttrEngine) Deltas() []telemetry.ProcessTrafficSample {
	e.mu.Lock()
	defer e.mu.Unlock()

	type acc struct {
		in, out uint64
	}
	deltas := make(map[attrKey]*acc, len(e.totals))
	for k, cur := range e.totals {
		last := e.lastSent[k]
		dIn, dOut := delta(cur[0], last[0]), delta(cur[1], last[1])
		if dIn+dOut == 0 {
			continue
		}
		e.lastSent[k] = cur
		a := deltas[k]
		if a == nil {
			a = &acc{}
			deltas[k] = a
		}
		a.in += dIn
		a.out += dOut
	}

	out := make([]telemetry.ProcessTrafficSample, 0, len(deltas))
	for k, a := range deltas {
		out = append(out, telemetry.ProcessTrafficSample{
			PID:      k.pid,
			Process:  k.process,
			Proto:    k.proto,
			RemoteIP: k.remoteIP,
			Port:     k.port,
			BytesIn:  a.in,
			BytesOut: a.out,
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func delta(cur, last uint64) uint64 {
	if cur < last {
		return cur // motor sifirlandi
	}
	return cur - last
}
