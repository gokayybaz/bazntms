package agent

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/pkg/proctraffic"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// attrSnapLen, atif + L7 inspeksiyonu icin yakalama basina alinan bayt.
// Salt atif icin ~54 bayt yeter; TLS ClientHello SNI uzantisi ve HTTP istek
// satiri+Host cogu zaman 128'i asar, bu yuzden 600.
const attrSnapLen = 600

// pcapAttrSource, yakalanan paketleri sureclere atfeder ve donemlik delta
// uretir (nethogs yontemi). Ayrica TLS SNI + HTTP Host cikararak surec bazli
// uygulama gorunurlugu (L7) toplar. Agent root/admin olarak calisirken tam
// kapsamli; izin yoksa atif kismi olur, telemetri aksamaz.
type pcapAttrSource struct {
	mu       sync.Mutex
	prov     proctraffic.Provider
	handle   *pcap.Handle
	loHandle *pcap.Handle // loopback stub-resolver DNS (best-effort; nil olabilir)
	localIPs map[string]struct{}
	totals   map[attrKey][2]uint64 // [in, out] kumulatif
	lastSent map[attrKey][2]uint64
	l7       *l7Tracker         // surec × (tls/http) × host — kendi kilidi
	dns      map[dnsKey]*dnsAgg // surec × domain — sorgu/yanit kumulatif
	dnsSent  map[dnsKey][2]uint64

	stopCh chan struct{}
	doneCh chan struct{}
}

type l7Key struct {
	pid      int32
	process  string
	kind     string // "tls" | "http"
	host     string
	remoteIP string
}

type l7Agg struct {
	bytes uint64
	count uint64
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

// newPcapAttrSource, verilen arayuzde atf yakalamasini baslatir. Windows'ta
// friendly arayuz adi (\Device\NPF_{GUID} degil "Ethernet" gibi) once pcap
// cihaz adina cevrilir — bu adim yalniz pcap arka ucu kuruldugunda calisir,
// yani ETW/eBPF varsayilaninda Npcap hic aranmaz.
func newPcapAttrSource(iface string) (*pcapAttrSource, error) {
	if dev, rerr := ResolvePcapDevice(iface); rerr != nil {
		slog.Debug("pcap cihazi cozulemedi, ham arayuz adi denenecek", "iface", iface, "err", rerr)
	} else if dev != iface {
		slog.Info("pcap cihazi cozuldu", "arayuz", iface, "cihaz", dev)
		iface = dev
	}
	handle, err := pcap.OpenLive(iface, attrSnapLen, false, time.Second)
	if err != nil {
		return nil, err
	}
	if err := handle.SetBPFFilter("ip or ip6"); err != nil {
		handle.Close()
		return nil, err
	}
	e := &pcapAttrSource{
		prov:     proctraffic.NewProvider(),
		handle:   handle,
		localIPs: proctraffic.LocalIPs(),
		totals:   map[attrKey][2]uint64{},
		lastSent: map[attrKey][2]uint64{},
		l7:       newL7Tracker(),
		dns:      map[dnsKey]*dnsAgg{},
		dnsSent:  map[dnsKey][2]uint64{},
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	// Loopback stub-resolver DNS: ana handle loopback-disi tek arayuzu
	// dinledigi icin systemd-resolved / dnsmasq / Docker gomulu DNS'e giden
	// sorgular gorunmez. Ayri bir loopback handle'i (BPF: udp) acilir;
	// acilamazsa DNS gorunurlugu kisitli kalir ama atif/telemetri aksamaz.
	if lo, lerr := openLoopbackDNS(); lerr != nil {
		slog.Debug("loopback DNS yakalama yok — stub-resolver DNS gorunurlugu kisitli", "err", lerr)
	} else {
		e.loHandle = lo
		slog.Info("loopback DNS yakalama aktif")
	}
	go e.loop()
	return e, nil
}

func (e *pcapAttrSource) Stop() {
	close(e.stopCh)
	<-e.doneCh
	e.handle.Close()
	if e.loHandle != nil {
		e.loHandle.Close()
	}
}

// Method, AttrSource arayüzü için: bu arka uç pcap tabanlıdır.
func (e *pcapAttrSource) Method() string { return "pcap" }

func (e *pcapAttrSource) loop() {
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

	// Loopback handle (varsa) yalnizca UDP tasir → attributeDNS: e.totals'a
	// yazmaz, yalnizca DNS gorunurlugunu besler. loHandle yoksa loPackets nil
	// kalir ve o select dali hic tetiklenmez.
	var loPackets <-chan gopacket.Packet
	if e.loHandle != nil {
		loSrc := gopacket.NewPacketSource(e.loHandle, e.loHandle.LinkType())
		loSrc.Lazy = true
		loSrc.NoCopy = true
		loPackets = loSrc.Packets()
	}

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
		case pkt := <-loPackets:
			if pkt == nil {
				continue
			}
			e.mu.Lock()
			e.attributeDNS(pkt, full, index)
			e.mu.Unlock()
		}
	}
}

// ProcInfoAlias, proctraffic.ProcInfo ile ayni yapidir (import dongususuz kullanim).
type ProcInfoAlias = proctraffic.ProcInfo

func (e *pcapAttrSource) attribute(pkt gopacket.Packet, full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
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
	var payload []byte
	switch t := tl.(type) {
	case *layers.TCP:
		proto = "tcp"
		sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
		payload = t.Payload
	case *layers.UDP:
		proto = "udp"
		sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
		payload = t.Payload
	default:
		return
	}

	// DNS gorunurlugu: UDP/53 sorgu ve yanitlarindaki alan adlari. Genel atif
	// gatinden ONCE calisir — DNS soketi kisa omurlu oldugu icin surece
	// atfedilemese bile alan adi kayda gecmeli.
	if proto == "udp" && (sport == 53 || dport == 53) {
		e.sniffDNS(srcIP, sport, dstIP, dport, payload, full, index)
	}

	info, ok := lookupProc(proto, srcIP, sport, dstIP, dport, full, index)
	if !ok || (info.Process == "" && info.PID == 0) {
		return
	}

	_, srcLocal := e.localIPs[srcIP]
	_, dstLocal := e.localIPs[dstIP]

	// L7 uygulama gorunurlugu: giden TCP istegde SNI / HTTP Host cikar
	if srcLocal && proto == "tcp" && len(payload) > 0 {
		e.l7.observe(info.PID, info.Process, dstIP, payload, length)
	}

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

// attributeDNS, loopback handle'indan gelen bir UDP paketini DNS gorunurlugu
// icin isler. attribute()'in tam yolundan farki: e.totals'a hic yazmaz
// (loopback trafigi surec trafik sayaclarina katilmaz) ve port gati yoktur —
// Docker gomulu DNS sorgunun hedef portunu DNAT ile degistirir, o yuzden
// karar parseDNSNames'e birakilir.
func (e *pcapAttrSource) attributeDNS(pkt gopacket.Packet, full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
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
	udp, ok := pkt.TransportLayer().(*layers.UDP)
	if !ok {
		return
	}
	e.sniffDNS(srcIP, uint16(udp.SrcPort), dstIP, uint16(udp.DstPort), udp.Payload, full, index)
}

// sniffDNS, bir UDP payload'ini DNS mesaji olarak cozmeye calisir; alan adi
// cikarsa surece atfedip kaydeder. Surec atfi best-effort'tur: DNS soketleri
// milisaniyelik oldugu icin /proc/net/udp anligina cogu zaman yakalanmaz —
// o durumda alan adi bos surecle (yalnizca domain gorunurlugu) kaydedilir.
func (e *pcapAttrSource) sniffDNS(srcIP string, sport uint16, dstIP string, dport uint16, payload []byte,
	full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) {
	if len(payload) < 12 {
		return
	}
	names, isResp := parseDNSNames(payload)
	if len(names) == 0 {
		return
	}
	info, _ := lookupDNSProc(srcIP, sport, dstIP, dport, full, index)
	for _, dom := range names {
		k := dnsKey{pid: info.PID, process: info.Process, domain: dom}
		a := e.dns[k]
		if a == nil {
			if len(e.dns) >= 4000 {
				continue
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
}

// lookupProc, bir 5'linin iki yonunu de deneyerek (once tam anahtar, sonra
// yalnizca port) soket→surec eslemesini bulur.
func lookupProc(proto, srcIP string, sport uint16, dstIP string, dport uint16,
	full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) (ProcInfoAlias, bool) {
	if info, ok := full[proctraffic.Key{Proto: proto, LocalIP: srcIP, LocalPort: sport, RemoteIP: dstIP, RemotePort: dport}]; ok {
		return info, true
	}
	if info, ok := full[proctraffic.Key{Proto: proto, LocalIP: dstIP, LocalPort: dport, RemoteIP: srcIP, RemotePort: sport}]; ok {
		return info, true
	}
	if info, ok := index[portKey{proto: proto, lp: sport, rp: dport}]; ok {
		return info, true
	}
	if info, ok := index[portKey{proto: proto, lp: dport, rp: sport}]; ok {
		return info, true
	}
	return ProcInfoAlias{}, false
}

// lookupDNSProc, bir DNS paketini (bir tarafi 53) surece esler. Once normal
// lookupProc; tutmazsa baglantisiz UDP soketi varsayimiyla 53-olmayan
// (efemer) tarafi YALNIZCA yerel porttan eslestirir. musl libc (Alpine/
// BusyBox) resolver'i UDP soketini connect() etmez → /proc/net/udp'de uzak
// port 0 kalir ve 4'lu esleme hicbir zaman tutmaz; glibc connect() ettigi
// icin orada lookupProc zaten yeter.
func lookupDNSProc(srcIP string, sport uint16, dstIP string, dport uint16,
	full map[proctraffic.Key]ProcInfoAlias, index map[portKey]ProcInfoAlias) (ProcInfoAlias, bool) {
	if info, ok := lookupProc("udp", srcIP, sport, dstIP, dport, full, index); ok {
		return info, true
	}
	ephem := sport
	if sport == 53 {
		ephem = dport
	}
	if info, ok := index[portKey{proto: "udp", lp: ephem, rp: 0}]; ok && (info.Process != "" || info.PID != 0) {
		return info, true
	}
	return ProcInfoAlias{}, false
}

// Deltas, son gonderimden bu yana surec bazli trafik farklarini dondurur.
func (e *pcapAttrSource) Deltas() []telemetry.ProcessTrafficSample {
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
