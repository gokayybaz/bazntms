package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestAgentDNSSaveAndTop(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "adns.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now().Unix()

	if err := st.SaveAgentDNS(1, now, []telemetry.DNSSample{
		{PID: 10, Process: "chrome", Domain: "www.google.com", Queries: 5, Responses: 5},
		{PID: 10, Process: "chrome", Domain: "api.github.com", Queries: 2, Responses: 2},
		{PID: 0, Process: "x", Domain: "", Queries: 9}, // domain bos → atlanir
	}); err != nil {
		t.Fatalf("SaveAgentDNS: %v", err)
	}
	if err := st.SaveAgentDNS(2, now, []telemetry.DNSSample{
		{PID: 1, Process: "firefox", Domain: "www.google.com", Queries: 3, Responses: 3},
	}); err != nil {
		t.Fatalf("SaveAgentDNS 2: %v", err)
	}

	top, err := st.TopAgentDNS(time.Now().Add(-time.Hour), 0, 10, "")
	if err != nil {
		t.Fatalf("TopAgentDNS: %v", err)
	}
	// www.google.com hem chrome hem firefox'ta → iki satır (process bazlı grup)
	var chromeG *AgentDNSUsage
	for i := range top {
		if top[i].Domain == "www.google.com" && top[i].Process == "chrome" {
			chromeG = &top[i]
		}
		if top[i].Domain == "" {
			t.Fatal("boş domain TopAgentDNS'e girmemeliydi")
		}
	}
	if chromeG == nil || chromeG.Queries != 5 || chromeG.AgentCnt != 1 {
		t.Fatalf("www.google.com/chrome hatalı: %+v", chromeG)
	}

	// agent filtresi
	byAgent, _ := st.TopAgentDNS(time.Now().Add(-time.Hour), 2, 10, "")
	if len(byAgent) != 1 || byAgent[0].Process != "firefox" {
		t.Fatalf("agent filtresi hatalı: %+v", byAgent)
	}
}
