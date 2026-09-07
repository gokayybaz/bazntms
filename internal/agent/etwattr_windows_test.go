//go:build windows

package agent

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestETWLayout(t *testing.T) {
	if err := checkLayout(); err != nil {
		t.Fatal(err)
	}
}

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

	// TCP recv (id 11): uzak uç = saddr:sport
	k, out, in, ok = kernelNetFlow(11, blob, 9999)
	if !ok || in != 1460 || out != 0 {
		t.Fatalf("recv: ok=%v out=%d in=%d", ok, out, in)
	}
	if k.remoteIP != "192.168.1.5" || k.port != 51000 {
		t.Fatalf("recv key: %+v", k)
	}

	// UDP send (id 42)
	k, _, _, ok = kernelNetFlow(42, buildKernelNetV4(7, 64, "8.8.8.8", "10.0.0.2", 53, 40000), 0)
	if !ok || k.proto != "udp" || k.remoteIP != "8.8.8.8" || k.port != 53 {
		t.Fatalf("udp send: %+v ok=%v", k, ok)
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
}

func TestKernelNetFlowV6(t *testing.T) {
	b := make([]byte, 44)
	binary.LittleEndian.PutUint32(b[0:4], 55)
	binary.LittleEndian.PutUint32(b[4:8], 200)
	copy(b[8:24], net.ParseIP("2606:4700:4700::1111"))
	copy(b[24:40], net.ParseIP("fe80::1"))
	binary.BigEndian.PutUint16(b[40:42], 443)
	binary.BigEndian.PutUint16(b[42:44], 60000)

	k, out, _, ok := kernelNetFlow(26, b, 0) // TCP send v6
	if !ok || out != 200 || k.remoteIP != "2606:4700:4700::1111" || k.port != 443 {
		t.Fatalf("v6 send: %+v out=%d ok=%v", k, out, ok)
	}
	k, _, in, ok := kernelNetFlow(59, b, 0) // UDP recv v6
	if !ok || in != 200 || k.proto != "udp" || k.remoteIP != "fe80::1" || k.port != 60000 {
		t.Fatalf("v6 udp recv: %+v in=%d ok=%v", k, in, ok)
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
