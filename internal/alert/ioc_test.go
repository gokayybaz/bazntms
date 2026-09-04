package alert

import (
	"strings"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

// stubMatcher, verilen domainleri (üst alan dahil) eşleştirir.
type stubMatcher struct{ bad []string }

func (s stubMatcher) Count() int { return len(s.bad) }
func (s stubMatcher) Match(d string) (string, bool) {
	d = strings.ToLower(d)
	for _, b := range s.bad {
		if d == b || strings.HasSuffix(d, "."+b) {
			return b, true
		}
	}
	return "", false
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

	m.SetIOC(stubMatcher{bad: []string{"evil-c2.example"}})
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

	m.SetIOC(stubMatcher{bad: []string{"bad.example"}})
	cfg := DefaultConfig()
	cfg.IOC.Enabled = false
	m.checkIOC(cfg)

	if evs, _ := m.st.RecentAlertEvents(10); len(evs) != 0 {
		t.Fatalf("IOC kapalıyken uyarı üretildi: %d", len(evs))
	}
}
