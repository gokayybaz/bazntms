// Package flows, ag cihazlarindan UDP ile gelen akis kayitlarini toplar:
// NetFlow v5 (sabit format), NetFlow v9 ve IPFIX/v10 (sablon tabanli — bkz.
// template.go / nfv9.go / ipfix.go) ve sFlow v5 (ornekleme tabanli — bkz.
// sflow.go). Ucu de ayni Collector uzerinden, gerekirse ayni portta.
package flows

import (
	"encoding/binary"
	"net"
	"time"
)

const (
	v5HeaderSize = 24
	v5RecordSize = 48
)

type Collector struct {
	Conn *net.UDPConn
	// ExporterIP boş değilse tüm akışlar bu IP'ye atfedilir; paketin kaynak
	// IP'si yok sayılır. Hub bir NAT/röle arkasındayken (ör. Docker Desktop
	// UDP iletimi) gerçek exporter IP'si kaybolur — tek exporter'lı kurulumlarda
	// cihaz eşleştirmesini korumak için kullanılır.
	ExporterIP string
	OnFlows    func(device string, rows []Row)

	templates *TemplateCache // v9/IPFIX sablon onbellegi (Listen'de kurulur)
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

// Listen, UDP dinleyicisini baslatir; her paket icin OnFlows cagrılır.
func (c *Collector) Listen(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	c.Conn = conn
	if c.templates == nil {
		c.templates = NewTemplateCache()
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			exporterKey := peer.IP.String()
			device := exporterKey
			if c.ExporterIP != "" {
				device = c.ExporterIP
			}
			rows := c.parse(buf[:n], device, exporterKey, time.Now())
			if len(rows) > 0 && c.OnFlows != nil {
				c.OnFlows(device, rows)
			}
		}
	}()
	return nil
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

func (c *Collector) Close() {
	if c.Conn != nil {
		_ = c.Conn.Close()
	}
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
