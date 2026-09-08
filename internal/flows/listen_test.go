package flows

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// v9 datagramı: 20B header + template flowset (id 0) + data flowset (id 256).
// Alanlar: srcIPv4(8), dstIPv4(12), srcPort(7), dstPort(11), proto(4),
// inBytes(1), inPkts(2) — sabit 21B kayıt.
func v9Datagram(sourceID uint32, records int) []byte {
	fields := []struct{ typ, length uint16 }{
		{8, 4}, {12, 4}, {7, 2}, {11, 2}, {4, 1}, {1, 4}, {2, 4},
	}
	tmpl := make([]byte, 4+4+len(fields)*4)
	binary.BigEndian.PutUint16(tmpl[0:], 0)
	binary.BigEndian.PutUint16(tmpl[2:], uint16(len(tmpl)))
	binary.BigEndian.PutUint16(tmpl[4:], 256)
	binary.BigEndian.PutUint16(tmpl[6:], uint16(len(fields)))
	o := 8
	for _, f := range fields {
		binary.BigEndian.PutUint16(tmpl[o:], f.typ)
		binary.BigEndian.PutUint16(tmpl[o+2:], f.length)
		o += 4
	}

	const recLen = 21
	data := make([]byte, 4+records*recLen)
	binary.BigEndian.PutUint16(data[0:], 256)
	binary.BigEndian.PutUint16(data[2:], uint16(len(data)))
	for i := 0; i < records; i++ {
		r := data[4+i*recLen:]
		copy(r[0:], []byte{10, 1, 2, byte(i + 1)})
		copy(r[4:], []byte{8, 8, 8, 8})
		binary.BigEndian.PutUint16(r[8:], uint16(40000+i))
		binary.BigEndian.PutUint16(r[10:], 443)
		r[12] = 6
		binary.BigEndian.PutUint32(r[13:], 1500)
		binary.BigEndian.PutUint32(r[17:], 10)
	}

	hdr := make([]byte, 20)
	binary.BigEndian.PutUint16(hdr[0:], 9)
	binary.BigEndian.PutUint16(hdr[2:], 2)
	binary.BigEndian.PutUint32(hdr[16:], sourceID)
	return append(append(hdr, tmpl...), data...)
}

// TestListenPipeline, reader→worker hattının datagram'ları alıp OnFlows'a
// ilettiğini ve Close'un temiz kapandığını doğrular (S21.9).
func TestListenPipeline(t *testing.T) {
	var mu sync.Mutex
	total := 0
	c := &Collector{
		Workers: 4,
		OnFlows: func(device string, rows []Row) {
			mu.Lock()
			total += len(rows)
			mu.Unlock()
		},
	}
	if err := c.Listen("127.0.0.1:0"); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := c.Conn.LocalAddr().(*net.UDPAddr)

	send, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer send.Close()

	const datagrams = 200
	const recsPer = 20
	for i := 0; i < datagrams; i++ {
		if _, err := send.Write(v9Datagram(uint32(1+i%4), recsPer)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// hattın boşalmasını bekle
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		got := total
		mu.Unlock()
		if got >= datagrams*recsPer {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("zaman aşımı: %d/%d kayıt işlendi", got, datagrams*recsPer)
		}
		time.Sleep(20 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() { c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close 3sn içinde dönmedi (goroutine sızıntısı?)")
	}
}

// TestListenConcurrentTemplateCache, çoklu worker'ın paylaşılan şablon
// önbelleğine yarışsız eriştiğini (RWMutex) doğrular — -race ile anlamlı.
func TestListenConcurrentTemplateCache(t *testing.T) {
	var recv atomic.Int64
	c := &Collector{
		Workers: 8,
		OnFlows: func(_ string, rows []Row) { recv.Add(int64(len(rows))) },
	}
	if err := c.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	addr := c.Conn.LocalAddr().(*net.UDPAddr)

	var wg sync.WaitGroup
	for s := 0; s < 6; s++ {
		wg.Add(1)
		go func(sourceID uint32) {
			defer wg.Done()
			conn, err := net.DialUDP("udp", nil, addr)
			if err != nil {
				return
			}
			defer conn.Close()
			for i := 0; i < 100; i++ {
				_, _ = conn.Write(v9Datagram(sourceID, 5))
			}
		}(uint32(100 + s))
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	if recv.Load() == 0 {
		t.Fatal("hiç kayıt işlenmedi")
	}
}
