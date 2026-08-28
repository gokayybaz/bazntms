package capture

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func serializeDNS(d *layers.DNS) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true}
	if err := gopacket.SerializeLayers(buf, opts, d); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestParseDNSQuery(t *testing.T) {
	payload := serializeDNS(&layers.DNS{
		ID:      42,
		QR:      false,
		QDCount: 1,
		Questions: []layers.DNSQuestion{
			{Name: []byte("Example.COM"), Type: layers.DNSTypeA, Class: layers.DNSClassIN},
		},
	})

	p := parseDNS(payload)
	if p == nil {
		t.Fatal("parse edilemedi")
	}
	if p.isResp {
		t.Fatal("sorgu yanit olarak isaretlendi")
	}
	if len(p.names) != 1 || p.names[0] != "example.com" {
		t.Fatalf("domain hatali: %v", p.names)
	}
}

func TestParseDNSResponseIPs(t *testing.T) {
	payload := serializeDNS(&layers.DNS{
		ID:      42,
		QR:      true,
		QDCount: 1,
		ANCount: 1,
		Questions: []layers.DNSQuestion{
			{Name: []byte("example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN},
		},
		Answers: []layers.DNSResourceRecord{
			{
				Type:  layers.DNSTypeA,
				Class: layers.DNSClassIN,
				IP:    net.IPv4(93, 184, 216, 34),
			},
		},
	})

	p := parseDNS(payload)
	if p == nil || !p.isResp {
		t.Fatal("yanit parse edilemedi")
	}
	found := false
	for _, ip := range p.answers {
		if ip == "93.184.216.34" {
			found = true
		}
	}
	if !found {
		t.Fatalf("yanit IP eksik: %v", p.answers)
	}
}

func TestParseDNSRejectsGarbage(t *testing.T) {
	if p := parseDNS([]byte{1, 2, 3}); p != nil {
		t.Fatal("bozuk payload parse edilmedi")
	}
	if p := parseDNS(nil); p != nil {
		t.Fatal("bos payload parse edilmedi")
	}
}

func TestApplyDNSAndTopDomains(t *testing.T) {
	e := NewEngine()

	// sorgu + yanit
	q := parseDNS(serializeDNS(&layers.DNS{
		ID: 1, QDCount: 1,
		Questions: []layers.DNSQuestion{{Name: []byte("api.example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}))
	e.applyDNS(q)

	r := parseDNS(serializeDNS(&layers.DNS{
		ID: 2, QR: true, QDCount: 1, ANCount: 1,
		Questions: []layers.DNSQuestion{{Name: []byte("api.example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
		Answers:   []layers.DNSResourceRecord{{Type: layers.DNSTypeA, Class: layers.DNSClassIN, IP: net.IPv4(1, 2, 3, 4)}},
	}))
	e.applyDNS(r)

	// ters DNS sorgulari sayilmamali
	ptr := parseDNS(serializeDNS(&layers.DNS{
		ID: 3, QDCount: 1,
		Questions: []layers.DNSQuestion{{Name: []byte("4.3.2.1.in-addr.arpa"), Type: layers.DNSTypePTR, Class: layers.DNSClassIN}},
	}))
	e.applyDNS(ptr)

	top := e.topDomainStats()
	if len(top) != 1 {
		t.Fatalf("beklenen 1 domain, gelen: %d", len(top))
	}
	d := top[0]
	if d.Domain != "api.example.com" || d.Queries != 1 || d.Responses != 1 {
		t.Fatalf("sayaclar hatali: %+v", d)
	}
	if len(d.IPs) != 1 || d.IPs[0] != "1.2.3.4" {
		t.Fatalf("IP eslesmesi hatali: %v", d.IPs)
	}
}
