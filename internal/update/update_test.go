package update

// Faz 7.3 guncelleme altyapisi testleri: surum karsilastirma, manifest
// dogrulamasi ve httptest hub uzerinden tam Apply akisi (indir → verify →
// degistir).

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.1", -1},
		{"1.2.3", "1.2.3", 0},
		{"2.0.0", "1.9.9", 1},
		{"v0.2.0", "v0.10.0", -1},    // numerik karsilastirma (lexik degil)
		{"dev", "v1.0.0", -1},        // numerik olmayan en dusuk
		{"v1.0.0-rc1", "v1.0.0", -1}, // on surum (rc) surumden dusuk
		{"1.2", "1.2.0", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestInstallSwap(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "agent")
	if err := os.WriteFile(exe, []byte("v1-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	newF := filepath.Join(dir, "new")
	if err := os.WriteFile(newF, []byte("v2-binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(newF, exe); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil || string(data) != "v2-binary" {
		t.Fatalf("degisim yok: %s %v", data, err)
	}
	if fi, _ := os.Stat(exe); fi.Mode()&0o111 == 0 {
		t.Fatal("yeni binary calistirilabilir degil")
	}
	CleanupOld(exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatal(".old kalintisi silinmedi")
	}
}

// TestApplyFlow, imza + manifest + hub taklidi ile tam guncelleme akisi.
func TestApplyFlow(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(pub)

	// "yeni surum" binary'si — testin calistigi platforma gore
	exeName := fmt.Sprintf("bazntms-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exe := filepath.Join(dir, exeName)
	payload := []byte("#!/bin/sh\necho new-agent\n")
	if err := os.WriteFile(exe, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum, size, err := FileSHA256(exe)
	if err != nil {
		t.Fatal(err)
	}
	sig := hex.EncodeToString(ed25519.Sign(priv, []byte(sum)))

	// imza dogrulama birim kontrolu
	if err := VerifySignature(sum, sig, pubHex); err != nil {
		t.Fatalf("imza: %v", err)
	}

	// hub taklidi: manifest + dosya sunumu
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/update/manifest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"channel":"stable","version":"v9.9.9","files":[{"name":%q,"os":%q,"arch":%q,"version":"v9.9.9","sha256":%q,"size":%d,"signature":%q}]}`,
			exeName, runtime.GOOS, runtime.GOARCH, sum, size, sig)
	})
	mux.HandleFunc("/api/v1/agent/update/file/stable/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, exe)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// "calisan agent" binary'si: eski surum
	current := filepath.Join(dir, "agent.current")
	if err := os.WriteFile(current, []byte("old-agent"), 0o755); err != nil {
		t.Fatal(err)
	}

	// tam akis: Check (yeni surum var) → indir → dogrula → degistir
	client := NewClient(srv.URL, "stable", pubHex)
	applied, err := client.ApplyTo("v1.0.0", current)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied {
		t.Fatal("guncelleme uygulanmadi")
	}
	data, err := os.ReadFile(current)
	if err != nil || string(data) != string(payload) {
		t.Fatalf("binary degismedi: %v", err)
	}

	// guncel surumde uygulama yok
	applied, err = client.ApplyTo("v9.9.9", current)
	if err != nil || applied {
		t.Fatalf("guncel surumde degisim: %v %v", applied, err)
	}

	// yanlis public key ile reddedilmeli
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	badClient := NewClient(srv.URL, "stable", hex.EncodeToString(otherPub))
	if _, err := badClient.ApplyTo("v1.0.0", current); err == nil {
		t.Fatal("yanlis imza kabul edildi")
	}
}
