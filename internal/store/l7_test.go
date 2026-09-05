package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gokayybaz/bazntms/pkg/telemetry"
)

func TestL7SaveAndTop(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "l7.db"))
	if err != nil {
		t.Fatalf("acilamadi: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now().Unix()

	if err := st.SaveL7(1, now, []telemetry.L7Sample{
		{PID: 10, Process: "chrome", Kind: "tls", Host: "api.github.com", RemoteIP: "140.82.1.1", Bytes: 500, Count: 3},
		{PID: 10, Process: "chrome", Kind: "tls", Host: "www.google.com", RemoteIP: "142.250.0.1", Bytes: 200, Count: 1},
		{PID: 5, Process: "curl", Kind: "http", Host: "example.com", Bytes: 100, Count: 1},
		{PID: 0, Process: "x", Kind: "tls", Host: "", Count: 9}, // host bos → atlanir
	}); err != nil {
		t.Fatalf("SaveL7: %v", err)
	}
	// ikinci agent ayni host
	if err := st.SaveL7(2, now, []telemetry.L7Sample{
		{PID: 1, Process: "firefox", Kind: "tls", Host: "api.github.com", Bytes: 50, Count: 2},
	}); err != nil {
		t.Fatalf("SaveL7 2: %v", err)
	}

	top, err := st.TopL7(time.Now().Add(-time.Hour), 0, 10, "")
	if err != nil {
		t.Fatalf("TopL7: %v", err)
	}
	// host+kind+process bazli grup: (api.github.com,tls,chrome), (api.github.com,tls,firefox), ...
	var github *L7Usage
	for i := range top {
		if top[i].Host == "api.github.com" && top[i].Process == "chrome" {
			github = &top[i]
		}
	}
	if github == nil || github.Hits != 3 || github.AgentCnt != 1 {
		t.Fatalf("api.github.com/chrome hatali: %+v", github)
	}
	for _, u := range top {
		if u.Host == "" {
			t.Fatal("bos host TopL7'ye girmemeliydi")
		}
	}

	// agent filtresi
	byAgent, _ := st.TopL7(time.Now().Add(-time.Hour), 2, 10, "")
	if len(byAgent) != 1 || byAgent[0].Process != "firefox" {
		t.Fatalf("agent filtresi hatali: %+v", byAgent)
	}
}
