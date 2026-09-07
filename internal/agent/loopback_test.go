package agent

import (
	"net"
	"testing"

	"github.com/gokayybaz/bazntms/pkg/proctraffic"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// dnsUDPPacket, loopback stub-resolver'a giden bir DNS sorgusunu (Ethernet/
// IPv4/UDP/53) seri hale getirip cozulmus gopacket.Packet olarak dondurur.
func dnsUDPPacket(t *testing.T, srcIP, dstIP string, srcPort, dstPort int, name string, isResp bool) gopacket.Packet {
	t.Helper()
	eth := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0, 0, 0, 0, 0, 1}, DstMAC: net.HardwareAddr{0, 0, 0, 0, 0, 2},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolUDP,
		SrcIP: net.ParseIP(srcIP), DstIP: net.ParseIP(dstIP),
	}
	udp := &layers.UDP{SrcPort: layers.UDPPort(srcPort), DstPort: layers.UDPPort(dstPort)}
	_ = udp.SetNetworkLayerForChecksum(ip)
	dns := &layers.DNS{
		ID: 42, QR: isResp, QDCount: 1,
		Questions: []layers.DNSQuestion{{Name: []byte(name), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true},
		eth, ip, udp, dns); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
}

func TestAttributeDNSLoopback(t *testing.T) {
	e := &pcapAttrSource{dns: map[dnsKey]*dnsAgg{}}

	// stub-resolver: uygulama 127.0.0.1:51000 → 127.0.0.53:53
	full := map[proctraffic.Key]ProcInfoAlias{
		{Proto: "udp", LocalIP: "127.0.0.1", LocalPort: 51000, RemoteIP: "127.0.0.53", RemotePort: 53}: {PID: 900, Process: "firefox"},
	}
	index := map[portKey]ProcInfoAlias{}

	q := dnsUDPPacket(t, "127.0.0.1", "127.0.0.53", 51000, 53, "www.example.com", false)
	e.attributeDNS(q, full, index)
	r := dnsUDPPacket(t, "127.0.0.53", "127.0.0.1", 53, 51000, "www.example.com", true)
	e.attributeDNS(r, full, index)

	a := e.dns[dnsKey{pid: 900, process: "firefox", domain: "www.example.com"}]
	if a == nil || a.queries != 1 || a.responses != 1 {
		t.Fatalf("loopback DNS sürece atfedilmedi: %+v", a)
	}

	// attributeDNS e.totals'a DOKUNMAMALI (loopback trafiği süreç sayaçlarına girmez)
	if len(e.totals) != 0 {
		t.Fatalf("attributeDNS totals'a yazdı: %v", e.totals)
	}
}

func TestAttributeDNSMuslUnconnectedSocket(t *testing.T) {
	e := &pcapAttrSource{dns: map[dnsKey]*dnsAgg{}}

	// musl/BusyBox: soket connect() edilmez → /proc/net/udp'de uzak port 0.
	// proctraffic snapshot'i yalnizca yerel portu bilir → index[{udp, X, 0}].
	index := map[portKey]ProcInfoAlias{
		{proto: "udp", lp: 51000, rp: 0}: {PID: 42, Process: "wget"},
	}
	full := map[proctraffic.Key]ProcInfoAlias{}

	q := dnsUDPPacket(t, "172.18.0.10", "127.0.0.11", 51000, 53, "github.com", false)
	e.attributeDNS(q, full, index)
	r := dnsUDPPacket(t, "127.0.0.11", "172.18.0.10", 53, 51000, "github.com", true)
	e.attributeDNS(r, full, index)

	a := e.dns[dnsKey{pid: 42, process: "wget", domain: "github.com"}]
	if a == nil || a.queries != 1 || a.responses != 1 {
		t.Fatalf("bağlantısız UDP soketi sürece atfedilmedi: %+v", a)
	}
}

func TestAttributeDNSNoSocketStillRecordsDomain(t *testing.T) {
	e := &pcapAttrSource{dns: map[dnsKey]*dnsAgg{}}
	// eşleşen soket yok (kısa ömürlü nslookup) → alan adı yine de boş süreçle
	// kaydedilmeli: domain görünürlüğü süreç atfından bağımsız.
	pkt := dnsUDPPacket(t, "127.0.0.1", "127.0.0.11", 40000, 54262, "www.wikipedia.org", false)
	e.attributeDNS(pkt, map[proctraffic.Key]ProcInfoAlias{}, map[portKey]ProcInfoAlias{})
	a := e.dns[dnsKey{pid: 0, process: "", domain: "www.wikipedia.org"}]
	if a == nil || a.queries != 1 {
		t.Fatalf("boş süreçle domain kaydı bekleniyordu: %+v", a)
	}
}

func TestSniffDNSIgnoresNonDNS(t *testing.T) {
	e := &pcapAttrSource{dns: map[dnsKey]*dnsAgg{}}
	// loopback'teki DNS olmayan UDP (ör. statsd) → parseDNSNames eler
	e.sniffDNS("127.0.0.1", 40000, "127.0.0.1", 8125, []byte("page.views:1|c\n\x00\x00"),
		map[proctraffic.Key]ProcInfoAlias{}, map[portKey]ProcInfoAlias{})
	if len(e.dns) != 0 {
		t.Fatalf("DNS olmayan UDP kaydedildi: %v", e.dns)
	}
}

func TestLoopbackDevice(t *testing.T) {
	// pcap kurulu olmayabilir (CI matris); yalnızca ad boş dönmemeli VEYA
	// açık hata dönmeli — panik/asla-dönmeme olmasın.
	dev, err := loopbackDevice()
	if err == nil && dev == "" {
		t.Fatal("hata yokken cihaz adı boş dönmemeli")
	}
}
