//go:build linux

package agent

import (
	"fmt"
	"time"

	"github.com/gokayybaz/bazntms/pkg/proctraffic"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// l7HelperFilter, eBPF modunda L7 (SNI/Host) için açılan dar kapsamlı pcap
// handle'ının BPF filtresi. eBPF bayt sayımını + DNS'i zaten yakaladığından bu
// handle yalnız web istek yönünü (giden, yaygın TLS/HTTP portları, PUSH'lu
// segment) taşır — paket hacmi tam yakalamaya göre çok düşük. Standart-dışı
// portlar (ör. 9443) bu modda L7 göremez (kabul edilmiş sınır — bkz.
// docs/decisions/0007).
const l7HelperFilter = "tcp and (dst port 443 or dst port 80 or dst port 8443 or dst port 8080) and (tcp[tcpflags] & tcp-push != 0)"

const (
	l7RefreshInterval = 1500 * time.Millisecond
	l7PendingTTL      = 5 * time.Second
	l7PendingMax      = 256
)

// l7Helper, eBPF modunda çalışan L7 sniffer'ı: dar filtreli bir pcap handle'dan
// giden TCP payload'larını okur, süreç eşlemesi yapıp l7Tracker'a besler.
// CAP_NET_RAW ister — açılamazsa çağıran taraf L7'yi sessiz boş bırakır.
type l7Helper struct {
	handle   *pcap.Handle
	prov     proctraffic.Provider
	track    *l7Tracker
	localIPs map[string]struct{}
	pending  []pendingL7 // süreç eşlemesi henüz gelmemiş SNI/Host'lar
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// pendingL7, ClientHello/HTTP isteği geldiğinde soket→PID anlığı henüz o
// bağlantıyı içermiyorsa buraya konur; sonraki refresh'te tekrar denenir.
type pendingL7 struct {
	srcIP, dstIP string
	sport, dport uint16
	host, kind   string
	length       uint64
	at           time.Time
}

func newL7Helper(iface string, track *l7Tracker) (*l7Helper, error) {
	if iface == "" {
		return nil, fmt.Errorf("L7 yardımcı handle için arayüz belirtilmedi")
	}
	h, err := pcap.OpenLive(iface, attrSnapLen, false, time.Second)
	if err != nil {
		return nil, err
	}
	if err := h.SetBPFFilter(l7HelperFilter); err != nil {
		h.Close()
		return nil, fmt.Errorf("L7 BPF filtresi: %w", err)
	}
	hp := &l7Helper{
		handle:   h,
		prov:     proctraffic.NewProvider(),
		track:    track,
		localIPs: proctraffic.LocalIPs(),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go hp.loop()
	return hp, nil
}

func (h *l7Helper) Stop() {
	close(h.stopCh)
	<-h.doneCh
	h.handle.Close()
}

func (h *l7Helper) loop() {
	defer close(h.doneCh)

	src := gopacket.NewPacketSource(h.handle, h.handle.LinkType())
	src.Lazy = true
	src.NoCopy = true
	packets := src.Packets()

	refreshT := time.NewTicker(l7RefreshInterval)
	defer refreshT.Stop()
	ipT := time.NewTicker(15 * time.Second)
	defer ipT.Stop()

	var full map[proctraffic.Key]ProcInfoAlias
	var index map[portKey]ProcInfoAlias
	refresh := func() {
		snap := h.prov.Snapshot()
		full = make(map[proctraffic.Key]ProcInfoAlias, len(snap))
		index = make(map[portKey]ProcInfoAlias, len(snap))
		for k, pi := range snap {
			full[proctraffic.Key(k)] = pi
			index[portKey{proto: k.Proto, lp: k.LocalPort, rp: k.RemotePort}] = pi
		}
	}
	refresh()

	for {
		select {
		case <-h.stopCh:
			return
		case <-refreshT.C:
			refresh()
			h.retryPending(full, index)
		case <-ipT.C:
			h.localIPs = proctraffic.LocalIPs()
		case pkt := <-packets:
			if pkt == nil {
				continue
			}
			h.handlePacket(pkt, full, index)
		}
	}
}

func (h *l7Helper) handlePacket(pkt gopacket.Packet, full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
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
	tcp, ok := pkt.TransportLayer().(*layers.TCP)
	if !ok || len(tcp.Payload) == 0 {
		return
	}
	// BPF filtresi giden yönü hedefler; yine de yerel-kaynak kontrolü yap
	// (bu portlarda yerel bir sunucu varsa gelen istekleri elemek için).
	if _, local := h.localIPs[srcIP]; !local {
		return
	}
	host, kind := sniffL7(tcp.Payload)
	if host == "" {
		return
	}
	length := uint64(pkt.Metadata().Length)
	if length == 0 {
		length = uint64(pkt.Metadata().CaptureLength)
	}
	sport, dport := uint16(tcp.SrcPort), uint16(tcp.DstPort)

	if info, ok := lookupProc("tcp", srcIP, sport, dstIP, dport, full, index); ok && (info.Process != "" || info.PID != 0) {
		h.track.record(info.PID, info.Process, dstIP, host, kind, length)
		return
	}
	// yeni bağlantı — soket anlığı henüz güncel değil, sonraki refresh'e bırak
	if len(h.pending) >= l7PendingMax {
		h.pending = h.pending[1:]
	}
	h.pending = append(h.pending, pendingL7{
		srcIP: srcIP, dstIP: dstIP, sport: sport, dport: dport,
		host: host, kind: kind, length: length, at: time.Now(),
	})
}

func (h *l7Helper) retryPending(full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
	if len(h.pending) == 0 {
		return
	}
	now := time.Now()
	kept := h.pending[:0]
	for _, p := range h.pending {
		if now.Sub(p.at) > l7PendingTTL {
			continue
		}
		if info, ok := lookupProc("tcp", p.srcIP, p.sport, p.dstIP, p.dport, full, index); ok && (info.Process != "" || info.PID != 0) {
			h.track.record(info.PID, info.Process, p.dstIP, p.host, p.kind, p.length)
			continue
		}
		kept = append(kept, p)
	}
	h.pending = kept
}
