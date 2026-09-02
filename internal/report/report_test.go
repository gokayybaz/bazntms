package report

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

func seededStore(t *testing.T) store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	now := time.Now()
	// 3 gunluk ornekler
	for d := 0; d < 3; d++ {
		for h := 0; h < 24; h += 6 {
			ts := now.Add(-time.Duration(d) * 24 * time.Hour).Truncate(time.Hour).Add(time.Duration(h) * time.Hour).Unix()
			_ = st.InsertSample(store.Sample{
				Ts: ts, Device: "en0", BpsIn: 50_000_000, BpsOut: 10_000_000, Pps: 100,
				Protocols: map[string]uint64{"TCP": 800, "UDP": 200},
			})
		}
	}
	_ = st.InsertEndpointDeltas([]store.EndpointDelta{
		{Ts: now.Unix(), Device: "en0", IP: "142.250.151.119", Hostname: "google.example", BytesIn: 900 << 20, BytesOut: 5 << 20, Packets: 1000},
		{Ts: now.Unix(), Device: "en0", IP: "104.18.33.206", BytesIn: 400 << 20, BytesOut: 50 << 20, Packets: 500},
	})
	_ = st.InsertConnectionEvents([]store.ConnectionEvent{
		{Ts: now.Unix(), Proto: "tcp", LocalAddr: "a", RemoteAddr: "b", Process: "chrome", Count: 12},
	})
	_ = st.InsertDNSDeltas([]store.DNSDelta{
		{Ts: now.Unix(), Domain: "api.example.com", Queries: 42, Responses: 40},
	})
	_, _ = st.InsertAlertEvent(store.AlertEvent{Ts: now.Unix(), Kind: "bw", Key: "in", Message: "Test uyarisi: hiz zirve"})
	return st
}

func TestBuildReport(t *testing.T) {
	d, err := Build(seededStore(t), nil, 7)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if d.Samples == 0 || d.TotalGB <= 0 {
		t.Fatalf("ozet veriler bos: %+v", d)
	}
	if len(d.Daily) < 2 {
		t.Fatalf("gunluk veri eksik: %d", len(d.Daily))
	}
	if len(d.TopEndpoints) != 2 || len(d.TopProcesses) != 1 || len(d.TopDomains) != 1 {
		t.Fatalf("bolumler eksik: %d %d %d", len(d.TopEndpoints), len(d.TopProcesses), len(d.TopDomains))
	}
	if d.AlertCounts["bw"] != 1 {
		t.Fatalf("uyari sayaci hatali: %v", d.AlertCounts)
	}
	if len(d.Protocols) == 0 || d.Protocols[0].Name != "TCP" {
		t.Fatalf("protokol siralamasi hatali: %+v", d.Protocols)
	}
}

func TestRenderHTML(t *testing.T) {
	d, err := Build(seededStore(t), nil, 7)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	html, err := d.RenderHTML()
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	s := string(html)
	for _, want := range []string{"Ağ Trafik Raporu", "Yönetici Özeti", "142.250.151.119", "api.example.com", "chrome", "Test uyarisi"} {
		if !strings.Contains(s, want) {
			t.Fatalf("HTML'de eksik: %q", want)
		}
	}
}

func TestRenderPDF(t *testing.T) {
	d, err := Build(seededStore(t), nil, 7)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	pdf, err := d.RenderPDF()
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	if len(pdf) < 2000 {
		t.Fatalf("PDF cok kucuk: %d bayt", len(pdf))
	}
	if !strings.HasPrefix(string(pdf[:8]), "%PDF") {
		t.Fatalf("PDF imzasi yok: %q", string(pdf[:8]))
	}
	// fpdf UTF8 fontlari kullanilan karakterlerle subset embed eder; kucuk dosya normaldir
	if len(pdf) < 10_000 {
		t.Fatalf("PDF cok kucuk: %d bayt", len(pdf))
	}
}
