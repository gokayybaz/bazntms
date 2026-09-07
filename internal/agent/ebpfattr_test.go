//go:build linux

package agent

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// nonLoopbackIface, makinenin ilk loopback-olmayan IPv4 arayüzünü (ad, IP)
// döndürür (Docker'da eth0). eBPF bayt sayımı loopback'i atladığından testler
// bunun üzerinden gider.
func nonLoopbackIface(t *testing.T) (name, ip string) {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Skipf("arayüzler okunamadı: %v", err)
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if v4 := ipn.IP.To4(); v4 != nil {
					return ifi.Name, v4.String()
				}
			}
		}
	}
	t.Skip("loopback-olmayan IPv4 arayüz yok")
	return "", ""
}

// TestEBPFAttrSourceCounting, canlı kernele programları yükler, loopback-olmayan
// bir IP üzerinden bilinen boyutta trafik üretir ve haritadan okunan baytın tam
// eşleştiğini doğrular. root + BTF gerektirir; yoksa atlar.
func TestEBPFAttrSourceCounting(t *testing.T) {
	if testing.Short() {
		t.Skip("kısa mod — canlı eBPF yüklemesi atlandı")
	}
	if os.Geteuid() != 0 {
		t.Skip("eBPF yüklemesi root gerektirir")
	}

	src, err := newEbpfAttrSource(AttrConfig{})
	if err != nil {
		t.Skipf("eBPF yüklenemedi (kernel/BTF/ortam): %v", err)
	}
	defer src.Stop()

	_, host := nonLoopbackIface(t)
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		_, _ = io.Copy(c, c) // echo
		_ = c.Close()
	}()

	const payload = 512 * 1024
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, payload)
	rdDone := make(chan struct{})
	go func() {
		_, _ = io.ReadFull(conn, buf)
		close(rdDone)
	}()
	if _, err := conn.Write(buf); err != nil {
		t.Fatal(err)
	}
	<-rdDone
	_ = conn.Close()

	// drain döngüsünün en az bir tam turu
	time.Sleep(ebpfDrainInterval + time.Second)

	var out, in uint64
	for _, d := range src.Deltas() {
		if d.Proto == "tcp" && d.RemoteIP == host {
			out += d.BytesOut
			in += d.BytesIn
		}
	}
	t.Logf("eBPF sayım (%s): out=%d in=%d (payload=%d, echo → ~2×)", host, out, in, payload)

	// echo: istemci payload yazar+okur, sunucu payload okur+yazar → her yön ~2×.
	// Çok gevşek alt sınır: en az bir payload'lık.
	if out < payload {
		t.Errorf("giden bayt beklenenden düşük: %d < %d", out, payload)
	}
	if in < payload {
		t.Errorf("gelen bayt beklenenden düşük: %d < %d", in, payload)
	}
}

// TestEBPFAttrSourceDNS, yerel bir sahte DNS değişimi üzerinden ringbuf →
// parseDNSNames → DNSDeltas yolunu doğrular. Sahte sunucu 127.0.0.1'de dinler;
// eBPF DNS filtresi loopback hedefi yakalar (Docker gömülü DNS senaryosu).
func TestEBPFAttrSourceDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("kısa mod")
	}
	if os.Geteuid() != 0 {
		t.Skip("root gerektirir")
	}
	src, err := newEbpfAttrSource(AttrConfig{})
	if err != nil {
		t.Skipf("eBPF yüklenemedi: %v", err)
	}
	defer src.Stop()

	srv, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	const domain = "probe.bazntms.test"
	resp := dnsPacket(t, domain, true, "1.2.3.4")
	query := dnsPacket(t, domain, false, "")
	go func() {
		b := make([]byte, 512)
		n, addr, e := srv.ReadFrom(b)
		if e != nil || n == 0 {
			return
		}
		_, _ = srv.WriteTo(resp, addr)
	}()

	c, err := net.Dial("udp", srv.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(query); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	if _, err := c.Read(buf); err != nil { // yanıtı oku → skb_consume_udp tetiklenir
		t.Fatalf("yanıt okunamadı: %v", err)
	}
	_ = c.Close()

	// dnsLoop asenkron — ringbuf olayını işlemesi için kısa bekleme
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, d := range src.DNSDeltas() {
			if d.Domain == domain {
				t.Logf("eBPF DNS: %s q=%d r=%d pid=%d proc=%q", d.Domain, d.Queries, d.Responses, d.PID, d.Process)
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("eBPF DNS: %q ringbuf üzerinden görülmedi", domain)
}

// TestEBPFAttrSourceL7, eBPF modundaki dar-filtreli yardımcı pcap handle'ın
// HTTP Host'u çıkarıp L7Deltas'a yansıttığını doğrular.
func TestEBPFAttrSourceL7(t *testing.T) {
	if testing.Short() {
		t.Skip("kısa mod")
	}
	if os.Geteuid() != 0 {
		t.Skip("root gerektirir")
	}
	_, host := nonLoopbackIface(t)

	// "any" pseudo-device — aynı-host trafiği tek bir fiziksel arayüzde
	// görünmeyebilir; üretimde autoIface() somut fiziksel arayüz verir.
	src, err := newEbpfAttrSource(AttrConfig{Iface: "any"})
	if err != nil {
		t.Skipf("eBPF yüklenemedi: %v", err)
	}
	defer src.Stop()
	if src.l7h == nil {
		t.Fatal("L7 yardımcı handle açılmadı (root + geçerli arayüz verildi)")
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(host, "8080"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		_, _ = io.Copy(io.Discard, c)
		_ = c.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	const wantHost = "probe.l7.bazntms.test"
	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: x\r\n\r\n", wantHost); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	_ = conn.Close()

	deadline := time.Now().Add(6 * time.Second) // helper refresh + pending retry
	for time.Now().Before(deadline) {
		for _, d := range src.L7Deltas() {
			if d.Host == wantHost && d.Kind == "http" {
				t.Logf("eBPF L7: host=%s kind=%s proc=%q remote=%s", d.Host, d.Kind, d.Process, d.RemoteIP)
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("eBPF L7: %q yardımcı handle üzerinden görülmedi", wantHost)
}
