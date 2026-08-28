package capture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
)

const historySize = 120

type EndpointStat struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname,omitempty"`
	Local    bool   `json:"local"`
	Country  string `json:"country,omitempty"`
	ASN      string `json:"asn,omitempty"`
	In       uint64 `json:"in"`
	Out      uint64 `json:"out"`
	Total    uint64 `json:"total"`
	Packets  uint64 `json:"packets"`
}

type PortStat struct {
	Port    uint16 `json:"port"`
	Name    string `json:"name,omitempty"`
	Bytes   uint64 `json:"bytes"`
	Packets uint64 `json:"packets"`
}

type Bucket struct {
	Ts      int64  `json:"ts"`
	In      uint64 `json:"in"`
	Out     uint64 `json:"out"`
	Local   uint64 `json:"local"`
	Packets uint64 `json:"packets"`
}

type Snapshot struct {
	Running      bool              `json:"running"`
	Device       string            `json:"device,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    string            `json:"started_at,omitempty"`
	TotalPackets uint64            `json:"total_packets"`
	TotalBytes   uint64            `json:"total_bytes"`
	Dropped      uint64            `json:"dropped"`
	BpsIn        float64           `json:"bps_in"`
	BpsOut       float64           `json:"bps_out"`
	BpsLocal     float64           `json:"bps_local"`
	Pps          uint64            `json:"pps"`
	Protocols    map[string]uint64 `json:"protocols"`
	TopEndpoints []EndpointStat    `json:"top_endpoints"`
	TopPorts     []PortStat        `json:"top_ports"`
	TopDomains   []DomainStat      `json:"top_domains"`
	History      []Bucket          `json:"history"`
	LocalIPCount int               `json:"local_ip_count"`
}

type endpointAgg struct {
	IP      string
	Local   bool
	In      uint64
	Out     uint64
	Packets uint64
}

type portAgg struct {
	Bytes   uint64
	Packets uint64
}

type Engine struct {
	mu sync.Mutex

	// stopMu, Start/Stop cagrilarini serilestirir; Stop islemi (dongu cikisi +
	// handle kapanisi) sirasinda yeni Start veya ikinci Stop giris yapamaz.
	stopMu sync.Mutex

	handle    *pcap.Handle
	device    string
	startedAt time.Time
	stopCh    chan struct{}
	doneCh    chan struct{}
	stopped   atomicFlag
	runErr    atomicStr

	totalPackets uint64
	totalBytes   uint64
	dropped      uint64

	protocols map[string]uint64
	endpoints map[string]*endpointAgg
	ports     map[uint16]*portAgg

	dnsCounts map[string]*dnsAgg
	domainIPs map[string]map[string]struct{}

	running bool // mu altinda korunur

	// kayit (pcap yazimi) durumu; recMu ile korunur
	recMu       sync.Mutex
	recDir      string
	recMaxBytes uint64
	recFile     *os.File
	recWriter   *pcapgo.Writer
	recName     string
	recDevice   string
	recPackets  uint64
	recBytes    uint64
	recErr      string
	linkType    layers.LinkType

	history []Bucket

	localMu   sync.RWMutex
	localNets []*net.IPNet

	res *resolver
}

func NewEngine() *Engine {
	e := &Engine{
		protocols:   map[string]uint64{},
		endpoints:   map[string]*endpointAgg{},
		ports:       map[uint16]*portAgg{},
		dnsCounts:   map[string]*dnsAgg{},
		domainIPs:   map[string]map[string]struct{}{},
		history:     make([]Bucket, 0, historySize),
		res:         newResolver(),
		recDir:      "captures",
		recMaxBytes: 100 << 20,
	}
	e.refreshLocalNets()
	return e
}

func (e *Engine) Start(device string) error {
	e.stopMu.Lock()
	defer e.stopMu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("capture already running on %s", e.device)
	}

	inactive, err := pcap.NewInactiveHandle(device)
	if err != nil {
		return fmt.Errorf("open %s: %w", device, err)
	}
	defer inactive.CleanUp()
	if err := inactive.SetSnapLen(65535); err != nil {
		return fmt.Errorf("snaplen: %w", err)
	}
	if err := inactive.SetPromisc(false); err != nil {
		return fmt.Errorf("promisc: %w", err)
	}
	if err := inactive.SetTimeout(time.Second); err != nil {
		return fmt.Errorf("timeout: %w", err)
	}
	handle, err := inactive.Activate()
	if err != nil {
		return fmt.Errorf("activate %s: %w (root/admin yetkisi gerekli olabilir)", device, err)
	}

	e.handle = handle
	e.device = device
	e.running = true
	e.startedAt = time.Now()
	e.runErr.store("")
	e.totalPackets = 0
	e.totalBytes = 0
	e.dropped = 0
	e.protocols = map[string]uint64{}
	e.endpoints = map[string]*endpointAgg{}
	e.ports = map[uint16]*portAgg{}
	e.dnsCounts = map[string]*dnsAgg{}
	e.domainIPs = map[string]map[string]struct{}{}
	e.history = e.history[:0]
	e.linkType = handle.LinkType()
	e.refreshLocalNets()

	e.stopped.store(false)
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})
	go e.loop()
	return nil
}

func (e *Engine) Stop() {
	// Start/Stop'u serilestir: cift stop -> cift close(stopCh) panigi ve
	// kapali handle uzerine Stats() onlenir.
	e.stopMu.Lock()
	defer e.stopMu.Unlock()

	e.mu.Lock()
	if e.handle == nil {
		e.mu.Unlock()
		return
	}
	handle, stopCh, doneCh := e.handle, e.stopCh, e.doneCh
	e.stopped.store(true) // okuma dongusundeki hata yollarina "dur" sinyali
	e.mu.Unlock()

	close(stopCh)
	<-doneCh

	// dongu bitti; once istatistik al, sonra kaydi kapat, sonra handle kapat.
	e.recMu.Lock()
	if e.recFile != nil {
		e.closeRecLocked()
	}
	e.recMu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if stats, err := handle.Stats(); err == nil {
		e.dropped += uint64(stats.PacketsDropped)
	}
	handle.Close()
	e.handle = nil
	e.running = false
}

func (e *Engine) loop() {
	defer close(e.doneCh)

	e.mu.Lock()
	linkType := e.handle.LinkType()
	handle := e.handle
	e.mu.Unlock()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	localTicker := time.NewTicker(15 * time.Second)
	defer localTicker.Stop()

	consecErr := 0
	for {
		select {
		case <-e.stopCh:
			return
		case <-localTicker.C:
			e.refreshLocalNets()
		case <-ticker.C:
			e.rollBucket()
		default:
		}

		// PacketSource KULLANMA: o kendi gizli goroutine'inde ReadPacketData
		// yapar ve Close() ile use-after-free segfault uretir. Okumayi bu
		// goroutine'de yapiyoruz; Stop() doneCh'i bekledigi icin Close aninda
		// handle'a dokunan kimse kalmaz.
		data, ci, err := handle.ReadPacketData()
		if err != nil {
			if e.stopped.load() {
				return
			}
			if isTimeoutErr(err) {
				consecErr = 0
				continue
			}
			consecErr++
			if consecErr > 128 {
				e.runErr.store(err.Error())
				// olcu dongusu coktu: handle'i da kapat, motoru bos durumda birak
				e.stopped.store(true)
				e.mu.Lock()
				if h := e.handle; h != nil {
					h.Close()
					e.handle = nil
				}
				e.running = false
				e.mu.Unlock()
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		consecErr = 0
		if len(data) == 0 {
			continue
		}
		e.recordPacket(data, ci)
		pkt := gopacket.NewPacket(data, linkType, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
		pkt.Metadata().CaptureInfo = ci
		e.process(pkt)
	}
}

// isTimeoutErr, pcap okuma zaman asimlarini (normal bos durum) ayirt eder.
func isTimeoutErr(err error) bool {
	if ne, ok := err.(pcap.NextError); ok {
		return ne == pcap.NextErrorTimeoutExpired
	}
	if errors.Is(err, pcap.NextErrorTimeoutExpired) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out")
}

func (e *Engine) process(pkt gopacket.Packet) {
	nl := pkt.NetworkLayer()
	if nl == nil {
		return
	}
	var srcIP, dstIP net.IP
	switch l := nl.(type) {
	case *layers.IPv4:
		srcIP, dstIP = l.SrcIP, l.DstIP
	case *layers.IPv6:
		srcIP, dstIP = l.SrcIP, l.DstIP
	default:
		return
	}

	length := uint64(pkt.Metadata().Length)
	if length == 0 {
		length = uint64(pkt.Metadata().CaptureLength)
	}

	proto := "other"
	var sport, dport uint16
	var dns *dnsParsed
	switch {
	case pkt.Layer(layers.LayerTypeICMPv4) != nil:
		proto = "ICMP"
	case pkt.Layer(layers.LayerTypeICMPv6) != nil:
		proto = "ICMPv6"
	}
	if tl := pkt.TransportLayer(); tl != nil {
		switch t := tl.(type) {
		case *layers.TCP:
			proto = "TCP"
			sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
		case *layers.UDP:
			proto = "UDP"
			sport, dport = uint16(t.SrcPort), uint16(t.DstPort)
			if dport == 53 || sport == 53 {
				dns = parseDNS(t.Payload)
			}
		default:
			proto = tl.LayerType().String()
		}
	} else if proto == "other" {
		proto = nl.LayerType().String()
	}

	srcLocal := e.isLocal(srcIP)
	dstLocal := e.isLocal(dstIP)

	e.mu.Lock()
	defer e.mu.Unlock()

	if dns != nil {
		e.applyDNS(dns)
	}
	e.totalPackets++
	e.totalBytes += length
	e.protocols[proto]++

	if p := e.endpoints[srcIP.String()]; p != nil {
		p.Packets++
		if srcLocal {
			p.Local = true
			p.Out += length
		} else {
			p.In += length
		}
	} else {
		p = &endpointAgg{IP: srcIP.String(), Local: srcLocal, Packets: 1}
		if srcLocal {
			p.Out = length
		} else {
			p.In = length
		}
		e.endpoints[p.IP] = p
	}
	if p := e.endpoints[dstIP.String()]; p != nil {
		if dstLocal {
			p.Local = true
			p.In += length
		} else {
			p.Out += length
		}
	} else {
		p = &endpointAgg{IP: dstIP.String(), Local: dstLocal}
		if dstLocal {
			p.In = length
		} else {
			p.Out = length
		}
		e.endpoints[p.IP] = p
	}

	if proto == "TCP" || proto == "UDP" {
		pa := e.ports[sport]
		if pa == nil {
			pa = &portAgg{}
			e.ports[sport] = pa
		}
		pa.Bytes += length
		pa.Packets++
		pa = e.ports[dport]
		if pa == nil {
			pa = &portAgg{}
			e.ports[dport] = pa
		}
		pa.Bytes += length
		pa.Packets++
	}

	// son saniye kovasina ekle
	if n := len(e.history); n > 0 {
		b := &e.history[n-1]
		switch {
		case srcLocal && dstLocal:
			b.Local += length
		case srcLocal:
			b.Out += length
		case dstLocal:
			b.In += length
		default:
			b.Local += length
		}
		b.Packets++
	}
}

func (e *Engine) rollBucket() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.history = append(e.history, Bucket{Ts: time.Now().Unix()})
	if len(e.history) > historySize {
		e.history = e.history[len(e.history)-historySize:]
	}
}

func (e *Engine) refreshLocalNets() {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	var nets []*net.IPNet
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				nets = append(nets, ipnet)
			}
		}
	}
	e.localMu.Lock()
	e.localNets = nets
	e.localMu.Unlock()
}

func (e *Engine) isLocal(ip net.IP) bool {
	e.localMu.RLock()
	defer e.localMu.RUnlock()
	for _, n := range e.localNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (e *Engine) Snapshot() *Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := &Snapshot{
		Running:      e.running,
		Device:       e.device,
		Error:        e.runErr.load(),
		StartedAt:    e.startedAt.Format(time.RFC3339),
		TotalPackets: e.totalPackets,
		TotalBytes:   e.totalBytes,
		Dropped:      e.dropped,
		Protocols:    map[string]uint64{},
		TopEndpoints: make([]EndpointStat, 0, 64),
		TopPorts:     make([]PortStat, 0, 32),
		TopDomains:   []DomainStat{},
		History:      make([]Bucket, len(e.history)),
	}
	copy(s.History, e.history)
	for k, v := range e.protocols {
		s.Protocols[k] = v
	}
	if s.Running {
		if stats, err := e.handle.Stats(); err == nil {
			s.Dropped = e.dropped + uint64(stats.PacketsDropped)
		}
	}

	type epTotal struct {
		agg   *endpointAgg
		total uint64
	}
	list := make([]epTotal, 0, len(e.endpoints))
	for _, v := range e.endpoints {
		t := v.In + v.Out
		list = append(list, epTotal{v, t})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].total > list[j].total })
	for i, et := range list {
		if i >= 50 {
			break
		}
		hs := e.res.Lookup(et.agg.IP)
		if hs == "" {
			e.res.Queue(et.agg.IP)
		}
		s.TopEndpoints = append(s.TopEndpoints, EndpointStat{
			IP:       et.agg.IP,
			Hostname: hs,
			Local:    et.agg.Local,
			In:       et.agg.In,
			Out:      et.agg.Out,
			Total:    et.total,
			Packets:  et.agg.Packets,
		})
	}

	type ptTotal struct {
		port uint16
		agg  *portAgg
	}
	plist := make([]ptTotal, 0, len(e.ports))
	for p, v := range e.ports {
		plist = append(plist, ptTotal{p, v})
	}
	sort.Slice(plist, func(i, j int) bool { return plist[i].agg.Bytes > plist[j].agg.Bytes })
	for i, pt := range plist {
		if i >= 25 {
			break
		}
		s.TopPorts = append(s.TopPorts, PortStat{
			Port:    pt.port,
			Name:    portName(pt.port),
			Bytes:   pt.agg.Bytes,
			Packets: pt.agg.Packets,
		})
	}

	s.TopDomains = e.topDomainStats()

	if n := len(s.History); n > 0 {
		last := s.History[n-1]
		s.BpsIn = float64(last.In) * 8
		s.BpsOut = float64(last.Out) * 8
		s.BpsLocal = float64(last.Local) * 8
		s.Pps = last.Packets
	}
	e.localMu.RLock()
	s.LocalIPCount = len(e.localNets)
	e.localMu.RUnlock()
	return s
}

func portName(p uint16) string {
	tcp := layers.TCPPort(p)
	if n := tcp.String(); n != "" && !isNumericName(n, p) {
		return n
	}
	udp := layers.UDPPort(p)
	if n := udp.String(); n != "" && !isNumericName(n, p) {
		return n
	}
	return ""
}

func isNumericName(s string, p uint16) bool {
	return s == "" || s == strconv.Itoa(int(p))
}

type atomicFlag struct {
	mu sync.Mutex
	v  bool
}

func (a *atomicFlag) load() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

func (a *atomicFlag) store(v bool) {
	a.mu.Lock()
	a.v = v
	a.mu.Unlock()
}

type atomicStr struct {
	mu sync.Mutex
	v  string
}

func (a *atomicStr) load() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

func (a *atomicStr) store(v string) {
	a.mu.Lock()
	a.v = v
	a.mu.Unlock()
}

type resolver struct {
	mu    sync.RWMutex
	cache map[string]string
	queue chan string
}

func newResolver() *resolver {
	r := &resolver{
		cache: map[string]string{},
		queue: make(chan string, 128),
	}
	go r.worker()
	return r
}

func (r *resolver) worker() {
	for ip := range r.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		names, err := net.DefaultResolver.LookupAddr(ctx, ip)
		cancel()
		if err == nil && len(names) > 0 {
			name := names[0]
			if len(name) > 1 && name[len(name)-1] == '.' {
				name = name[:len(name)-1]
			}
			r.mu.Lock()
			r.cache[ip] = name
			r.mu.Unlock()
		}
	}
}

func (r *resolver) Lookup(ip string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache[ip]
}

func (r *resolver) Queue(ip string) {
	select {
	case r.queue <- ip:
	default:
	}
}
