package report

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// seededStore, filo modeline gore doldurulmus bir SQLite store dondurur:
// bir agent'in ~36 saatlik kumulatif arayuz ornekleri + surec trafigi +
// NetFlow + bir uyari olayi.
func seededStore(t *testing.T) store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	aid, err := st.RegisterAgent(store.Agent{Name: "test-agent", Site: "hq", TokenHash: "hash-1", Version: "test"})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}

	// 36 saat, 10 dk aralikli kumulatif sayaclar: ~8 Mbit/s in, ~2 Mbit/s out
	const step = 10 * time.Minute
	base := time.Now().Add(-36 * time.Hour).Truncate(time.Hour)
	var rx, tx, rxp, txp uint64
	for ts := base; ts.Before(time.Now()); ts = ts.Add(step) {
		secs := uint64(step.Seconds())
		rx += 8_000_000 / 8 * secs
		tx += 2_000_000 / 8 * secs
		rxp += 90 * secs
		txp += 20 * secs
		if err := st.SaveIfaceSamples(aid, ts.Unix(), []telemetry.InterfaceSample{
			{Name: "eth0", RxBytes: rx, TxBytes: tx, RxPackets: rxp, TxPackets: txp},
		}); err != nil {
			t.Fatalf("iface ornegi: %v", err)
		}
	}

	now := time.Now()
	if err := st.SaveProcessTraffic(aid, now.Add(-time.Hour).Unix(), []telemetry.ProcessTrafficSample{
		{PID: 1, Process: "chrome", Proto: "tcp", RemoteIP: "142.250.151.119", Port: 443, BytesIn: 900 << 20, BytesOut: 5 << 20},
		{PID: 2, Process: "curl", Proto: "tcp", RemoteIP: "104.18.33.206", Port: 443, BytesIn: 50 << 20, BytesOut: 2 << 20},
	}); err != nil {
		t.Fatalf("surec trafigi: %v", err)
	}

	if err := st.SaveFlows([]store.FlowRow{
		{Ts: now.Add(-30 * time.Minute).Unix(), Device: "gw", Src: "10.0.0.5", Dst: "142.250.151.119", SrcPort: 40000, DstPort: 443, Proto: "tcp", Packets: 1000, Octets: 900 << 20},
		{Ts: now.Add(-30 * time.Minute).Unix(), Device: "gw", Src: "10.0.0.6", Dst: "104.18.33.206", SrcPort: 40001, DstPort: 443, Proto: "udp", Packets: 500, Octets: 400 << 20},
	}); err != nil {
		t.Fatalf("flows: %v", err)
	}

	if _, err := st.InsertAlertEvent(store.AlertEvent{Ts: now.Unix(), Kind: "bw", Key: "in", Message: "Test uyarisi: hiz zirve"}); err != nil {
		t.Fatalf("uyari: %v", err)
	}
	return st
}

func TestBuildReport(t *testing.T) {
	d, err := Build(seededStore(t), nil, 7)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if d.Sources == 0 || d.TotalGB <= 0 {
		t.Fatalf("ozet veriler bos: %+v", d)
	}
	if d.AvgInBps <= 0 || d.AvgOutBps <= 0 {
		t.Fatalf("ortalama verim hesaplanmadi: in=%v out=%v", d.AvgInBps, d.AvgOutBps)
	}
	if len(d.Daily) < 2 {
		t.Fatalf("gunluk veri eksik: %d", len(d.Daily))
	}
	if len(d.Agents) != 1 {
		t.Fatalf("agent filosu eksik: %d", len(d.Agents))
	}
	if len(d.TopEndpoints) == 0 {
		t.Fatalf("hedefler bos")
	}
	if len(d.TopProcesses) != 2 {
		t.Fatalf("surecler eksik: %d", len(d.TopProcesses))
	}
	if d.AlertCounts["bw"] != 1 {
		t.Fatalf("uyari sayaci hatali: %v", d.AlertCounts)
	}
	if len(d.Protocols) == 0 || d.Protocols[0].Name != "TCP" {
		t.Fatalf("protokol siralamasi hatali: %+v", d.Protocols)
	}
	if d.Empty {
		t.Fatalf("Empty true olmamaliydi")
	}
}

func TestBuildReportEmpty(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	d, err := Build(st, nil, 7)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !d.Empty {
		t.Fatalf("bos store icin Empty true bekleniyordu: %+v", d)
	}
	html, err := d.RenderHTML()
	if err != nil {
		t.Fatalf("html: %v", err)
	}
	if !strings.Contains(string(html), "filo trafiği kaydı bulunamadı") {
		t.Fatalf("bos durum bandi yok")
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
	for _, want := range []string{"Ağ Trafik Raporu", "Yönetici Özeti", "Agent Filosu", "test-agent", "142.250.151.119", "chrome", "Süreç Bazlı Trafik", "Test uyarisi"} {
		if !strings.Contains(s, want) {
			t.Fatalf("HTML'de eksik: %q", want)
		}
	}
	if strings.Contains(s, "DNS Sorguları") {
		t.Fatalf("DNS bolumu kaldirilmaliydi")
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
	if !strings.HasPrefix(string(pdf[:8]), "%PDF") {
		t.Fatalf("PDF imzasi yok: %q", string(pdf[:8]))
	}
	if len(pdf) < 10_000 {
		t.Fatalf("PDF cok kucuk: %d bayt", len(pdf))
	}
}

func TestBuildEnterprise(t *testing.T) {
	d, err := BuildEnterprise(seededStore(t), 30)
	if err != nil {
		t.Fatalf("build enterprise: %v", err)
	}
	if d.AgentTotal != 1 {
		t.Fatalf("agent sayisi: %d", d.AgentTotal)
	}
	if d.TotalGB <= 0 || d.AvgBps <= 0 {
		t.Fatalf("kapasite hesaplanmadi: %+v", d)
	}
	if d.P95Bps <= 0 {
		t.Fatalf("banding percentile hesaplanmadi: %+v", d)
	}
	if len(d.TopEndpoints) == 0 || len(d.TopProcesses) == 0 {
		t.Fatalf("top listeler bos")
	}
	html, err := d.RenderEnterpriseHTML()
	if err != nil {
		t.Fatalf("enterprise html: %v", err)
	}
	if !strings.Contains(string(html), "Kurumsal Rapor") || !strings.Contains(string(html), "Süreç Bazlı Trafik") {
		t.Fatalf("enterprise HTML govdesi beklenenden farkli")
	}
}
