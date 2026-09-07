package update

// GitHub release senkronu: hub, bir GitHub deposunun en son yayınını (release)
// periyodik olarak çeker ve güncelleme kanalı dizinine (dir/<channel>/)
// manifest.json + agent binary'leri olarak yazar. Hub bu dizini olduğu gibi
// agent'lara sunar (internal/server/updates.go); agent tarafı akışı
// değişmez (manifest → sürüm karşılaştır → indir → SHA-256 → atomik değişim).
//
// İmza (ed25519): varsayılan YOK — güven zinciri "GitHub HTTPS + hub→agent
// pinli TLS" (agent zaten hub'a tam güvenir: config, yakalama politikası,
// mTLS CA). OPT-IN: release'te pipeline-imzalı bir `manifest.json` asset'i
// varsa (release.yml, `UPDATE_SIGNING_SEED` secret'ı ile `bazntmsctl update
// sign`) syncer onun imzalarını geçirir ve indirilen binary'lerin SHA256'sını
// imzalı manifest'e karşı doğrular. Tam statik/air-gapped kanal için
// -update-github-repo='' + elle hazırlanmış -updates-dir.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ghRelease / ghAsset, GitHub releases API'sinin kullandığımız alanları.
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ghTargets, hub'ın sunduğu (os, arch) → release asset adı eşlemesi.
// Adlar .github/workflows/release.yml'deki `go build -o` çıktılarıyla birebir.
var ghTargets = []struct{ OS, Arch, Asset string }{
	{"linux", "amd64", "bazntms-agent-linux-amd64"},
	{"linux", "arm64", "bazntms-agent-linux-arm64"},
	{"darwin", "amd64", "bazntms-agent-darwin-amd64"},
	{"darwin", "arm64", "bazntms-agent-darwin-arm64"},
	{"windows", "amd64", "bazntms-agent-windows-amd64.exe"},
}

// GitHubSyncer, tek bir depo + kanal için release senkronudur.
type GitHubSyncer struct {
	Repo    string // "owner/name"
	Dir     string // güncelleme kanalı kök dizini (hub -updates-dir)
	Channel string // "stable" (varsayılan)
	Token   string // opsiyonel GitHub API token'ı (rate limit için)
	HTTP    *http.Client
}

// NewGitHubSyncer, varsayılan 5 dk timeout'lu istemciyle bir syncer kurar.
func NewGitHubSyncer(repo, dir, token string) *GitHubSyncer {
	return &GitHubSyncer{
		Repo:    repo,
		Dir:     dir,
		Channel: "stable",
		Token:   token,
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (g *GitHubSyncer) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	return g.HTTP.Do(req)
}

// apiBase, testlerin api.github.com'u httptest sunucusuyla değiştirebilmesi
// için ayrı: boşsa gerçek GitHub.
var apiBase = "https://api.github.com"

// SyncOnce, en son release'i tek sefer senkron eder.
// changed: manifest.json bu çağrıda değişti mi (yeni sürüm indirildi mi).
func (g *GitHubSyncer) SyncOnce(ctx context.Context) (changed bool, version string, err error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, g.Repo)
	resp, err := g.get(ctx, url)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return false, "", fmt.Errorf("github releases HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return false, "", fmt.Errorf("release json: %w", err)
	}
	ver := strings.TrimSpace(rel.TagName)
	if ver == "" {
		return false, "", fmt.Errorf("release tag_name boş")
	}

	chDir := filepath.Join(g.Dir, g.Channel)

	// Mevcut manifest zaten bu sürümdeyse hiçbir şey indirme.
	if cur, e := os.ReadFile(filepath.Join(chDir, "manifest.json")); e == nil {
		if m, e2 := ParseManifest(cur); e2 == nil && CompareVersions(m.Version, ver) == 0 {
			return false, ver, nil
		}
	}
	if err := os.MkdirAll(chDir, 0o755); err != nil {
		return false, "", err
	}

	assets := make(map[string]ghAsset, len(rel.Assets))
	for _, a := range rel.Assets {
		assets[a.Name] = a
	}

	// Release'te pipeline-imzalı bir manifest.json varsa (bazntmsctl update sign,
	// release.yml opt-in) imzaları oradan al; yoksa imzasız manifest üret
	// (bugünkü davranış — güven: GitHub HTTPS + hub→agent pinli TLS).
	var signed *Manifest
	if a, ok := assets["manifest.json"]; ok {
		if raw, derr := g.getBody(ctx, a.BrowserDownloadURL); derr != nil {
			slog.Warn("release manifest.json indirilemedi, imzasız üretilecek", "err", derr)
		} else if sm, perr := ParseManifest(raw); perr != nil || CompareVersions(sm.Version, ver) != 0 {
			slog.Warn("release manifest.json kullanılamadı, imzasız üretilecek", "version", ver, "err", perr)
		} else {
			signed = sm
			slog.Info("pipeline-imzalı update manifest kullanılıyor", "version", ver)
		}
	}

	m := Manifest{Channel: g.Channel, Version: ver, CreatedAt: time.Now().Unix()}
	for _, t := range ghTargets {
		a, ok := assets[t.Asset]
		if !ok {
			slog.Warn("release'te agent asset'i yok, atlanıyor", "asset", t.Asset, "version", ver)
			continue
		}
		dest := filepath.Join(chDir, t.Asset)
		sum, size, derr := g.download(ctx, a, dest)
		if derr != nil {
			return false, "", fmt.Errorf("%s indirilemedi: %w", t.Asset, derr)
		}
		mf := ManifestFile{Name: t.Asset, OS: t.OS, Arch: t.Arch, Version: ver, SHA256: sum, Size: size}
		if signed != nil {
			sf := signed.FindFile(t.OS, t.Arch)
			if sf == nil || !strings.EqualFold(sf.SHA256, sum) || sf.Size != size {
				return false, "", fmt.Errorf("%s: imzalı manifest ile SHA256/boyut uyuşmuyor — release kurcalanmış olabilir", t.Asset)
			}
			mf.Signature = sf.Signature
		}
		m.Files = append(m.Files, mf)
	}
	if len(m.Files) == 0 {
		return false, "", fmt.Errorf("release %s içinde tanıdık agent asset'i yok", ver)
	}

	data, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return false, "", err
	}
	tmp := filepath.Join(chDir, ".manifest.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return false, "", err
	}
	if err := os.Rename(tmp, filepath.Join(chDir, "manifest.json")); err != nil {
		return false, "", err
	}
	return true, ver, nil
}

// getBody, küçük bir asset'i (manifest.json) belleğe indirir.
func (g *GitHubSyncer) getBody(ctx context.Context, url string) ([]byte, error) {
	resp, err := g.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// download, asset'i dest'e atomik yazar (temp + rename) ve SHA-256'sını döner.
func (g *GitHubSyncer) download(ctx context.Context, a ghAsset, dest string) (string, int64, error) {
	resp, err := g.get(ctx, a.BrowserDownloadURL)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".dl-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, 1<<31))
	if cErr := tmp.Close(); cErr != nil && err == nil {
		err = cErr
	}
	if err != nil {
		return "", 0, err
	}
	if a.Size > 0 && n != a.Size {
		return "", 0, fmt.Errorf("boyut %d != beklenen %d", n, a.Size)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Run, ilk senkronu hemen, sonrasını interval aralığıyla yapar; ctx iptal
// edilene kadar çalışır. Hata durumunda bir sonraki tur'da yeniden dener.
func (g *GitHubSyncer) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	for {
		changed, ver, err := g.SyncOnce(ctx)
		switch {
		case err != nil:
			slog.Warn("agent güncelleme senkronu başarısız", "repo", g.Repo, "err", err)
		case changed:
			slog.Info("agent güncelleme kanalı yenilendi", "repo", g.Repo, "channel", g.Channel, "version", ver)
		default:
			slog.Debug("agent güncelleme kanalı güncel", "version", ver)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
