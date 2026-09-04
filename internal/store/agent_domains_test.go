package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestRecentAgentDomains(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "ad.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	a1, _ := st.RegisterAgent(Agent{Name: "ws-01", TokenHash: "h1"})
	a2, _ := st.RegisterAgent(Agent{Name: "ws-02", TokenHash: "h2"})
	now := time.Now().Unix()

	if err := st.SaveL7(a1, now, []telemetry.L7Sample{
		{PID: 10, Process: "chrome", Kind: "tls", Host: "evil.example", Bytes: 100, Count: 2},
		{PID: 10, Process: "chrome", Kind: "tls", Host: "", Count: 5}, // bos → atlanir
	}); err != nil {
		t.Fatalf("SaveL7: %v", err)
	}
	if err := st.SaveAgentDNS(a1, now, []telemetry.DNSSample{
		{PID: 10, Process: "chrome", Domain: "evil.example", Queries: 1},
		{PID: 22, Process: "svchost", Domain: "cdn.good.example", Queries: 3},
	}); err != nil {
		t.Fatalf("SaveAgentDNS: %v", err)
	}
	if err := st.SaveAgentDNS(a2, now, []telemetry.DNSSample{
		{PID: 1, Process: "firefox", Domain: "another.bad.example", Queries: 1},
	}); err != nil {
		t.Fatalf("SaveAgentDNS 2: %v", err)
	}
	// pencere disinda — gelmemelı
	if err := st.SaveAgentDNS(a2, now-9999, []telemetry.DNSSample{
		{PID: 1, Process: "firefox", Domain: "old.example", Queries: 1},
	}); err != nil {
		t.Fatalf("SaveAgentDNS eski: %v", err)
	}

	rows, err := st.RecentAgentDomains(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("RecentAgentDomains: %v", err)
	}

	type key struct {
		agent          int64
		domain, source string
	}
	got := map[key]AgentDomainSeen{}
	for _, r := range rows {
		got[key{r.AgentID, r.Domain, r.Source}] = r
	}

	if _, ok := got[key{a1, "evil.example", "l7"}]; !ok {
		t.Error("a1 l7 evil.example eksik")
	}
	if _, ok := got[key{a1, "evil.example", "dns"}]; !ok {
		t.Error("a1 dns evil.example eksik")
	}
	if r, ok := got[key{a1, "cdn.good.example", "dns"}]; !ok || r.AgentName != "ws-01" || r.Process != "svchost" {
		t.Errorf("a1 dns cdn.good.example yanlış: %+v ok=%v", r, ok)
	}
	if _, ok := got[key{a2, "another.bad.example", "dns"}]; !ok {
		t.Error("a2 dns another.bad.example eksik")
	}
	if _, ok := got[key{a2, "old.example", "dns"}]; ok {
		t.Error("pencere dışındaki domain geldi")
	}
	for _, r := range rows {
		if r.Domain == "" {
			t.Error("boş domain satırı döndü")
		}
	}
}
