package main

// flowgen.go — S21.1: sentetik NetFlow v5/v9 + IPFIX + sFlow v5 datagram
// üreteci. Her sahte exporter tek bir protokole bağlıdır (gerçek dünya);
// v9/IPFIX exporter'ları şablonu ilk birkaç datagram'da ve sonra her
// templateEvery datagram'da bir yeniler, aradakiler yalnız veri taşır.
// Amaç: internal/flows collector'ının ayrıştırma + yazım yolunu hedef hızda
// (≥ 50k flow/sn, patlamada 200k) sürmek.

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

type flowGenConfig struct {
	target     string
	proto      string // v5 | v9 | ipfix | sflow | mix
	rate       int    // sürekli flow kaydı/sn
	burst      int    // patlama flow kaydı/sn (0 = yok)
	burstAfter time.Duration
	burstFor   time.Duration
	exporters  int
}

const (
	recsPerDatagram = 24 // datagram başına flow kaydı (gerçekçi NetFlow yoğunluğu)
	flowSenders     = 4  // paralel gönderici goroutine
	v9TemplateID    = 256
	templateEvery   = 400 // her exporter bu kadar datagram'da bir şablonu yeniler
)

// runFlowGen, hedef UDP adresine sentetik flow datagramları gönderir.
func runFlowGen(ctx context.Context, cfg flowGenConfig, st *stats) {
	protos := protoList(cfg.proto)
	if len(protos) == 0 {
		fmt.Fprintf(os.Stderr, "geçersiz -flow-proto: %q\n", cfg.proto)
		return
	}
	if cfg.exporters < 1 {
		cfg.exporters = 1
	}

	udpAddr, err := net.ResolveUDPAddr("udp", cfg.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flow-target çözülemedi: %v\n", err)
		return
	}

	start := time.Now()
	var wg sync.WaitGroup
	for s := 0; s < flowSenders; s++ {
		conn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "flow UDP dial: %v\n", err)
			break
		}
		wg.Add(1)
		go func(senderID int, conn *net.UDPConn) {
			defer wg.Done()
			defer func() { _ = conn.Close() }()
			sendLoop(ctx, conn, cfg, protos, senderID, start, st)
		}(s, conn)
	}
	wg.Wait()
}

// sendLoop, tek göndericinin token-bucket temposunda datagram akıtması.
// Her exporter tek bir protokole bağlıdır (gerçek exporter davranışı) —
// exporter index'i protokol listesine mod alınarak dağıtılır. v9/IPFIX
// exporter'ları şablonu ilk datagram'larında ve her templateEvery datagram'da
// bir yeniler.
func sendLoop(ctx context.Context, conn *net.UDPConn, cfg flowGenConfig, protos []string, senderID int, start time.Time, st *stats) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(senderID)<<32))
	share := float64(flowSenders)
	last := time.Now()
	var credit float64
	var dgCount int64
	perExp := make([]int64, cfg.exporters)

	for ctx.Err() == nil {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now

		rate := float64(cfg.rate) / share
		if cfg.burst > 0 {
			el := now.Sub(start)
			if el >= cfg.burstAfter && el < cfg.burstAfter+cfg.burstFor {
				rate = float64(cfg.burst) / share
			}
		}
		credit += rate * dt
		if credit > rate { // en fazla 1 sn backlog — duraklamadan sonra taşmasın
			credit = rate
		}

		sent := 0
		for credit >= recsPerDatagram && ctx.Err() == nil {
			exporter := int(dgCount) % cfg.exporters
			proto := protos[exporter%len(protos)]
			k := perExp[exporter]
			perExp[exporter]++
			withTemplate := k < 3 || k%templateEvery == 0

			dg := buildDatagram(proto, exporter, withTemplate, now, rng)
			if _, err := conn.Write(dg); err != nil {
				st.flowErrors.Add(1)
			} else {
				st.flowDatagrams.Add(1)
				st.flowRecords.Add(recsPerDatagram)
			}
			credit -= recsPerDatagram
			dgCount++
			sent++
			if sent >= 64 { // uzun burst'te scheduler'a nefes ver
				break
			}
		}
		if sent == 0 {
			time.Sleep(time.Millisecond)
		}
	}
}

func protoList(p string) []string {
	switch p {
	case "v5", "v9", "ipfix", "sflow":
		return []string{p}
	case "mix", "":
		return []string{"v9", "ipfix", "sflow", "v5"}
	default:
		return nil
	}
}

// --- kayıt üretimi ---

type flowRec struct {
	src, dst       [4]byte
	sport, dport   uint16
	proto          uint8
	packets, bytes uint32
}

var flowProtos = []uint8{6, 6, 6, 17, 1} // ağırlıklı tcp

func randFlows(rng *rand.Rand, exporter int) []flowRec {
	recs := make([]flowRec, recsPerDatagram)
	for i := range recs {
		r := flowRec{
			// kaynak: exporter'a bağlı /16 iç ağ; hedef: rastgele "internet"
			src:   [4]byte{10, byte(exporter), byte(rng.Intn(256)), byte(1 + rng.Intn(254))},
			dst:   [4]byte{byte(1 + rng.Intn(223)), byte(rng.Intn(256)), byte(rng.Intn(256)), byte(1 + rng.Intn(254))},
			sport: uint16(1024 + rng.Intn(64000)),
			dport: []uint16{80, 443, 443, 443, 22, 53, 8080}[rng.Intn(7)],
			proto: flowProtos[rng.Intn(len(flowProtos))],
		}
		r.packets = uint32(1 + rng.Intn(4000))
		r.bytes = r.packets * uint32(40+rng.Intn(1460))
		recs[i] = r
	}
	return recs
}

func buildDatagram(proto string, exporter int, withTemplate bool, now time.Time, rng *rand.Rand) []byte {
	recs := randFlows(rng, exporter)
	switch proto {
	case "v5":
		return buildV5(recs, exporter, now)
	case "v9":
		return buildV9(recs, exporter, withTemplate, now)
	case "ipfix":
		return buildIPFIX(recs, exporter, withTemplate, now)
	case "sflow":
		return buildSFlow(recs, exporter, rng)
	}
	return nil
}

// --- NetFlow v5 (sabit 24B header + 48B kayıt) ---

func buildV5(recs []flowRec, exporter int, now time.Time) []byte {
	b := make([]byte, 24+len(recs)*48)
	binary.BigEndian.PutUint16(b[0:], 5)
	binary.BigEndian.PutUint16(b[2:], uint16(len(recs)))
	binary.BigEndian.PutUint32(b[4:], uint32(now.UnixMilli()&0x7fffffff)) // sysUptime ms
	binary.BigEndian.PutUint32(b[8:], uint32(now.Unix()))
	binary.BigEndian.PutUint32(b[12:], uint32(now.Nanosecond()))
	binary.BigEndian.PutUint32(b[16:], uint32(exporter)) // flow sequence
	off := 24
	for _, r := range recs {
		copy(b[off:], r.src[:])
		copy(b[off+4:], r.dst[:])
		binary.BigEndian.PutUint32(b[off+16:], r.packets)
		binary.BigEndian.PutUint32(b[off+20:], r.bytes)
		binary.BigEndian.PutUint16(b[off+32:], r.sport)
		binary.BigEndian.PutUint16(b[off+34:], r.dport)
		b[off+38] = r.proto
		off += 48
	}
	return b
}

// v9/IPFIX ortak alan listesi: srcIPv4, dstIPv4, srcPort, dstPort, proto,
// inBytes, inPkts → sabit 21 baytlık kayıt.
var flowFields = []struct {
	typ, length uint16
}{
	{8, 4}, {12, 4}, {7, 2}, {11, 2}, {4, 1}, {1, 4}, {2, 4},
}

const flowRecLen = 4 + 4 + 2 + 2 + 1 + 4 + 4

func encodeFlowRecord(dst []byte, r flowRec) {
	copy(dst[0:], r.src[:])
	copy(dst[4:], r.dst[:])
	binary.BigEndian.PutUint16(dst[8:], r.sport)
	binary.BigEndian.PutUint16(dst[10:], r.dport)
	dst[12] = r.proto
	binary.BigEndian.PutUint32(dst[13:], r.bytes)
	binary.BigEndian.PutUint32(dst[17:], r.packets)
}

// --- NetFlow v9 ---

func buildV9(recs []flowRec, exporter int, withTemplate bool, now time.Time) []byte {
	var tmplSet []byte
	if withTemplate {
		// flowset 0: id(2) len(2) [templateID(2) fieldCount(2) fields...]
		fieldsLen := len(flowFields) * 4
		tmplSet = make([]byte, 4+4+fieldsLen)
		binary.BigEndian.PutUint16(tmplSet[0:], 0)
		binary.BigEndian.PutUint16(tmplSet[2:], uint16(len(tmplSet)))
		binary.BigEndian.PutUint16(tmplSet[4:], v9TemplateID)
		binary.BigEndian.PutUint16(tmplSet[6:], uint16(len(flowFields)))
		o := 8
		for _, f := range flowFields {
			binary.BigEndian.PutUint16(tmplSet[o:], f.typ)
			binary.BigEndian.PutUint16(tmplSet[o+2:], f.length)
			o += 4
		}
	}

	dataBody := make([]byte, len(recs)*flowRecLen)
	for i, r := range recs {
		encodeFlowRecord(dataBody[i*flowRecLen:], r)
	}
	dataSet := make([]byte, 4+len(dataBody))
	binary.BigEndian.PutUint16(dataSet[0:], v9TemplateID)
	binary.BigEndian.PutUint16(dataSet[2:], uint16(len(dataSet)))
	copy(dataSet[4:], dataBody)

	count := 1
	if withTemplate {
		count = 2
	}
	hdr := make([]byte, 20)
	binary.BigEndian.PutUint16(hdr[0:], 9)
	binary.BigEndian.PutUint16(hdr[2:], uint16(count))
	binary.BigEndian.PutUint32(hdr[4:], uint32(now.UnixMilli()&0x7fffffff))
	binary.BigEndian.PutUint32(hdr[8:], uint32(now.Unix()))
	binary.BigEndian.PutUint32(hdr[12:], uint32(exporter)) // seq
	binary.BigEndian.PutUint32(hdr[16:], uint32(1000+exporter))

	out := append(hdr, tmplSet...)
	return append(out, dataSet...)
}

// --- IPFIX / NetFlow v10 ---

func buildIPFIX(recs []flowRec, exporter int, withTemplate bool, now time.Time) []byte {
	var tmplSet []byte
	if withTemplate {
		fieldsLen := len(flowFields) * 4
		tmplSet = make([]byte, 4+4+fieldsLen)
		binary.BigEndian.PutUint16(tmplSet[0:], 2) // set id 2 = template
		binary.BigEndian.PutUint16(tmplSet[2:], uint16(len(tmplSet)))
		binary.BigEndian.PutUint16(tmplSet[4:], v9TemplateID)
		binary.BigEndian.PutUint16(tmplSet[6:], uint16(len(flowFields)))
		o := 8
		for _, f := range flowFields {
			binary.BigEndian.PutUint16(tmplSet[o:], f.typ)
			binary.BigEndian.PutUint16(tmplSet[o+2:], f.length)
			o += 4
		}
	}

	dataBody := make([]byte, len(recs)*flowRecLen)
	for i, r := range recs {
		encodeFlowRecord(dataBody[i*flowRecLen:], r)
	}
	dataSet := make([]byte, 4+len(dataBody))
	binary.BigEndian.PutUint16(dataSet[0:], v9TemplateID)
	binary.BigEndian.PutUint16(dataSet[2:], uint16(len(dataSet)))
	copy(dataSet[4:], dataBody)

	total := 16 + len(tmplSet) + len(dataSet)
	hdr := make([]byte, 16)
	binary.BigEndian.PutUint16(hdr[0:], 10)
	binary.BigEndian.PutUint16(hdr[2:], uint16(total))
	binary.BigEndian.PutUint32(hdr[4:], uint32(now.Unix())) // export time
	binary.BigEndian.PutUint32(hdr[8:], uint32(exporter))   // seq
	binary.BigEndian.PutUint32(hdr[12:], uint32(1000+exporter))

	out := append(hdr, tmplSet...)
	return append(out, dataSet...)
}

// --- sFlow v5 (örnekleme tabanlı; ham paket başlığı gömülür) ---

func buildSFlow(recs []flowRec, exporter int, rng *rand.Rand) []byte {
	w := &xdrWriter{}
	w.u32(5)                                    // version
	w.u32(1)                                    // agent address type = IPv4
	w.raw([]byte{10, 0, 0, byte(1 + exporter)}) // agent address
	w.u32(0)                                    // sub agent id
	w.u32(uint32(exporter))                     // sequence
	w.u32(uint32(time.Now().UnixMilli() & 0x7fffffff))
	w.u32(uint32(len(recs))) // num samples

	for _, r := range recs {
		sample := buildSFlowFlowSample(r, rng)
		w.u32(1) // sample type = flow_sample (format 1)
		w.u32(uint32(len(sample)))
		w.rawAligned(sample)
	}
	return w.bytes()
}

func buildSFlowFlowSample(r flowRec, rng *rand.Rand) []byte {
	rate := []uint32{1, 512, 1024, 2048}[rng.Intn(4)]
	frameLen := 40 + rng.Intn(1460)

	// ham paket başlığı: Ethernet(14) + IPv4(20) + L4(ilk 4 bayt port)
	hdr := make([]byte, 14+20+4)
	binary.BigEndian.PutUint16(hdr[12:], 0x0800) // ethertype IPv4
	hdr[14] = 0x45                               // v4, IHL 5
	hdr[23] = r.proto
	copy(hdr[26:], r.src[:])
	copy(hdr[30:], r.dst[:])
	binary.BigEndian.PutUint16(hdr[34:], r.sport)
	binary.BigEndian.PutUint16(hdr[36:], r.dport)

	w := &xdrWriter{}
	w.u32(uint32(rng.Intn(1 << 20))) // sample sequence
	w.u32(0)                         // source id
	w.u32(rate)                      // sampling rate
	w.u32(uint32(frameLen) * rate)   // sample pool
	w.u32(0)                         // drops
	w.u32(0)                         // input if
	w.u32(0)                         // output if
	w.u32(1)                         // num records

	// raw packet header record (format 1)
	rec := &xdrWriter{}
	rec.u32(1)                // header protocol = ETHERNET
	rec.u32(uint32(frameLen)) // frame length
	rec.u32(0)                // stripped
	rec.u32(uint32(len(hdr))) // header length
	rec.rawAligned(hdr)

	recBytes := rec.bytes()
	w.u32(1) // record type = raw packet header
	w.u32(uint32(len(recBytes)))
	w.rawAligned(recBytes)
	return w.bytes()
}

// xdrWriter, 4-bayt hizalı (XDR) big-endian yazıcı.
type xdrWriter struct{ buf []byte }

func (w *xdrWriter) u32(v uint32) {
	var t [4]byte
	binary.BigEndian.PutUint32(t[:], v)
	w.buf = append(w.buf, t[:]...)
}

func (w *xdrWriter) raw(b []byte) { w.buf = append(w.buf, b...) }

func (w *xdrWriter) rawAligned(b []byte) {
	w.buf = append(w.buf, b...)
	for len(w.buf)%4 != 0 {
		w.buf = append(w.buf, 0)
	}
}

func (w *xdrWriter) bytes() []byte { return w.buf }
