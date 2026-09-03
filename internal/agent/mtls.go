package agent

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gokayybaz/bazntms/internal/pki"
)

// mTLS dosyalari state dosyasinin yaninda tutulur:
//
//	<state>.crt  agent istemci sertifikasi (hub CA'sinca imzali)
//	<state>.key  istemci ozel anahtari (PKCS8 ed25519)
//	<state>.ca   hub CA sertifikasi (pinlenmis — hub'i dogrulamak icin)
func (c *Client) certPath() string { return c.opts.StateFile + ".crt" }
func (c *Client) keyPath() string  { return c.opts.StateFile + ".key" }
func (c *Client) caPath() string   { return c.opts.StateFile + ".ca" }

// caPEM, hub'i dogrulamak icin kullanilacak CA'yi dondurur: once -hub-ca
// dosyasi, sonra pinlenmis <state>.ca. Ikisi de yoksa nil (ilk hello TOFU).
func (c *Client) caPEM() []byte {
	if c.opts.HubCAFile != "" {
		if b, err := os.ReadFile(c.opts.HubCAFile); err == nil {
			return b
		}
		slog.Warn("hub CA dosyasi okunamadi", "dosya", c.opts.HubCAFile)
	}
	if b, err := os.ReadFile(c.caPath()); err == nil {
		return b
	}
	return nil
}

// reloadTLS, diskteki sertifika/anahtar/CA'ya gore c.http transport'unu
// yeniden kurar. Sertifika yoksa mTLS kapali; CA varsa yine de hub dogrulanir.
func (c *Client) reloadTLS() {
	caPEM := c.caPEM()
	certPEM, certErr := os.ReadFile(c.certPath())
	keyPEM, keyErr := os.ReadFile(c.keyPath())

	if caPEM == nil && (certErr != nil || keyErr != nil) {
		return // pinlenecek CA da, istemci sertifikasi da yok → duz http
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caPEM != nil {
		if pool, err := pki.PoolFromPEM(caPEM); err == nil {
			tlsCfg.RootCAs = pool
		}
	}
	c.mtls = false
	if certErr == nil && keyErr == nil {
		if pair, err := pki.ClientCertificate(keyPEM, certPEM); err == nil {
			tlsCfg.Certificates = []tls.Certificate{pair}
			c.mtls = true
			if pair.Leaf != nil {
				c.certEnd = pair.Leaf.NotAfter
			}
		} else {
			slog.Warn("istemci sertifikasi yuklenemedi", "err", err)
		}
	}
	c.http = &http.Client{
		Timeout:   c.opts.HTTPTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
}

// bootstrapClient, ilk hello icin kullanilan gecici istemci: pinlenmis CA
// yoksa TOFU (InsecureSkipVerify) — enrollment token zaten kimlik saglar,
// donen CA bir sonraki baglantidan itibaren pinlenir.
func (c *Client) bootstrapClient() *http.Client {
	if caPEM := c.caPEM(); caPEM != nil {
		if pool, err := pki.PoolFromPEM(caPEM); err == nil {
			return &http.Client{
				Timeout:   c.opts.HTTPTimeout,
				Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}},
			}
		}
	}
	slog.Warn("hub CA pinli degil — ilk baglantida sunucu dogrulanmiyor (TOFU); -hub-ca ile onceden saglayin")
	return &http.Client{
		Timeout:   c.opts.HTTPTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, //nolint:gosec // TOFU bootstrap; donen CA pinlenir
	}
}

// saveCerts, hub'dan gelen istemci sertifikasi + CA'yi diske yazar ve
// transport'u yeniden kurar.
func (c *Client) saveCerts(keyPEM []byte, clientCertPEM, caCertPEM string) {
	if keyPEM != nil {
		_ = os.WriteFile(c.keyPath(), keyPEM, 0o600)
	}
	if clientCertPEM != "" {
		_ = os.WriteFile(c.certPath(), []byte(clientCertPEM), 0o600)
	}
	if caCertPEM != "" {
		_ = os.WriteFile(c.caPath(), []byte(caCertPEM), 0o644)
	}
	c.reloadTLS()
}

// certNeedsRenewal, istemci sertifikasi omrunun yarisindan fazlasi gectiyse
// true doner (yenileme icin).
func (c *Client) certNeedsRenewal() bool {
	if !c.mtls || c.certEnd.IsZero() {
		return false
	}
	return time.Until(c.certEnd) < pki.ClientValidity/2
}
