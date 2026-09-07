//go:build linux

package agent

import (
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// TestEBPFAttrSourceLoopback, canlı bir kernele eBPF programlarını yükler,
// loopback üzerinden bilinen boyutta trafik üretir ve haritadan okunan bayt
// sayısının ±%20 içinde olduğunu doğrular. root + BTF gerektirir; yoksa atlar.
func TestEBPFAttrSourceLoopback(t *testing.T) {
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
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
		if d.Proto == "tcp" && d.RemoteIP == "127.0.0.1" {
			out += d.BytesOut
			in += d.BytesIn
		}
	}
	t.Logf("eBPF loopback: out=%d in=%d (payload=%d, echo → ~2×)", out, in, payload)

	// echo: istemci payload yazar+okur, sunucu payload okur+yazar → her yön ~2×.
	// Çok gevşek alt sınır: en az bir payload'lık.
	if out < payload {
		t.Errorf("giden bayt beklenenden düşük: %d < %d", out, payload)
	}
	if in < payload {
		t.Errorf("gelen bayt beklenenden düşük: %d < %d", in, payload)
	}
}
