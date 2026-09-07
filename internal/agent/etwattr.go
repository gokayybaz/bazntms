//go:build windows

package agent

import (
	"log/slog"
	"sync"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

// etwSessionName, gerçek-zamanlı ETW oturumunun adı. Agent yeniden başlarsa
// aynı adlı yetim oturum StartTraceW'de temizlenir (bkz. etwSession.start).
const etwSessionName = "bazNTMS-Attr"

// Microsoft-Windows-Kernel-Network {7DD42A49-5329-4832-8DFD-43D979153A88}
var kernelNetworkGUID = windows.GUID{
	Data1: 0x7DD42A49, Data2: 0x5329, Data3: 0x4832,
	Data4: [8]byte{0x8D, 0xFD, 0x43, 0xD9, 0x79, 0x15, 0x3A, 0x88},
}

// Kernel-Network anahtar sözcükleri: TCP+UDP, IPv4+IPv6.
const knwTCPUDPv4v6 = 0x1 | 0x2 | 0x4 | 0x8

// Microsoft-Windows-DNS-Client {1C95126E-7EEA-49A9-A3FE-A378B03DDB4D}
var dnsClientGUID = windows.GUID{
	Data1: 0x1C95126E, Data2: 0x7EEA, Data3: 0x49A9,
	Data4: [8]byte{0xA3, 0xFE, 0xA3, 0x78, 0xB0, 0x3D, 0xDB, 0x4D},
}

// etwAttrSource, AttrSource'un Windows ETW implementasyonu. Bayt sayımı
// Kernel-Network, DNS görünürlüğü DNS-Client olaylarından gelir. L7 (SNI/Host)
// payload gerektirir — ETW taşımaz, nil döner.
type etwAttrSource struct {
	sess *etwSession

	mu       sync.Mutex
	flows    map[etwFlowKey]*[2]uint64 // [in, out] — her Deltas'ta boşaltılır
	dns      map[etwDNSKey]*dnsAgg     // her DNSDeltas'ta boşaltılır
	names    map[uint32]string
	lostSeen uint64

	doneCh chan struct{}
}

var _ AttrSource = (*etwAttrSource)(nil)

// etwFlowKey saf ayrıştırıcılarla birlikte etwparse.go'da tanımlıdır.

type etwDNSKey struct {
	pid    uint32
	domain string
}

func newEtwAttrSource(_ AttrConfig) (*etwAttrSource, error) {
	sess, err := startETWSession(etwSessionName,
		etwProvider{GUID: kernelNetworkGUID, Keywords: knwTCPUDPv4v6},
		etwProvider{GUID: dnsClientGUID, Keywords: 0xFFFFFFFFFFFFFFFF},
	)
	if err != nil {
		return nil, err
	}
	e := &etwAttrSource{
		sess:   sess,
		flows:  map[etwFlowKey]*[2]uint64{},
		dns:    map[etwDNSKey]*dnsAgg{},
		names:  map[uint32]string{},
		doneCh: make(chan struct{}),
	}
	slog.Info("ETW atıf motoru aktif — süreç trafiği + DNS",
		"not", "L7 (SNI/Host) ETW'de yok; gerekiyorsa -collect-method=pcap + Npcap")
	go func() {
		defer close(e.doneCh)
		sess.Process(etwWanted, e.onEvent)
	}()
	return e, nil
}

func etwWanted(id uint16) bool { return kernelNetWanted(id) || dnsClientWanted(id) }

func (e *etwAttrSource) onEvent(ev etwEvent) {
	switch {
	case guidEqual(&ev.Provider, &kernelNetworkGUID):
		if k, out, in, ok := kernelNetFlow(ev.ID, ev.Data, ev.PID); ok {
			e.addFlow(k, out, in)
		}
	case guidEqual(&ev.Provider, &dnsClientGUID):
		if dom, isResp, ok := dnsClientDomain(ev.ID, ev.Data); ok {
			e.addDNS(ev.PID, dom, isResp)
		}
	}
}

func (e *etwAttrSource) addFlow(k etwFlowKey, out, in uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	acc := e.flows[k]
	if acc == nil {
		if len(e.flows) >= 16384 {
			return
		}
		acc = &[2]uint64{}
		e.flows[k] = acc
	}
	acc[0] += in
	acc[1] += out
}

func (e *etwAttrSource) addDNS(pid uint32, domain string, isResp bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	k := etwDNSKey{pid: pid, domain: domain}
	a := e.dns[k]
	if a == nil {
		if len(e.dns) >= 4000 {
			return
		}
		a = &dnsAgg{}
		e.dns[k] = a
	}
	if isResp {
		a.responses++
	} else {
		a.queries++
	}
}

// Deltas, son çağrıdan bu yana süreç bazlı trafik farklarını döndürür.
func (e *etwAttrSource) Deltas() []telemetry.ProcessTrafficSample {
	e.mu.Lock()
	flows := e.flows
	e.flows = map[etwFlowKey]*[2]uint64{}
	e.mu.Unlock()

	if lost := e.sess.LostEvents(); lost > e.lostSeen {
		slog.Warn("ETW olayları düşüyor — host çok yoğun olabilir", "toplam_kayip", lost)
		e.lostSeen = lost
	}

	out := make([]telemetry.ProcessTrafficSample, 0, len(flows))
	for k, acc := range flows {
		if acc[0]+acc[1] == 0 {
			continue
		}
		out = append(out, telemetry.ProcessTrafficSample{
			PID:      int32(k.pid),
			Process:  e.procName(k.pid),
			Proto:    k.proto,
			RemoteIP: k.remoteIP,
			Port:     k.port,
			BytesIn:  acc[0],
			BytesOut: acc[1],
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func (e *etwAttrSource) procName(pid uint32) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n, ok := e.names[pid]; ok {
		return n
	}
	n := ""
	if p, err := process.NewProcess(int32(pid)); err == nil {
		if v, err := p.Name(); err == nil {
			n = v
		}
	}
	if len(e.names) > 8192 {
		e.names = map[uint32]string{}
	}
	e.names[pid] = n
	return n
}

// L7Deltas — ETW payload taşımaz; L7 için pcap (-collect-method=pcap) gerekir.
func (e *etwAttrSource) L7Deltas() []telemetry.L7Sample { return nil }

// DNSDeltas, son çağrıdan bu yana süreç bazlı DNS sorgu/yanıt farklarını
// döndürür (DNS-Client 3006/3008 olaylarından).
func (e *etwAttrSource) DNSDeltas() []telemetry.DNSSample {
	e.mu.Lock()
	dns := e.dns
	e.dns = map[etwDNSKey]*dnsAgg{}
	e.mu.Unlock()

	out := make([]telemetry.DNSSample, 0, len(dns))
	for k, a := range dns {
		if a.queries+a.responses == 0 {
			continue
		}
		out = append(out, telemetry.DNSSample{
			PID:       int32(k.pid),
			Process:   e.procName(k.pid),
			Domain:    k.domain,
			Queries:   a.queries,
			Responses: a.responses,
		})
	}
	if len(out) > 500 {
		out = out[:500]
	}
	return out
}

func (e *etwAttrSource) Method() string { return "etw" }

func (e *etwAttrSource) Stop() {
	e.sess.Stop()
	<-e.doneCh
}
