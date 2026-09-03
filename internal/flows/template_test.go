package flows

import (
	"encoding/binary"
	"testing"
	"time"
)

func be16(v uint16) []byte { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); return b }
func be32(v uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }

// v9 template flowset govdesi: templateID + fieldCount + [type,len]...
func v9Template(id uint16, fields [][2]uint16) []byte {
	b := append(be16(id), be16(uint16(len(fields)))...)
	for _, f := range fields {
		b = append(b, be16(f[0])...)
		b = append(b, be16(f[1])...)
	}
	return b
}

func flowset(id uint16, body []byte) []byte {
	total := uint16(4 + len(body))
	return append(append(be16(id), be16(total)...), body...)
}

var testFields = [][2]uint16{
	{ieSourceIPv4Address, 4}, {ieDestinationIPv4Address, 4},
	{ieSourceTransportPort, 2}, {ieDestinationTransport, 2},
	{ieProtocolIdentifier, 1}, {ieOctetDeltaCount, 4}, {iePacketDeltaCount, 4},
}

func testRecord() []byte {
	r := []byte{10, 0, 0, 5, 8, 8, 8, 8} // src 10.0.0.5, dst 8.8.8.8
	r = append(r, be16(40000)...)        // src port
	r = append(r, be16(53)...)           // dst port
	r = append(r, 17)                    // udp
	r = append(r, be32(1500)...)         // octets
	r = append(r, be32(3)...)            // packets
	return r
}

func TestParseV9TemplateThenData(t *testing.T) {
	cache := NewTemplateCache()
	now := time.Now()

	hdr := func(count uint16) []byte {
		h := make([]byte, 20)
		binary.BigEndian.PutUint16(h[0:2], 9)
		binary.BigEndian.PutUint16(h[2:4], count)
		binary.BigEndian.PutUint32(h[8:12], uint32(now.Unix()))
		binary.BigEndian.PutUint32(h[16:20], 1) // sourceID
		return h
	}

	// 1) once yalniz template — veri yok
	tmplPkt := append(hdr(1), flowset(0, v9Template(256, testFields))...)
	if rows := ParseV9(cache, tmplPkt, "10.0.0.1", "10.0.0.1", now); len(rows) != 0 {
		t.Fatalf("template paketinden akis cikmamali: %d", len(rows))
	}

	// 2) data flowset — sablon artik cache'te
	dataPkt := append(hdr(1), flowset(256, append(testRecord(), testRecord()...))...)
	rows := ParseV9(cache, dataPkt, "rtr-1", "10.0.0.1", now)
	if len(rows) != 2 {
		t.Fatalf("2 kayit beklenirdi: %d", len(rows))
	}
	r := rows[0]
	if r.Src != "10.0.0.5" || r.Dst != "8.8.8.8" || r.SrcPort != 40000 || r.DstPort != 53 ||
		r.Proto != "udp" || r.Octets != 1500 || r.Packets != 3 || r.Device != "rtr-1" {
		t.Fatalf("kayit hatali: %+v", r)
	}

	// 3) sablon bilinmeyen exporter'dan gelirse cozulemez
	if rows := ParseV9(cache, dataPkt, "x", "9.9.9.9", now); len(rows) != 0 {
		t.Fatalf("baska exporter'in sablonu kullanilmamali: %d", len(rows))
	}
}

func TestParseIPFIXTemplateThenData(t *testing.T) {
	cache := NewTemplateCache()
	now := time.Now()

	build := func(sets ...[]byte) []byte {
		body := []byte{}
		for _, s := range sets {
			body = append(body, s...)
		}
		h := make([]byte, 16)
		binary.BigEndian.PutUint16(h[0:2], 10)
		binary.BigEndian.PutUint16(h[2:4], uint16(16+len(body))) // toplam uzunluk
		binary.BigEndian.PutUint32(h[4:8], uint32(now.Unix()))
		binary.BigEndian.PutUint32(h[12:16], 7) // obsDomainID
		return append(h, body...)
	}

	tmplSet := flowset(2, v9Template(300, testFields))
	dataSet := flowset(300, testRecord())

	// tek pakette template + data (IPFIX'te yaygin)
	rows := ParseIPFIX(cache, build(tmplSet, dataSet), "fw-1", "192.168.0.1", now)
	if len(rows) != 1 {
		t.Fatalf("1 kayit beklenirdi: %d", len(rows))
	}
	if rows[0].Dst != "8.8.8.8" || rows[0].Proto != "udp" || rows[0].Octets != 1500 {
		t.Fatalf("IPFIX kayit hatali: %+v", rows[0])
	}

	// sonraki pakette yalniz data — sablon hatirlanir
	rows = ParseIPFIX(cache, build(dataSet), "fw-1", "192.168.0.1", now)
	if len(rows) != 1 {
		t.Fatalf("sablon hatirlanmadi: %d", len(rows))
	}
}

func TestParseV9EnterpriseFieldSkipped(t *testing.T) {
	// IPFIX enterprise-bit'li alan: type|0x8000 + 4 bayt enterprise number
	cache := NewTemplateCache()
	now := time.Now()
	fields := [][2]uint16{
		{ieSourceIPv4Address, 4}, {ieDestinationIPv4Address, 4},
		{0x8000 | 999, 4}, // enterprise alan — parseTemplate atlar ama uzunluk kayda dahil
		{ieProtocolIdentifier, 1},
	}
	tmplBody := be16(500)
	tmplBody = append(tmplBody, be16(uint16(len(fields)))...)
	for _, f := range fields {
		tmplBody = append(tmplBody, be16(f[0])...)
		tmplBody = append(tmplBody, be16(f[1])...)
		if f[0]&0x8000 != 0 {
			tmplBody = append(tmplBody, be32(12345)...) // enterprise number
		}
	}
	h := make([]byte, 16)
	binary.BigEndian.PutUint16(h[0:2], 10)
	binary.BigEndian.PutUint32(h[4:8], uint32(now.Unix()))
	binary.BigEndian.PutUint32(h[12:16], 1)
	rec := []byte{1, 1, 1, 1, 2, 2, 2, 2, 0xAA, 0xBB, 0xCC, 0xDD, 6} // src,dst,enterprise(4),proto
	body := append(flowset(2, tmplBody), flowset(500, rec)...)
	binary.BigEndian.PutUint16(h[2:4], uint16(16+len(body)))
	rows := ParseIPFIX(cache, append(h, body...), "d", "e", now)
	if len(rows) != 1 || rows[0].Src != "1.1.1.1" || rows[0].Dst != "2.2.2.2" || rows[0].Proto != "tcp" {
		t.Fatalf("enterprise alanli sablon cozulemedi: %+v", rows)
	}
}
