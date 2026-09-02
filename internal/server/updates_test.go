package server

// Faz 7.3: guncelleme kanali uclari testleri (agentAuth korumali, path
// guvenligi ve sunum davranisi).

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/store"
)

func newUpdateServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "upd.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stable", "manifest.json"), []byte(
		`{"channel":"stable","version":"v9.9.9","files":[{"name":"bazntms-agent-test","os":"linux","arch":"amd64","version":"v9.9.9","sha256":"abc","size":3,"signature":"ff"}]}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stable", "bazntms-agent-test"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	engine := capture.NewEngine()
	mgr := alert.NewManager(alert.DefaultConfig(), st, engine, 30)
	srv := New(nil, engine, st, "test.db", mgr, nil, "admin-pass-1", "", 30, false, nil, nil, nil)
	srv.SetUpdatesDir(dir)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	tok := "upd-agent-token"
	if _, err := st.RegisterAgent(store.Agent{
		Name: "upd-agent", TokenHash: TokenHashString(tok), ProtocolVersion: 1,
	}); err != nil {
		t.Fatalf("agent kaydi: %v", err)
	}
	return ts, dir, tok
}

func TestUpdatesSafeName(t *testing.T) {
	ok := []string{"bazntms-agent-linux-amd64", "bazntms-agent-windows-amd64.exe", "bazntms-agent_2.pkg"}
	bad := []string{"", "..", ".", "a/b", `.hidden`, `x\y`, `f;rm`}
	for _, n := range ok {
		if _, valid := updatesSafe(n); !valid {
			t.Errorf("reddedilmemeli: %q", n)
		}
	}
	for _, n := range bad {
		if _, valid := updatesSafe(n); valid {
			t.Errorf("kabul edilmemeli: %q", n)
		}
	}
}

func TestUpdateEndpointsAuthAndServing(t *testing.T) {
	ts, _, tok := newUpdateServer(t)

	// agent token'siz istek 401
	resp, err := http.Get(ts.URL + "/api/v1/agent/update/manifest?channel=stable")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token'siz 401 beklenirdi: %d", resp.StatusCode)
	}

	// gecerli token + kanal → manifest JSON
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/update/manifest?channel=stable", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest 200 beklenirdi: %d", resp.StatusCode)
	}

	// gecerli token + dosya
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/update/file/stable/bazntms-agent-test", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp2.Body.Read(buf)
	if string(buf[:n]) != "bin" {
		t.Fatalf("dosya icerigi: %q", buf[:n])
	}

	// eksik kanal → 404
	req3, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/update/manifest?channel=beta", nil)
	req3.Header.Set("Authorization", "Bearer "+tok)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("eksik kanal 404: %d", resp3.StatusCode)
	}

	// gecersiz dosya adi → 400
	req4, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/agent/update/file/stable/bogus;rm", nil)
	req4.Header.Set("Authorization", "Bearer "+tok)
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusBadRequest {
		t.Fatalf("gecersiz ad 400: %d", resp4.StatusCode)
	}
}
