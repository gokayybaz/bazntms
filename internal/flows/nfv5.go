// Package flows, NetFlow v5 toplar. v9/IPFIX ve sFlow ileri fazda
// goflow2 ile bu pakete eklenecektir.
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
	Conn    *net.UDPConn
	OnFlows func(device string, rows []Row)
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
	go func() {
		buf := make([]byte, 65535)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			device := peer.IP.String()
			rows := ParseV5(buf[:n], device, time.Now())
			if len(rows) > 0 && c.OnFlows != nil {
				c.OnFlows(device, rows)
			}
		}
	}()
	return nil
}

func (c *Collector) Close() {
	if c.Conn != nil {
		c.Conn.Close()
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
	uptimeSecs := binary.BigEndian.Uint32(payload[4:8])
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
		firstUptime := binary.BigEndian.Uint32(rec[24:28])
		lastUptime := binary.BigEndian.Uint32(rec[28:32])
		srcPort := binary.BigEndian.Uint16(rec[32:34])
		dstPort := binary.BigEndian.Uint16(rec[34:36])
		protoNum := rec[38]

		// akis zamani: uptime tabanli, header unix zamanina baglanir
		ts := base.Add(-time.Duration(uptimeSecs)*time.Second + time.Duration(lastUptime)*time.Millisecond)
		if ts.After(receivedAt) {
			ts = receivedAt
		}
		_ = firstUptime

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
