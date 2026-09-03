package agent

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func dnsPacket(t *testing.T, name string, isResp bool, answer string) []byte {
	t.Helper()
	d := layers.DNS{
		ID:      1234,
		QR:      isResp,
		OpCode:  layers.DNSOpCodeQuery,
		QDCount: 1,
		Questions: []layers.DNSQuestion{
			{Name: []byte(name), Type: layers.DNSTypeA, Class: layers.DNSClassIN},
		},
	}
	if isResp && answer != "" {
		d.ANCount = 1
		d.Answers = []layers.DNSResourceRecord{
			{Name: []byte(name), Type: layers.DNSTypeA, Class: layers.DNSClassIN, IP: net.ParseIP(answer).To4()},
		}
	}
	buf := gopacket.NewSerializeBuffer()
	if err := d.SerializeTo(buf, gopacket.SerializeOptions{}); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return buf.Bytes()
}

func TestParseDNSNames(t *testing.T) {
	// sorgu
	names, isResp := parseDNSNames(dnsPacket(t, "www.example.com", false, ""))
	if isResp || len(names) != 1 || names[0] != "www.example.com" {
		t.Fatalf("sorgu çözülemedi: %v resp=%v", names, isResp)
	}
	// yanıt
	names, isResp = parseDNSNames(dnsPacket(t, "cdn.example.org", true, "1.2.3.4"))
	if !isResp || len(names) != 1 || names[0] != "cdn.example.org" {
		t.Fatalf("yanıt çözülemedi: %v resp=%v", names, isResp)
	}
	// büyük harf + trailing dot normalize
	names, _ = parseDNSNames(dnsPacket(t, "API.GitHub.COM.", false, ""))
	if len(names) != 1 || names[0] != "api.github.com" {
		t.Fatalf("normalize edilmedi: %v", names)
	}
	// ters arama + .local + noktasız → elenir
	for _, bad := range []string{"1.0.0.127.in-addr.arpa", "printer.local", "localhost"} {
		if names, _ := parseDNSNames(dnsPacket(t, bad, false, "")); len(names) != 0 {
			t.Fatalf("%q elenmeliydi: %v", bad, names)
		}
	}
	// çöp payload → panik yok, boş
	if names, _ := parseDNSNames([]byte{0x00, 0x01, 0x02}); len(names) != 0 {
		t.Fatalf("çöp payload'dan isim çıktı: %v", names)
	}
}
