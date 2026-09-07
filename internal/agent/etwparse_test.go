package agent

import (
	"encoding/binary"
	"net"
	"testing"
)

// buildKernelNetV4, bir Kernel-Network v4 olayının UserData blob'unu kurar.
func buildKernelNetV4(pid, size uint32, daddr, saddr string, dport, sport uint16) []byte {
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[0:4], pid)
	binary.LittleEndian.PutUint32(b[4:8], size)
	copy(b[8:12], net.ParseIP(daddr).To4())
	copy(b[12:16], net.ParseIP(saddr).To4())
	binary.BigEndian.PutUint16(b[16:18], dport) // ağ sırası
	binary.BigEndian.PutUint16(b[18:20], sport)
	return b
}

func TestKernelNetFlowV4(t *testing.T) {
	// TCP send (id 10): uzak uç = daddr:dport
	blob := buildKernelNetV4(1234, 1460, "93.184.216.34", "192.168.1.5", 443, 51000)
	k, out, in, ok := kernelNetFlow(10, blob, 9999)
	if !ok || out != 1460 || in != 0 {
		t.Fatalf("send: ok=%v out=%d in=%d", ok, out, in)
	}
	if k.pid != 1234 || k.proto != "tcp" || k.remoteIP != "93.184.216.34" || k.port != 443 {
		t.Fatalf("send key: %+v", k)
	}

	// TCP recv (id 11): bağlantı-yönelimli — peer yine daddr:dport (canlı ETW
	// verisiyle doğrulandı), yalnız bayt yönü in.
	k, out, in, ok = kernelNetFlow(11, blob, 9999)
	if !ok || in != 1460 || out != 0 {
		t.Fatalf("recv: ok=%v out=%d in=%d", ok, out, in)
	}
	if k.remoteIP != "93.184.216.34" || k.port != 443 {
		t.Fatalf("recv key: %+v", k)
	}

	// UDP send (id 42): peer = daddr:dport
	k, _, _, ok = kernelNetFlow(42, buildKernelNetV4(7, 64, "8.8.8.8", "10.0.0.2", 53, 40000), 0)
	if !ok || k.proto != "udp" || k.remoteIP != "8.8.8.8" || k.port != 53 {
		t.Fatalf("udp send: %+v ok=%v", k, ok)
	}

	// UDP recv (id 43): paket-yönelimli — peer = saddr:sport (uçlar ters)
	k, _, in, ok = kernelNetFlow(43, buildKernelNetV4(7, 64, "10.0.0.2", "8.8.8.8", 40000, 53), 0)
	if !ok || in != 64 || k.proto != "udp" || k.remoteIP != "8.8.8.8" || k.port != 53 {
		t.Fatalf("udp recv: %+v in=%d ok=%v", k, in, ok)
	}

	// header PID fallback (UserData PID = 0)
	k, _, _, ok = kernelNetFlow(10, buildKernelNetV4(0, 100, "1.1.1.1", "2.2.2.2", 443, 5000), 4242)
	if !ok || k.pid != 4242 {
		t.Fatalf("header pid fallback: %+v", k)
	}

	// ilgisiz id / kısa blob / sıfır boyut → ok=false
	if _, _, _, ok := kernelNetFlow(12, blob, 0); ok {
		t.Fatal("id 12 (connect) atlanmalı")
	}
	if _, _, _, ok := kernelNetFlow(10, blob[:10], 0); ok {
		t.Fatal("kısa blob reddedilmeli")
	}
	if _, _, _, ok := kernelNetFlow(10, buildKernelNetV4(1, 0, "1.1.1.1", "2.2.2.2", 1, 2), 0); ok {
		t.Fatal("sıfır boyut atlanmalı")
	}

	// loopback uzak uç → elenir (eBPF ile aynı: yerel IPC atfı kirletmesin)
	if _, _, _, ok := kernelNetFlow(10, buildKernelNetV4(1, 100, "127.0.0.1", "127.0.0.1", 443, 5000), 0); ok {
		t.Fatal("v4 loopback send atlanmalı")
	}
	if _, _, _, ok := kernelNetFlow(11, buildKernelNetV4(1, 100, "127.0.0.53", "10.0.0.1", 53, 40000), 0); ok {
		t.Fatal("v4 TCP recv loopback (daddr) atlanmalı")
	}
	if _, _, _, ok := kernelNetFlow(43, buildKernelNetV4(1, 100, "10.0.0.1", "127.0.0.53", 40000, 53), 0); ok {
		t.Fatal("v4 UDP recv loopback (saddr) atlanmalı")
	}

	// mDNS: kendi çok-noktalı paketimiz geri döner → recv olayında saddr=biz,
	// daddr=224.0.0.251. İki uçtan biri multicast ise elenir.
	if _, _, _, ok := kernelNetFlow(43, buildKernelNetV4(1, 883, "224.0.0.251", "192.168.1.33", 5353, 5353), 0); ok {
		t.Fatal("mDNS multicast (daddr) atlanmalı")
	}
	// SSDP send → daddr=239.255.255.250
	if _, _, _, ok := kernelNetFlow(42, buildKernelNetV4(1, 200, "239.255.255.250", "192.168.1.33", 1900, 55000), 0); ok {
		t.Fatal("SSDP multicast atlanmalı")
	}
}

func TestKernelNetFlowV6(t *testing.T) {
	b := make([]byte, 44)
	binary.LittleEndian.PutUint32(b[0:4], 55)
	binary.LittleEndian.PutUint32(b[4:8], 200)
	copy(b[8:24], net.ParseIP("2606:4700:4700::1111"))
	copy(b[24:40], net.ParseIP("fe80::1"))
	binary.BigEndian.PutUint16(b[40:42], 443)
	binary.BigEndian.PutUint16(b[42:44], 60000)

	k, out, _, ok := kernelNetFlow(26, b, 0) // TCP send v6 → daddr:dport
	if !ok || out != 200 || k.remoteIP != "2606:4700:4700::1111" || k.port != 443 {
		t.Fatalf("v6 send: %+v out=%d ok=%v", k, out, ok)
	}
	k, _, in, ok := kernelNetFlow(27, b, 0) // TCP recv v6 → yine daddr:dport
	if !ok || in != 200 || k.remoteIP != "2606:4700:4700::1111" || k.port != 443 {
		t.Fatalf("v6 tcp recv: %+v in=%d ok=%v", k, in, ok)
	}
	k, _, in, ok = kernelNetFlow(59, b, 0) // UDP recv v6 → saddr:sport
	if !ok || in != 200 || k.proto != "udp" || k.remoteIP != "fe80::1" || k.port != 60000 {
		t.Fatalf("v6 udp recv: %+v in=%d ok=%v", k, in, ok)
	}

	// ::1 loopback → elenir
	lb := make([]byte, 44)
	binary.LittleEndian.PutUint32(lb[4:8], 100)
	copy(lb[8:24], net.ParseIP("::1"))
	copy(lb[24:40], net.ParseIP("::1"))
	binary.BigEndian.PutUint16(lb[40:42], 443)
	if _, _, _, ok := kernelNetFlow(26, lb, 0); ok {
		t.Fatal("v6 ::1 loopback atlanmalı")
	}
}

func TestKernelNetWanted(t *testing.T) {
	for _, id := range []uint16{10, 11, 26, 27, 42, 43, 58, 59} {
		if !kernelNetWanted(id) {
			t.Errorf("id %d istenmeli", id)
		}
	}
	for _, id := range []uint16{0, 12, 13, 15, 49, 100} {
		if kernelNetWanted(id) {
			t.Errorf("id %d istenmemeli", id)
		}
	}
}

// utf16zLE, bir dizeyi null-sonlu little-endian UTF-16 bayt dizisine çevirir
// (DNS-Client olay UserData'sı bu biçimde).
func utf16zLE(s string) []byte {
	b := make([]byte, 0, len(s)*2+2)
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return append(b, 0, 0)
}

func TestDNSClientDomain(t *testing.T) {
	// 3006 (sorgu) → isResp=false
	dom, isResp, ok := dnsClientDomain(3006, utf16zLE("api.github.com"))
	if !ok || isResp || dom != "api.github.com" {
		t.Fatalf("3006: dom=%q resp=%v ok=%v", dom, isResp, ok)
	}
	// 3008 (yanıt) → isResp=true; trailing dot + büyük harf normalize
	dom, isResp, ok = dnsClientDomain(3008, utf16zLE("CDN.Example.COM."))
	if !ok || !isResp || dom != "cdn.example.com" {
		t.Fatalf("3008: dom=%q resp=%v ok=%v", dom, isResp, ok)
	}
	// ekstra veri (QueryType vb.) QueryName'den sonra → yalnız ad okunur
	blob := append(utf16zLE("example.org"), 0x1c, 0x00, 0x00, 0x00) // + QueryType
	if dom, _, ok := dnsClientDomain(3006, blob); !ok || dom != "example.org" {
		t.Fatalf("kuyruklu blob: dom=%q ok=%v", dom, ok)
	}
	// ters arama / .local / noktasız / ilgisiz id → elenir
	for _, bad := range []string{"1.0.0.127.in-addr.arpa", "printer.local", "wpad"} {
		if _, _, ok := dnsClientDomain(3006, utf16zLE(bad)); ok {
			t.Errorf("%q elenmeliydi", bad)
		}
	}
	if _, _, ok := dnsClientDomain(3020, utf16zLE("example.com")); ok {
		t.Error("id 3020 istenmemeli")
	}
	if _, _, ok := dnsClientDomain(3006, nil); ok {
		t.Error("boş blob reddedilmeli")
	}
}
