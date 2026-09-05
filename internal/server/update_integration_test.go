package server

// S11.6 (B2): update.Client'in agentAuth-korumali guncelleme uclarina karsi
// gercekten calistigini dogrular — hem Bearer token hem mTLS istemci
// sertifikasi yoluyla. Once auth'suz istek reddedilmeli.

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/agent"
	"github.com/gokayybaz/bazntms/internal/alert"
	"github.com/gokayybaz/bazntms/internal/capture"
	"github.com/gokayybaz/bazntms/internal/pki"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/update"
)

// writeSignedChannel, <dir>/stable altina imzali manifest + agent binary yazar,
// public key hex'ini dondurur.
func writeSignedChannel(t *testing.T, dir, newVersion string) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	chDir := filepath.Join(dir, "stable")
	if err := os.MkdirAll(chDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "bazntms-agent-" + runtime.GOOS + "-" + runtime.GOARCH
	payload := []byte("#!/bin/sh\necho updated " + newVersion + "\n")
	if err := os.WriteFile(filepath.Join(chDir, name), payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum, size, err := update.FileSHA256(filepath.Join(chDir, name))
	if err != nil {
		t.Fatal(err)
	}
	mf := update.Manifest{
		Channel: "stable", Version: newVersion,
		Files: []update.ManifestFile{{
			Name: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: newVersion,
			SHA256: sum, Size: size, Signature: hex.EncodeToString(ed25519.Sign(priv, []byte(sum))),
		}},
	}
	b, _ := json.Marshal(mf)
	if err := os.WriteFile(filepath.Join(chDir, "manifest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(pub)
}

func TestUpdateClientBearerIntegration(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "u.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	updDir := t.TempDir()
	pubHex := writeSignedChannel(t, updDir, "v9.9.9")

	srv := New(nil, capture.NewEngine(), st, "t.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", "", 30, false, nil, nil, nil)
	srv.SetUpdatesDir(updDir)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	tok := "int-agent-token"
	if _, err := st.RegisterAgent(store.Agent{Name: "a1", TokenHash: store.TokenHash(tok), ProtocolVersion: 1}); err != nil {
		t.Fatalf("agent kaydi: %v", err)
	}

	cur := filepath.Join(t.TempDir(), "agent.bin")
	if err := os.WriteFile(cur, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// token'siz → agentAuth reddeder, ApplyTo hata döner
	if applied, err := update.NewClient(ts.URL, "stable", pubHex, "", nil).ApplyTo("v1.0.0", cur); err == nil || applied {
		t.Fatalf("token'siz ApplyTo başarılı olmamalıydı: applied=%v err=%v", applied, err)
	}
	if b, _ := os.ReadFile(cur); string(b) != "old" {
		t.Fatal("token'siz denemede binary değişmiş olmamalı")
	}

	// token'lı → tam akış çalışır
	applied, err := update.NewClient(ts.URL, "stable", pubHex, tok, nil).ApplyTo("v1.0.0", cur)
	if err != nil || !applied {
		t.Fatalf("token'lı ApplyTo: applied=%v err=%v", applied, err)
	}
	if b, _ := os.ReadFile(cur); !strings.Contains(string(b), "updated v9.9.9") {
		t.Fatalf("binary güncellenmedi: %q", b)
	}
}

func TestUpdateClientMTLSIntegration(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ca, err := pki.LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatalf("ca: %v", err)
	}

	updDir := t.TempDir()
	pubHex := writeSignedChannel(t, updDir, "v9.9.9")

	srv := New(nil, capture.NewEngine(), st, "m.db",
		alert.NewManager(alert.DefaultConfig(), st, capture.NewEngine(), 30),
		nil, "", testEnrollToken, 30, false, nil, nil, nil)
	srv.SetAgentCA(ca)
	srv.SetUpdatesDir(updDir)

	serverCert, err := ca.ServerTLSCertificate([]string{"127.0.0.1", "localhost", "::1"})
	if err != nil {
		t.Fatalf("server cert: %v", err)
	}
	ts := httptest.NewUnstartedServer(srv.Handler())
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    ca.Pool(),
	}
	ts.StartTLS()
	t.Cleanup(ts.Close)

	// gerçek agent enroll → istemci sertifikası alır
	statePath := filepath.Join(t.TempDir(), "agent.state.json")
	c := agent.New(agent.Options{
		HubURLs: []string{ts.URL}, EnrollToken: testEnrollToken,
		Name: "mtls-upd-agent", StateFile: statePath,
	})
	agState, err := c.Enroll()
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	cur := filepath.Join(t.TempDir(), "agent.bin")
	if err := os.WriteFile(cur, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// sertifika taşımayan istemci (düz RootCAs) → agentAuth reddeder
	plain := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca.Pool()}}}
	if applied, err := update.NewClient(ts.URL, "stable", pubHex, "", plain).ApplyTo("v1.0.0", cur); err == nil || applied {
		t.Fatalf("sertifikasız ApplyTo başarılı olmamalıydı: applied=%v err=%v", applied, err)
	}

	// agent'ın kendi transport'u (mTLS) + boş token → sertifika ile kabul
	applied, err := update.NewClient(ts.URL, "stable", pubHex, agState.Token, c.UpdateHTTPClient()).ApplyTo("v1.0.0", cur)
	if err != nil || !applied {
		t.Fatalf("mTLS ApplyTo: applied=%v err=%v", applied, err)
	}
	if b, _ := os.ReadFile(cur); !strings.Contains(string(b), "updated v9.9.9") {
		t.Fatalf("binary güncellenmedi: %q", b)
	}
}
