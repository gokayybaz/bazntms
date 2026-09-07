package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// fakeGitHub, releases/latest + asset indirmelerini taklit eden bir sunucu kurar.
func fakeGitHub(t *testing.T, tag string, assets map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	var relAssets []map[string]any
	for name, body := range assets {
		relAssets = append(relAssets, map[string]any{
			"name":                 name,
			"size":                 len(body),
			"browser_download_url": srv.URL + "/dl/" + name,
		})
	}
	mux.HandleFunc("/repos/acme/widget/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"assets":   relAssets,
		})
	})
	for name, body := range assets {
		body := body
		mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, _ *http.Request) {
			w.Write(body)
		})
	}
	return srv
}

func TestGitHubSyncerSyncOnce(t *testing.T) {
	linuxBin := []byte("fake linux amd64 agent binary")
	winBin := []byte("fake windows agent binary .exe")
	assets := map[string][]byte{
		"bazntms-agent-linux-amd64":       linuxBin,
		"bazntms-agent-windows-amd64.exe": winBin,
		"bazntms-agent-amd64.msi":         []byte("msi — bu manifest'e girmemeli"),
	}
	srv := fakeGitHub(t, "v1.4.0", assets)
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldBase }()

	dir := t.TempDir()
	g := NewGitHubSyncer("acme/widget", dir, "")
	g.HTTP = srv.Client()

	changed, ver, err := g.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if !changed || ver != "v1.4.0" {
		t.Fatalf("changed=%v ver=%q, beklenen true/v1.4.0", changed, ver)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "stable", "manifest.json"))
	if err != nil {
		t.Fatalf("manifest okunamadı: %v", err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(m.Files) != 2 {
		t.Fatalf("manifest'te %d dosya, beklenen 2 (linux+windows; msi hariç)", len(m.Files))
	}

	lf := m.FindFile("linux", "amd64")
	if lf == nil {
		t.Fatal("linux/amd64 manifest'te yok")
	}
	want := sha256.Sum256(linuxBin)
	if lf.SHA256 != hex.EncodeToString(want[:]) || lf.Size != int64(len(linuxBin)) {
		t.Fatalf("linux sha/size uyuşmuyor: %+v", lf)
	}
	got, err := os.ReadFile(filepath.Join(dir, "stable", "bazntms-agent-linux-amd64"))
	if err != nil || string(got) != string(linuxBin) {
		t.Fatalf("indirilen linux binary yanlış: %v", err)
	}

	// ikinci çağrı: aynı sürüm → değişiklik yok
	changed, _, err = g.SyncOnce(context.Background())
	if err != nil || changed {
		t.Fatalf("ikinci SyncOnce changed=%v err=%v, beklenen false/nil", changed, err)
	}
}

// TestGitHubSyncerSignedManifest, release'te pipeline-imzalı manifest.json
// varsa syncer'ın imzaları geçirdiğini + SHA256'yı ona karşı doğruladığını,
// ve manifest'in kurcalandığı durumda SyncOnce'ın reddettiğini test eder.
func TestGitHubSyncerSignedManifest(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	linuxBin := []byte("imzalı kanal — linux amd64 binary")
	sum := sha256.Sum256(linuxBin)
	sumHex := hex.EncodeToString(sum[:])

	signedManifest := func(sha string) []byte {
		m := Manifest{Channel: "stable", Version: "v3.1.0", Files: []ManifestFile{{
			Name: "bazntms-agent-linux-amd64", OS: "linux", Arch: "amd64", Version: "v3.1.0",
			SHA256: sha, Size: int64(len(linuxBin)),
			Signature: hex.EncodeToString(ed25519.Sign(priv, []byte(sha))),
		}}}
		b, _ := json.Marshal(&m)
		return b
	}

	t.Run("geçerli imza geçirilir", func(t *testing.T) {
		srv := fakeGitHub(t, "v3.1.0", map[string][]byte{
			"bazntms-agent-linux-amd64": linuxBin,
			"manifest.json":             signedManifest(sumHex),
		})
		defer srv.Close()
		oldBase := apiBase
		apiBase = srv.URL
		defer func() { apiBase = oldBase }()

		dir := t.TempDir()
		g := NewGitHubSyncer("acme/widget", dir, "")
		g.HTTP = srv.Client()
		if _, _, err := g.SyncOnce(context.Background()); err != nil {
			t.Fatalf("SyncOnce: %v", err)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "stable", "manifest.json"))
		m, err := ParseManifest(raw)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := VerifyManifest(m, hex.EncodeToString(pub)); err != nil {
			t.Fatalf("üretilen manifest imzası doğrulanmadı: %v", err)
		}
	})

	t.Run("kurcalanmış manifest reddedilir", func(t *testing.T) {
		bad := sha256.Sum256([]byte("başka bir şey"))
		srv := fakeGitHub(t, "v3.1.0", map[string][]byte{
			"bazntms-agent-linux-amd64": linuxBin,
			"manifest.json":             signedManifest(hex.EncodeToString(bad[:])),
		})
		defer srv.Close()
		oldBase := apiBase
		apiBase = srv.URL
		defer func() { apiBase = oldBase }()

		g := NewGitHubSyncer("acme/widget", t.TempDir(), "")
		g.HTTP = srv.Client()
		if _, _, err := g.SyncOnce(context.Background()); err == nil {
			t.Fatal("SHA256 uyuşmazlığında hata beklenirdi")
		}
	})
}

func TestGitHubSyncerSizeMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/widget/releases/latest" {
			fmt.Fprintf(w, `{"tag_name":"v2.0.0","assets":[{"name":"bazntms-agent-linux-amd64","size":999,"browser_download_url":%q}]}`,
				"http://"+r.Host+"/dl")
			return
		}
		w.Write([]byte("kısa gövde")) // 999 değil
	}))
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldBase }()

	g := NewGitHubSyncer("acme/widget", t.TempDir(), "")
	g.HTTP = srv.Client()
	if _, _, err := g.SyncOnce(context.Background()); err == nil {
		t.Fatal("boyut uyuşmazlığında hata beklsenirdi")
	}
}
