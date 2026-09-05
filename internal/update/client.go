package update

// Agent tarafı guncelleme istemcisi: manifest kontrolu → surum karsilastirma
// → indirme → SHA-256 + ed25519 dogrulama → atomik degisim (Faz 7.3).

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Client struct {
	HubURL    string
	Channel   string
	PublicKey string // hex ed25519 (bos ise yalnizca sha256 dogrulanir)
	Token     string // agent token → Authorization: Bearer (bos ise gonderilmez)
	HTTP      *http.Client
}

// NewClient, guncelleme istemcisini kurar. token: agent'in enrollment
// token'i (uclar agentAuth arkasinda — bos gecilirse gercek hub 401 doner;
// yalnizca auth'suz test/mock hub icin bos birakilir). httpClient: agent'in
// enrollment sonrasi transport'u (pinli CA + varsa mTLS istemci sertifikasi);
// nil ise 10 dk timeout'lu duz client kullanilir. Bkz. docs/decisions/0001.
func NewClient(hubURL, channel, publicKey, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Minute}
	}
	return &Client{
		HubURL:    hubURL,
		Channel:   channel,
		PublicKey: publicKey,
		Token:     token,
		HTTP:      httpClient,
	}
}

// auth, istek varsa Bearer token ekler.
func (c *Client) auth(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

// Check, guncel surumu sorgular; yukselme varsa manifest dosya kaydini dondurur.
func (c *Client) Check(currentVersion string) (*Manifest, *ManifestFile, error) {
	url := fmt.Sprintf("%s/api/v1/agent/update/manifest?channel=%s", c.HubURL, c.Channel)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, nil // kanal sunulmuyor: guncelleme yok
	}
	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("manifest HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		return nil, nil, err
	}
	if CompareVersions(manifest.Version, currentVersion) <= 0 {
		return manifest, nil, nil // guncel
	}
	mf := manifest.FindFile(runtime.GOOS, runtime.GOARCH)
	if mf == nil {
		return manifest, nil, fmt.Errorf("manifest'te %s/%s dosyasi yok", runtime.GOOS, runtime.GOARCH)
	}
	return manifest, mf, nil
}

// Apply, guncellemeyi indirir, dogrular ve calisan binary'yi degistirir.
// Donus: degisim yapildi mi. Cagiran taraf basarili degisimden sonra
// sureci yeniden baslatmaktan sorumludur (exit → supervisor restart).
func (c *Client) Apply(currentVersion string) (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}
	if real, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = real
	}
	return c.ApplyTo(currentVersion, exePath)
}

// ApplyTo, Apply'nin hedef yol parametreli versiyonudur (test/ozel kullanim).
func (c *Client) ApplyTo(currentVersion, exePath string) (bool, error) {
	manifest, mf, err := c.Check(currentVersion)
	if err != nil {
		return false, err
	}
	if mf == nil {
		return false, nil
	}
	url := fmt.Sprintf("%s/api/v1/agent/update/file/%s/%s", c.HubURL, manifest.Channel, mf.Name)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("indirme HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "bazntms-update-*")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 1<<32)); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := VerifyFile(tmpPath, *mf, c.PublicKey); err != nil {
		return false, fmt.Errorf("dogrulama: %w", err)
	}

	if err := Install(tmpPath, exePath); err != nil {
		return false, err
	}
	return true, nil
}
