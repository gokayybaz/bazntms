package capture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func TestRecordingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine()
	e.SetRecordOptions(dir, 0) // rotasyon kapali

	e.mu.Lock()
	e.device = "en0"
	e.linkType = layers.LinkTypeEthernet
	e.running = true // cgo/izin gerektirmeyen sahte "yakalama acik" durumu
	e.mu.Unlock()

	if err := e.StartRecording(); err != nil {
		t.Fatalf("kayit baslatilamadi: %v", err)
	}
	st := e.RecordStatus()
	if !st.Recording || st.File == "" {
		t.Fatalf("kayit durumu hatali: %+v", st)
	}

	// sentetik ethernet paketi yaz
	eth := &layers.Ethernet{
		SrcMAC:       []byte{1, 2, 3, 4, 5, 6},
		DstMAC:       []byte{6, 5, 4, 3, 2, 1},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{Version: 4, TTL: 64, SrcIP: []byte{10, 0, 0, 1}, DstIP: []byte{10, 0, 0, 2}, Protocol: layers.IPProtocolUDP}
	udp := &layers.UDP{SrcPort: 5353, DstPort: 53}
	udp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, udp, gopacket.Payload([]byte("merhaba"))); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	data := buf.Bytes()
	ci := gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(data), Length: len(data)}

	for i := 0; i < 5; i++ {
		e.recordPacket(data, ci)
	}
	info, err := e.StopRecording()
	if err != nil {
		t.Fatalf("kayit durdurulamadi: %v", err)
	}
	if info.Packets != 5 || info.Bytes != 5*uint64(len(data)) {
		t.Fatalf("ozet hatali: %+v", info)
	}
	if e.RecordStatus().Recording {
		t.Fatal("durdurulduktan sonra hala kayitta")
	}

	// dosyayi pcapgo okuyucusuyla dogrula (Wireshark uyumlulugu)
	f, err := os.Open(filepath.Join(dir, info.File))
	if err != nil {
		t.Fatalf("dosya yok: %v", err)
	}
	defer f.Close()
	r, err := pcapgo.NewReader(f)
	if err != nil {
		t.Fatalf("pcap okunamadi: %v", err)
	}
	if r.LinkType() != layers.LinkTypeEthernet {
		t.Fatalf("link tipi hatali: %v", r.LinkType())
	}
	n := 0
	for {
		d, _, err := r.ReadPacketData()
		if err != nil {
			break
		}
		if len(d) != len(data) {
			t.Fatalf("paket boyutu hatali: %d != %d", len(d), len(data))
		}
		n++
	}
	if n != 5 {
		t.Fatalf("5 paket beklenirdi: %d", n)
	}
}
