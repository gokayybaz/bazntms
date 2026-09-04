package flows

import (
	"encoding/binary"
	"testing"
	"time"
)

// be32 / be16 → template_test.go'da tanımlı (aynı paket).

// pad4, XDR 4-bayt hizalama dolgusu ekler.
func pad4(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// buildEthIPv4TCP, sFlow "raw packet header" için örnek çerçeve üretir.
func buildEthIPv4TCP(srcIP, dstIP [4]byte, sport, dport uint16, proto byte) []byte {
	eth := []byte{
		0xaa, 0xbb, 0xcc, 0x00, 0x00, 0x01, // dst MAC
		0xaa, 0xbb, 0xcc, 0x00, 0x00, 0x02, // src MAC
		0x08, 0x00, // ethertype IPv4
	}
	ip := make([]byte, 20)
	ip[0] = 0x45 // v4, IHL=5
	binary.BigEndian.PutUint16(ip[2:4], 1500)
	ip[8] = 64 // TTL
	ip[9] = proto
	copy(ip[12:16], srcIP[:])
	copy(ip[16:20], dstIP[:])
	l4 := make([]byte, 20)
	binary.BigEndian.PutUint16(l4[0:2], sport)
	binary.BigEndian.PutUint16(l4[2:4], dport)
	return append(append(eth, ip...), l4...)
}

func rawPacketRecord(frame []byte, frameLen uint32) []byte {
	body := []byte{}
	body = append(body, be32(1)...)        // header_protocol = ethernet
	body = append(body, be32(frameLen)...) // frame_length (örneklenmemiş orijinal)
	body = append(body, be32(0)...)        // stripped
	body = append(body, be32(uint32(len(frame)))...)
	body = append(body, frame...)
	unpadded := len(body)

	rec := []byte{}
	rec = append(rec, be32(1)...) // record type = raw packet header
	rec = append(rec, be32(uint32(unpadded))...)
	rec = append(rec, body...)
	return pad4(rec)
}

func flowSample(rate uint32, records []byte, numRecords uint32, expanded bool) []byte {
	body := []byte{}
	add := func(vs ...uint32) {
		for _, v := range vs {
			body = append(body, be32(v)...)
		}
	}
	add(1) // sequence_number
	if expanded {
		add(0, 3) // source_id_type, source_id_index
	} else {
		add(0) // source_id
	}
	add(rate)
	add(0) // sample_pool
	add(0) // drops
	if expanded {
		add(0, 1, 0, 2) // input_if_format/value, output_if_format/value
	} else {
		add(1, 2) // input_if, output_if
	}
	add(numRecords)
	body = append(body, records...)

	format := uint32(1)
	if expanded {
		format = 3
	}
	out := []byte{}
	out = append(out, be32(format)...)
	out = append(out, be32(uint32(len(body)))...)
	out = append(out, body...)
	return out
}

func sflowDatagram(samples []byte, numSamples uint32) []byte {
	d := []byte{}
	d = append(d, be32(5)...)             // version
	d = append(d, be32(1)...)             // agent addr type IPv4
	d = append(d, []byte{10, 0, 0, 1}...) // agent addr
	d = append(d, be32(0)...)             // sub agent id
	d = append(d, be32(7)...)             // seq
	d = append(d, be32(123456)...)        // uptime ms
	d = append(d, be32(numSamples)...)    // num samples
	d = append(d, samples...)
	return d
}

func TestParseSFlowFlowSample(t *testing.T) {
	frame := buildEthIPv4TCP([4]byte{192, 168, 1, 10}, [4]byte{93, 184, 216, 34}, 51000, 443, 6)
	rec := rawPacketRecord(frame, 1500)
	dg := sflowDatagram(flowSample(512, rec, 1, false), 1)

	rows := ParseSFlow(dg, "", time.Now())
	if len(rows) != 1 {
		t.Fatalf("1 satır bekleniyordu, %d geldi", len(rows))
	}
	r := rows[0]
	if r.Device != "10.0.0.1" {
		t.Errorf("device: %q (agent IP bekleniyordu)", r.Device)
	}
	if r.Src != "192.168.1.10" || r.Dst != "93.184.216.34" {
		t.Errorf("IP çifti: %s → %s", r.Src, r.Dst)
	}
	if r.SrcPort != 51000 || r.DstPort != 443 || r.Proto != "tcp" {
		t.Errorf("L4: %d→%d %s", r.SrcPort, r.DstPort, r.Proto)
	}
	if r.Packets != 512 { // rate
		t.Errorf("Packets = %d, 512 (rate) bekleniyordu", r.Packets)
	}
	if r.Octets != 1500*512 { // frame_length × rate
		t.Errorf("Octets = %d, %d bekleniyordu", r.Octets, 1500*512)
	}
}

func TestParseSFlowExpandedAndOverride(t *testing.T) {
	frame := buildEthIPv4TCP([4]byte{10, 1, 1, 1}, [4]byte{8, 8, 8, 8}, 33333, 53, 17)
	rec := rawPacketRecord(frame, 90)
	dg := sflowDatagram(flowSample(1000, rec, 1, true), 1)

	rows := ParseSFlow(dg, "203.0.113.5", time.Now())
	if len(rows) != 1 {
		t.Fatalf("1 satır bekleniyordu, %d", len(rows))
	}
	if rows[0].Device != "203.0.113.5" {
		t.Errorf("exporter override uygulanmadı: %q", rows[0].Device)
	}
	if rows[0].Proto != "udp" || rows[0].DstPort != 53 {
		t.Errorf("genişletilmiş örnek yanlış çözüldü: %+v", rows[0])
	}
	if rows[0].Octets != 90*1000 {
		t.Errorf("Octets = %d", rows[0].Octets)
	}
}

func TestParseSFlowCounterSampleIgnored(t *testing.T) {
	// counter sample (format 2) — atlanmalı, panik olmamalı
	cs := []byte{}
	cs = append(cs, be32(2)...) // sample type = counters_sample
	cs = append(cs, be32(8)...) // length
	cs = append(cs, be32(1)...) // seq
	cs = append(cs, be32(0)...) // num records = 0
	dg := sflowDatagram(cs, 1)
	if rows := ParseSFlow(dg, "", time.Now()); len(rows) != 0 {
		t.Fatalf("counter sample satır üretmemeli: %d", len(rows))
	}
}

func TestParseSFlowRejectsNonV5(t *testing.T) {
	if rows := ParseSFlow(be32(4), "", time.Now()); rows != nil {
		t.Fatal("v4 reddedilmeliydi")
	}
	if rows := ParseSFlow([]byte{1, 2}, "", time.Now()); rows != nil {
		t.Fatal("kısa datagram reddedilmeliydi")
	}
}

// TestCollectorRoutesSFlow, Collector.parse'ın sFlow'u NetFlow'dan ayırdığını doğrular.
func TestCollectorRoutesSFlow(t *testing.T) {
	c := &Collector{}
	frame := buildEthIPv4TCP([4]byte{172, 16, 0, 5}, [4]byte{1, 1, 1, 1}, 40000, 80, 6)
	dg := sflowDatagram(flowSample(256, rawPacketRecord(frame, 800), 1, false), 1)

	rows := c.parse(dg, "dev", "1.2.3.4", time.Now())
	if len(rows) != 1 || rows[0].DstPort != 80 {
		t.Fatalf("sFlow Collector.parse üzerinden çözülmedi: %+v", rows)
	}
}
