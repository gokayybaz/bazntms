package alert

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

var siemEv = store.AlertEvent{Ts: 1_700_000_000, Kind: "port", Key: "4444", Message: "Şüpheli porta bağlantı: 1.2.3.4:4444 (proc=nc)"}

func TestFormatCEF(t *testing.T) {
	s := formatCEF(siemEv)
	if !strings.HasPrefix(s, "CEF:0|bazNTMS|bazNTMS|1.0|port|") {
		t.Fatalf("CEF başlığı yanlış: %s", s)
	}
	if !strings.Contains(s, "|9|") { // port → severity 9
		t.Errorf("CEF severity 9 bekleniyordu: %s", s)
	}
	if !strings.Contains(s, "rt=1700000000000 ") || !strings.Contains(s, "cs1=4444") {
		t.Errorf("CEF uzantı alanları eksik: %s", s)
	}
	// '=' işareti mesajda kaçışlanmalı
	if !strings.Contains(s, `proc\=nc`) {
		t.Errorf("CEF uzantı kaçışı yapılmadı: %s", s)
	}
}

func TestFormatLEEF(t *testing.T) {
	s := formatLEEF(siemEv)
	if !strings.HasPrefix(s, "LEEF:1.0|bazNTMS|bazNTMS|1.0|port|") {
		t.Fatalf("LEEF başlığı yanlış: %s", s)
	}
	if !strings.Contains(s, "\tsev=9\t") || !strings.Contains(s, "\tcat=port\t") {
		t.Errorf("LEEF öznitelikleri eksik: %s", s)
	}
}

func TestSeverityMapping(t *testing.T) {
	cases := map[string][2]int{ // kind → {cef, syslog}
		"port": {9, 2}, "vpn_down": {8, 3}, "anomaly": {6, 4},
		"bw": {5, 4}, "proc": {4, 5}, "bilinmeyen": {5, 4},
	}
	for kind, want := range cases {
		if c := severityCEF(kind); c != want[0] {
			t.Errorf("%s: severityCEF=%d, %d bekleniyordu", kind, c, want[0])
		}
		if sl := severitySyslog(kind); sl != want[1] {
			t.Errorf("%s: severitySyslog=%d, %d bekleniyordu", kind, sl, want[1])
		}
	}
}

func TestDeliverSIEMSyslogUDP(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	deliverSIEM(SIEMConfig{
		Enabled: true, Format: "cef", Transport: "syslog-udp", Target: pc.LocalAddr().String(),
	}, siemEv)

	buf := make([]byte, 2048)
	pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("syslog paketi alınamadı: %v", err)
	}
	got := string(buf[:n])
	// <PRI> = 16*8 + 2 (port kritik) = 130
	if !strings.HasPrefix(got, "<130>") {
		t.Errorf("syslog PRI yanlış: %q", got)
	}
	if !strings.Contains(got, "bazntms: CEF:0|bazNTMS|") {
		t.Errorf("syslog gövdesi CEF içermiyor: %q", got)
	}
}

func TestDeliverSIEMSyslogTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	lines := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		lines <- line
	}()

	deliverSIEM(SIEMConfig{
		Enabled: true, Format: "leef", Transport: "syslog-tcp", Target: ln.Addr().String(),
	}, siemEv)

	select {
	case line := <-lines:
		if !strings.Contains(line, "LEEF:1.0|bazNTMS|") || !strings.HasSuffix(line, "\n") {
			t.Errorf("TCP syslog satırı yanlış: %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TCP syslog satırı gelmedi")
	}
}

func TestDeliverSIEMHTTPJSON(t *testing.T) {
	type req struct {
		auth  string
		ctype string
		body  string
	}
	got := make(chan req, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- req{r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(b)}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	deliverSIEM(SIEMConfig{
		Enabled: true, Format: "json", Transport: "http", Target: srv.URL, Token: "Splunk abc123",
	}, siemEv)

	select {
	case r := <-got:
		if r.auth != "Splunk abc123" {
			t.Errorf("Authorization başlığı yanlış: %q", r.auth)
		}
		if !strings.Contains(r.ctype, "application/json") {
			t.Errorf("Content-Type yanlış: %q", r.ctype)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(r.body), &m); err != nil {
			t.Fatalf("JSON gövde geçersiz: %v", err)
		}
		if m["kind"] != "port" || m["severity"].(float64) != 9 {
			t.Errorf("JSON alanları yanlış: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP isteği gelmedi")
	}
}

func TestDeliverSIEMTextSyslog(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	deliverSIEM(SIEMConfig{
		Enabled: true, Format: "text", Transport: "syslog-udp", Target: pc.LocalAddr().String(),
	}, siemEv)

	buf := make([]byte, 2048)
	pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("syslog paketi alınamadı: %v", err)
	}
	got := string(buf[:n])
	if strings.Contains(got, "CEF:") || strings.Contains(got, "LEEF:") {
		t.Errorf("düz metin bekleniyordu, yapılandırılmış format geldi: %q", got)
	}
	if !strings.Contains(got, "bazntms: bazNTMS [Şüpheli Port] Şüpheli porta") {
		t.Errorf("düz metin gövdesi yanlış: %q", got)
	}
}

func TestDeliverSIEMDisabled(t *testing.T) {
	// Enabled=false veya Target boş → hiçbir şey yapmamalı (panik/bağlantı yok)
	deliverSIEM(SIEMConfig{Enabled: false, Target: "127.0.0.1:1"}, siemEv)
	deliverSIEM(SIEMConfig{Enabled: true, Target: ""}, siemEv)
}
