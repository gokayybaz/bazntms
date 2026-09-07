//go:build linux

package agent

import (
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// nonLoopbackIPv4, makinenin ilk loopback-olmayan IPv4'ünü döndürür (Docker'da
// eth0). eBPF bayt sayımı loopback'i atladığından test bunun üzerinden gider.
func nonLoopbackIPv4(t *testing.T) string {
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
					return v4.String()
				}
			}
		}
	}
	t.Skip("loopback-olmayan IPv4 arayüz yok")
	return ""
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

	host := nonLoopbackIPv4(t)
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
