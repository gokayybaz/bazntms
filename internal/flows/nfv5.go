// Package flows, ag cihazlarindan UDP ile gelen akis kayitlarini toplar:
// NetFlow v5 (sabit format), NetFlow v9 ve IPFIX/v10 (sablon tabanli — bkz.
// template.go / nfv9.go / ipfix.go) ve sFlow v5 (ornekleme tabanli — bkz.
// sflow.go). Ucu de ayni Collector uzerinden, gerekirse ayni portta.
package flows

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/metrics"
)

const (
	v5HeaderSize = 24
	v5RecordSize = 48

	flowRcvBuf     = 8 << 20 // SO_RCVBUF: burst'te çekirdek kuyruğu (best-effort)
	flowQueueDepth = 4096    // reader → worker kanal derinliği. 200k flow/sn
	//                          patlamasında 1024 taşıyordu (S21.13 fazla küçüktü);
	//                          her *pkt 64KB → ~256MB worst-case, GOMEMLIMIT + GC
	//                          pool temizliği sınırlar.
	flowDefWorkers  = 6
	flowMaxDatagram = 65535
)

type Collector struct {
	Conn *net.UDPConn
	// ExporterIP boş değilse tüm akışlar bu IP'ye atfedilir; paketin kaynak
	// IP'si yok sayılır. Hub bir NAT/röle arkasındayken (ör. Docker Desktop
	// UDP iletimi) gerçek exporter IP'si kaybolur — tek exporter'lı kurulumlarda
	// cihaz eşleştirmesini korumak için kullanılır.
	ExporterIP string
	OnFlows    func(device string, rows []Row)
	// Workers, ayrıştırma + OnFlows worker sayısı (0 → 4). Reader goroutine
	// yalnızca ReadFromUDP yapar (S21.9 — 50k flow/sn'de sync OnFlows çağrısı
	// reader'ı bloklayıp çekirdek buffer'ını taşırıyordu).
	Workers int

	templates *TemplateCache // v9/IPFIX sablon onbellegi (Listen'de kurulur)

	packets   chan *pkt
	pool      sync.Pool
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// pkt, reader'dan worker'a taşınan ham datagram (havuzlanır).
type pkt struct {
	buf  [flowMaxDatagram]byte
	n    int
	peer netip.Addr
}

type Row struct {
	Ts      int64
	Device  string
	Src     string
	Dst     string
	SrcPort uint16
	DstPort uint16
	Proto   string
	Packets uint64
	Octets  uint64
}

// Listen, UDP dinleyicisini baslatir: bir reader goroutine datagram okur,
// N worker ayrıştırıp OnFlows çağırır.
func (c *Collector) Listen(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	_ = conn.SetReadBuffer(flowRcvBuf) // best-effort; OS sınırlayabilir
	c.Conn = conn
	if c.templates == nil {
		c.templates = NewTemplateCache()
	}
	c.pool.New = func() any { return new(pkt) }
	c.packets = make(chan *pkt, flowQueueDepth)

	workers := c.Workers
	if workers <= 0 {
		workers = flowDefWorkers
	}
	for i := 0; i < workers; i++ {
		c.wg.Add(1)
		go c.worker()
	}
	c.wg.Add(1)
	go c.reader()
	return nil
}

// reader, yalnızca datagram okur ve worker kanalına verir. Kanal doluysa
// datagram düşürülür (reader bloklanırsa çekirdek UDP buffer'ı taşar — bu
// katmanda görünmez; burada sayılır).
func (c *Collector) reader() {
	defer c.wg.Done()
	defer close(c.packets)
	for {
		p := c.pool.Get().(*pkt)
		n, ap, err := c.Conn.ReadFromUDPAddrPort(p.buf[:])
		if err != nil {
			c.pool.Put(p)
			return // conn kapandı
		}
		p.n = n
		p.peer = ap.Addr()
		select {
		case c.packets <- p:
		default:
			c.pool.Put(p)
			metrics.IncFlowsDropped("queue_full")
		}
	}
}

func (c *Collector) worker() {
	defer c.wg.Done()
	for p := range c.packets {
		c.process(p.buf[:p.n], p.peer)
		c.pool.Put(p)
	}
}

func (c *Collector) process(data []byte, peer netip.Addr) {
	exporterKey := peer.Unmap().String()
	device := exporterKey
	if c.ExporterIP != "" {
		device = c.ExporterIP
	}
	kind := datagramKind(data)
	rows := c.parse(data, device, exporterKey, time.Now())
	switch {
	case len(rows) > 0:
		metrics.AddFlowsReceived(kind, len(rows))
		if c.OnFlows != nil {
			c.OnFlows(device, rows)
		}
	case kind == "unknown":
		metrics.IncFlowsDropped("unknown_version")
	case len(data) < 4:
		metrics.IncFlowsDropped("short")
	default:
		// bilinen protokol ama 0 satır: şablon henüz gelmedi, bozuk kayıt
		// veya yalnız-sayaç sFlow örneği
		metrics.IncFlowsDropped("empty_parse")
	}
}

// datagramKind, ham datagramı protokol etiketine sınıflandırır (metrik için;
// parse ile aynı ayrım mantığı).
func datagramKind(payload []byte) string {
	if len(payload) < 4 {
		return "short"
	}
	if binary.BigEndian.Uint32(payload[0:4]) == 5 {
		return "sflow"
	}
	switch binary.BigEndian.Uint16(payload[0:2]) {
	case 5:
		return "v5"
	case 9:
		return "v9"
	case 10:
		return "ipfix"
	}
	return "unknown"
}

// parse, paket versiyonuna gore uygun cozucuye yonlendirir. sFlow v5 ile
// NetFlow ayni portta karisik gelebilir: sFlow datagrami 4 baytlik version
// alaniyla baslar (== 5), NetFlow v5/v9/IPFIX 2 baytlik version + count ile
// (uint32 olarak okununca daima >= 0x50000) — cakisma yok.
func (c *Collector) parse(payload []byte, device, exporterKey string, receivedAt time.Time) []Row {
	if len(payload) < 4 {
		return nil
	}
	if binary.BigEndian.Uint32(payload[0:4]) == 5 {
		return ParseSFlow(payload, c.ExporterIP, receivedAt)
	}
	switch binary.BigEndian.Uint16(payload[0:2]) {
	case 5:
		return ParseV5(payload, device, receivedAt)
	case 9:
		return ParseV9(c.templates, payload, device, exporterKey, receivedAt)
	case 10:
		return ParseIPFIX(c.templates, payload, device, exporterKey, receivedAt)
	}
	return nil
}

// Close, reader'ı (conn kapatarak) ve worker'ları durdurur; hepsi çıkana
// kadar bekler.
func (c *Collector) Close() {
	c.closeOnce.Do(func() {
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
		c.wg.Wait()
	})
}

// ParseV5, NetFlow v5 paketini cozer (test edilebilir saf fonksiyon).
func ParseV5(payload []byte, device string, receivedAt time.Time) []Row {
	if len(payload) < v5HeaderSize {
		return nil
	}
	version := binary.BigEndian.Uint16(payload[0:2])
	if version != 5 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(payload[2:4]))
	sysUptimeMs := binary.BigEndian.Uint32(payload[4:8]) // cihaz açılışından beri geçen süre (ms)
	unixSecs := binary.BigEndian.Uint32(payload[8:12])

	base := time.Unix(int64(unixSecs), 0)
	if unixSecs == 0 {
		base = receivedAt
	}

	rows := make([]Row, 0, count)
	for i := 0; i < count; i++ {
		off := v5HeaderSize + i*v5RecordSize
		if off+v5RecordSize > len(payload) {
			break
		}
		rec := payload[off : off+v5RecordSize]
		src := net.IP(rec[0:4]).String()
		dst := net.IP(rec[4:8]).String()
		packets := binary.BigEndian.Uint32(rec[16:20])
		octets := binary.BigEndian.Uint32(rec[20:24])
		lastMs := binary.BigEndian.Uint32(rec[28:32]) // akışın son paketi: SysUptime anı (ms)
		srcPort := binary.BigEndian.Uint16(rec[32:34])
		dstPort := binary.BigEndian.Uint16(rec[34:36])
		protoNum := rec[38]

		// akış sonu zamanı = header unix zamanı - (SysUptime - Last). Header'daki
		// SysUptime ve kayıttaki Last MİLİSANİYE cinsindendir; ikisi de ms olarak
		// işlenmezse zaman damgası günlerce/aylarca kayar.
		deltaMs := int64(sysUptimeMs) - int64(lastMs)
		if deltaMs < 0 {
			deltaMs = 0
		}
		ts := base.Add(-time.Duration(deltaMs) * time.Millisecond)
		if ts.After(receivedAt) || ts.Before(receivedAt.Add(-7*24*time.Hour)) {
			ts = receivedAt // saçma değer (saat kayması, uptime sıfırlanması) → alım zamanı
		}

		rows = append(rows, Row{
			Ts:      ts.Unix(),
			Device:  device,
			Src:     src,
			Dst:     dst,
			SrcPort: srcPort,
			DstPort: dstPort,
			Proto:   protoName(protoNum),
			Packets: uint64(packets),
			Octets:  uint64(octets),
		})
	}
	return rows
}

func protoName(n byte) string {
	switch n {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	default:
		return "proto-" + itoa(int(n))
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}
