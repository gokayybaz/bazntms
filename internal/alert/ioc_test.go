package alert

import (
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/threatintel"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// stubProvider, verilen domain/IP'leri (üst alan dahil) malicious işaretler.
type stubProvider struct{ bad []string }

func (stubProvider) Name() string { return "stub" }
func (s stubProvider) Lookup(indicator, typ string) (threatintel.Indicator, bool) {
	d := strings.ToLower(indicator)
	for _, b := range s.bad {
		if d == b || strings.HasSuffix(d, "."+b) {
			return threatintel.Indicator{Reputation: threatintel.Malicious, Confidence: 90, Source: "stub", RawRef: b}, true
		}
	}
	return threatintel.Indicator{}, false
}

func stubTI(bad ...string) *threatintel.Service {
	return threatintel.New(time.Minute, stubProvider{bad: bad})
}

func TestCheckIOC(t *testing.T) {
	m, st := newTestManager(t)
	a1, _ := st.RegisterAgent(store.Agent{Name: "ws-01", TokenHash: "h1"})
	now := time.Now().Unix()

	if err := st.SaveL7(a1, now, []telemetry.L7Sample{
		{PID: 9, Process: "powershell", Kind: "tls", Host: "cdn.evil-c2.example", Bytes: 10, Count: 1},
		{PID: 9, Process: "powershell", Kind: "tls", Host: "www.microsoft.com", Bytes: 99, Count: 5},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveAgentDNS(a1, now, []telemetry.DNSSample{
		{PID: 9, Process: "powershell", Domain: "evil-c2.example", Queries: 4},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()

	// eşleştirici yokken hiçbir şey olmamalı
	m.checkIOC(cfg)
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("matcher yokken uyarı üretildi: %d", len(evs))
	}

	m.SetThreatIntel(stubTI("evil-c2.example"))
	m.checkIOC(cfg)

	evs, _ := m.st.RecentAlertEvents(10)
	if len(evs) != 2 { // l7 host + dns domain — ayrı (agent|domain) anahtarları değil ama farklı kaynak → aynı key
		// key = agentID|domain; l7 "cdn.evil-c2.example" ve dns "evil-c2.example" farklı domain → 2 olay
		t.Fatalf("2 IOC olayı beklenirdi (l7 alt alan + dns kök), gelen: %d — %+v", len(evs), evs)
	}
	for _, e := range evs {
		if e.Kind != "ioc" {
			t.Errorf("kind = %q", e.Kind)
		}
		if !strings.Contains(e.Message, "ws-01") || !strings.Contains(e.Message, "powershell") {
			t.Errorf("mesaj eksik atıf: %q", e.Message)
		}
	}

	// microsoft.com eşleşmemeli → sayı artmadı
	m.checkIOC(cfg) // cooldown zaten tutuyor ama yine de
	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 2 {
		t.Fatalf("temiz domain uyarı üretti / cooldown çalışmadı: %d", len(evs))
	}
}

func TestCheckIOCDisabled(t *testing.T) {
	m, st := newTestManager(t)
	a1, _ := st.RegisterAgent(store.Agent{Name: "x", TokenHash: "h"})
	st.SaveAgentDNS(a1, time.Now().Unix(), []telemetry.DNSSample{{Process: "p", Domain: "bad.example", Queries: 1}})

	m.SetThreatIntel(stubTI("bad.example"))
	cfg := DefaultConfig()
	cfg.IOC.Enabled = false
	m.checkIOC(cfg)

	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("IOC kapalıyken uyarı üretildi: %d", len(evs))
	}
}
