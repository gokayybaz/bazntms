//go:build windows

package agent

import (
	"encoding/binary"
	"log/slog"
	"net"
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

// etwAttrSource, AttrSource'un Windows ETW implementasyonu (S20.8). Bayt
// sayımı Kernel-Network olaylarından gelir. L7 (SNI/Host) payload gerektirir —
// ETW taşımaz, nil döner; DNS S20.9'da DNS-Client sağlayıcısıyla eklenir.
type etwAttrSource struct {
	sess *etwSession

	mu       sync.Mutex
	flows    map[etwFlowKey]*[2]uint64 // [in, out]
	names    map[uint32]string
	lostSeen uint64

	doneCh chan struct{}
}

var _ AttrSource = (*etwAttrSource)(nil)

type etwFlowKey struct {
	pid      uint32
	proto    string // "tcp" | "udp"
	remoteIP string
	port     uint16
}

func newEtwAttrSource(_ AttrConfig) (*etwAttrSource, error) {
	sess, err := startETWSession(etwSessionName, kernelNetworkGUID, knwTCPUDPv4v6)
	if err != nil {
		return nil, err
	}
	e := &etwAttrSource{
		sess:   sess,
		flows:  map[etwFlowKey]*[2]uint64{},
		names:  map[uint32]string{},
		doneCh: make(chan struct{}),
	}
	go func() {
		defer close(e.doneCh)
		sess.Process(e.onRecord)
	}()
	return e, nil
}

func (e *etwAttrSource) onRecord(id uint16, userData []byte, headerPID uint32) {
	k, out, in, ok := kernelNetFlow(id, userData, headerPID)
	if !ok {
		return
	}
	e.mu.Lock()
	acc := e.flows[k]
	if acc == nil {
		if len(e.flows) >= 16384 {
			e.mu.Unlock()
			return
		}
		acc = &[2]uint64{}
		e.flows[k] = acc
	}
	acc[0] += in
	acc[1] += out
	e.mu.Unlock()
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

// DNSDeltas — S20.9'da Microsoft-Windows-DNS-Client sağlayıcısıyla eklenecek.
func (e *etwAttrSource) DNSDeltas() []telemetry.DNSSample { return nil }

func (e *etwAttrSource) Method() string { return "etw" }

func (e *etwAttrSource) Stop() {
	e.sess.Stop()
	<-e.doneCh
}

// ─── Kernel-Network olay çözümü (saf, test edilebilir) ───

// Kernel-Network manifest olay id'leri → (send?, udp?, v6?). TCP send/recv
// (10/11 v4, 26/27 v6), UDP send/recv (42/43 v4, 58/59 v6). Bağlantı /
// retransmit / disconnect vb. atlanır.
type etwNetShape struct{ send, udp, v6 bool }

var etwKernelNetIDs = map[uint16]etwNetShape{
	10: {send: true},
	11: {},
	26: {send: true, v6: true},
	27: {v6: true},
	42: {send: true, udp: true},
	43: {udp: true},
	58: {send: true, udp: true, v6: true},
	59: {udp: true, v6: true},
}

func kernelNetWanted(id uint16) bool { _, ok := etwKernelNetIDs[id]; return ok }

// kernelNetFlow, ham bir Kernel-Network olayını (event id + UserData blob'u +
// EventHeader PID'i) bir akış deltasına çözer. UserData düzeni sabittir:
//
//	v4:  PID(4) size(4) daddr(4) saddr(4) dport(2be) sport(2be) …
//	v6:  PID(4) size(4) daddr(16) saddr(16) dport(2be) sport(2be) …
//
// send olayında uzak uç = daddr:dport, recv'de saddr:sport.
func kernelNetFlow(id uint16, data []byte, headerPID uint32) (k etwFlowKey, out, in uint64, ok bool) {
	sh, wanted := etwKernelNetIDs[id]
	if !wanted {
		return
	}

	need := 20
	if sh.v6 {
		need = 44
	}
	if len(data) < need {
		return
	}

	pid := binary.LittleEndian.Uint32(data[0:4])
	if pid == 0 {
		pid = headerPID
	}
	size := binary.LittleEndian.Uint32(data[4:8])
	if size == 0 {
		return
	}

	var dIP, sIP net.IP
	var dPort, sPort uint16
	if sh.v6 {
		dIP, sIP = net.IP(append([]byte(nil), data[8:24]...)), net.IP(append([]byte(nil), data[24:40]...))
		dPort = binary.BigEndian.Uint16(data[40:42])
		sPort = binary.BigEndian.Uint16(data[42:44])
	} else {
		dIP, sIP = net.IP(append([]byte(nil), data[8:12]...)), net.IP(append([]byte(nil), data[12:16]...))
		dPort = binary.BigEndian.Uint16(data[16:18])
		sPort = binary.BigEndian.Uint16(data[18:20])
	}

	proto := "tcp"
	if sh.udp {
		proto = "udp"
	}

	if sh.send {
		k = etwFlowKey{pid: pid, proto: proto, remoteIP: dIP.String(), port: dPort}
		out = uint64(size)
	} else {
		k = etwFlowKey{pid: pid, proto: proto, remoteIP: sIP.String(), port: sPort}
		in = uint64(size)
	}
	ok = k.remoteIP != "" && k.remoteIP != "<nil>"
	return
}
