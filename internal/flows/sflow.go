package flows

import (
	"encoding/binary"
	"net"
	"time"
)

// sFlow v5 (RFC 3176 / sflow.org datagram v5). NetFlow'dan farklı olarak
// örnekleme tabanlıdır: cihaz her N. paketin başlığını kopyalar ve örnekleme
// oranıyla birlikte yollar. Biz "flow sample" içindeki ham paket başlığını
// (Ethernet→IP→TCP/UDP) çözüp örnekleme oranıyla ölçeklenmiş bir akış satırı
// üretiyoruz: Packets = rate, Octets = frame_length × rate (istatistiksel
// tahmin). "Counter sample" (arayüz sayaçları) şimdilik atlanır.
//
// Datagram: version(4)=5, agentAddrType(4), agentAddr(4|16), subAgentID(4),
// seqNum(4), uptimeMs(4), numSamples(4), ardından örnek kayıtları:
// sampleType(4) + sampleLen(4) + gövde.
//
//	sampleType format 1 → flow_sample, 3 → expanded_flow_sample
func ParseSFlow(payload []byte, deviceOverride string, receivedAt time.Time) []Row {
	r := &beReader{b: payload}
	if r.u32() != 5 { // yalnızca v5
		return nil
	}
	agentType := r.u32()
	var agentIP net.IP
	switch agentType {
	case 1:
		agentIP = net.IP(r.bytes(4))
	case 2:
		agentIP = net.IP(r.bytes(16))
	default:
		return nil
	}
	r.u32() // subAgentID
	r.u32() // seqNum
	r.u32() // uptimeMs
	numSamples := r.u32()
	if r.err || numSamples > 4096 {
		return nil
	}

	device := agentIP.String()
	if deviceOverride != "" {
		device = deviceOverride
	}
	ts := receivedAt.Unix()

	var rows []Row
	for i := uint32(0); i < numSamples && !r.err; i++ {
		sampleType := r.u32()
		sampleLen := r.u32()
		body := r.bytes(int(sampleLen))
		if r.err {
			break
		}
		format := sampleType & 0xFFF // enterprise (üst 20 bit) == 0 standart
		if sampleType>>12 != 0 {
			continue
		}
		switch format {
		case 1:
			rows = appendSFlowSample(rows, body, device, ts, false)
		case 3:
			rows = appendSFlowSample(rows, body, device, ts, true)
		}
	}
	return rows
}

// appendSFlowSample, tek bir flow_sample gövdesini çözer.
func appendSFlowSample(rows []Row, body []byte, device string, ts int64, expanded bool) []Row {
	s := &beReader{b: body}
	s.u32() // sequence_number
	if expanded {
		s.u32() // source_id_type
		s.u32() // source_id_index
	} else {
		s.u32() // source_id (type<<24 | index)
	}
	rate := s.u32()
	s.u32() // sample_pool
	s.u32() // drops
	if expanded {
		s.u32() // input_if_format
		s.u32() // input_if_value
		s.u32() // output_if_format
		s.u32() // output_if_value
	} else {
		s.u32() // input_if
		s.u32() // output_if
	}
	numRecords := s.u32()
	if s.err || numRecords > 256 {
		return rows
	}
	if rate == 0 {
		rate = 1
	}

	for i := uint32(0); i < numRecords && !s.err; i++ {
		recType := s.u32()
		recLen := s.u32()
		rec := s.bytes(int(recLen))
		if s.err {
			break
		}
		if recType != 1 { // yalnızca "raw packet header" (format 1)
			continue
		}
		row, ok := decodeRawPacketRecord(rec, device, ts, rate)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// decodeRawPacketRecord, "raw packet header" kaydını (header_protocol,
// frame_length, stripped, header_length, header[]) çözer ve içindeki
// Ethernet/IP çerçevesinden 5'li demeti çıkarır.
func decodeRawPacketRecord(rec []byte, device string, ts int64, rate uint32) (Row, bool) {
	h := &beReader{b: rec}
	proto := h.u32()
	frameLen := h.u32()
	h.u32() // stripped
	headerLen := h.u32()
	header := h.bytes(int(headerLen))
	if h.err || proto != 1 { // 1 = ETHERNET-ISO88023
		return Row{}, false
	}

	l3proto, l3 := parseEthernet(header)
	if l3proto != 0x0800 { // yalnızca IPv4
		return Row{}, false
	}
	ipProto, src, dst, l4, ok := parseIPv4(l3)
	if !ok {
		return Row{}, false
	}
	var sport, dport uint16
	switch ipProto {
	case 6, 17: // tcp / udp — ilk 4 bayt: srcPort(2) dstPort(2)
		if len(l4) >= 4 {
			sport = binary.BigEndian.Uint16(l4[0:2])
			dport = binary.BigEndian.Uint16(l4[2:4])
		}
	}

	return Row{
		Ts:      ts,
		Device:  device,
		Src:     src,
		Dst:     dst,
		SrcPort: sport,
		DstPort: dport,
		Proto:   protoName(byte(ipProto)),
		Packets: uint64(rate),
		Octets:  uint64(frameLen) * uint64(rate),
	}, true
}

// parseEthernet, ethertype ve L3 yükünü döndürür (802.1Q VLAN etiketi atlanır).
func parseEthernet(b []byte) (ethertype uint16, l3 []byte) {
	if len(b) < 14 {
		return 0, nil
	}
	et := binary.BigEndian.Uint16(b[12:14])
	off := 14
	for (et == 0x8100 || et == 0x88a8) && len(b) >= off+4 { // VLAN / QinQ
		et = binary.BigEndian.Uint16(b[off+2 : off+4])
		off += 4
	}
	return et, b[off:]
}

// parseIPv4, protokol numarası + kaynak/hedef IP + L4 yükünü döndürür.
func parseIPv4(b []byte) (proto byte, src, dst string, l4 []byte, ok bool) {
	if len(b) < 20 || b[0]>>4 != 4 {
		return 0, "", "", nil, false
	}
	ihl := int(b[0]&0x0F) * 4
	if ihl < 20 || len(b) < ihl {
		return 0, "", "", nil, false
	}
	proto = b[9]
	src = net.IP(b[12:16]).String()
	dst = net.IP(b[16:20]).String()
	return proto, src, dst, b[ihl:], true
}

// beReader, big-endian sıralı okuyucu — taşma güvenli (yetersiz baytta err set edilir).
type beReader struct {
	b   []byte
	off int
	err bool
}

func (r *beReader) u32() uint32 {
	if r.err || r.off+4 > len(r.b) {
		r.err = true
		return 0
	}
	v := binary.BigEndian.Uint32(r.b[r.off : r.off+4])
	r.off += 4
	return v
}

// bytes, n bayt döndürür ve ofseti 4-bayt sınırına hizalar (XDR dolgusu).
func (r *beReader) bytes(n int) []byte {
	if r.err || n < 0 || r.off+n > len(r.b) {
		r.err = true
		return nil
	}
	out := r.b[r.off : r.off+n]
	r.off += n
	if pad := r.off % 4; pad != 0 {
		r.off += 4 - pad
	}
	return out
}
