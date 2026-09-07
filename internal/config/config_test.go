package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "agent.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// collect.method verilmezse boş kalır (seçici "auto" sayar) — geriye uyum.
func TestAgentConfigCollectMethodDefault(t *testing.T) {
	cfg, err := LoadAgent(writeYAML(t, "collect:\n  pcap: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Collect.Method != "" {
		t.Fatalf("method boş beklenirdi, %q döndü", cfg.Collect.Method)
	}
	if !cfg.Collect.PCAP {
		t.Fatal("collect.pcap true okunmalıydı")
	}
}

func TestAgentConfigCollectMethodSet(t *testing.T) {
	cfg, err := LoadAgent(writeYAML(t, "collect:\n  method: ebpf\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Collect.Method != "ebpf" {
		t.Fatalf("method=ebpf beklenirdi, %q döndü", cfg.Collect.Method)
	}
}
