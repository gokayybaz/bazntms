// Package pki, agent↔hub karşılıklı TLS (mTLS) için küçük bir sertifika
// otoritesi (CA) sağlar: hub ilk çalıştırmada kendi CA'sını üretir, kendine
// bir sunucu sertifikası imzalar ve enrollment sırasında her agent'ın CSR'ını
// istemci sertifikasına dönüştürür.
//
// Anahtar tipi ECDSA P-256: sunucu sertifikası aynı hub portundan tarayıcıya
// da sunulduğu için evrensel destek şart (macOS Secure Transport / Safari
// ed25519 sunucu sertifikalarını doğrulamaz). Proje geneli ed25519 kullansa
// da (imza/güncelleme) TLS zinciri ECDSA'dır.
//
// Bu CA yalnızca agent kimlik doğrulaması içindir; tarayıcı/panel trafiği
// aynı CA'yı doğrulamaz (hub TLS'i VerifyClientCertIfGiven ile çalışır,
// istemci sertifikası opsiyoneldir).
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func newKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

const (
	caValidity     = 10 * 365 * 24 * time.Hour
	serverValidity = 397 * 24 * time.Hour // ~13 ay (tarayıcı üst sınırıyla uyumlu)
	// ClientValidity, agent istemci sertifikasının ömrü. Agent bunun yarısı
	// geçince yenilemeye çalışır (bkz. internal/agent).
	ClientValidity = 90 * 24 * time.Hour
	// ClientCNPrefix, istemci sertifikası CN'i: "<prefix><agentID>". Hub
	// middleware'i agent'ı bu CN'den çözer.
	ClientCNPrefix = "bazntms-agent-"
)

// CA, hub'ın sertifika otoritesidir.
type CA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
	dir     string
}

func serialNumber() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

// LoadOrCreateCA, `dir`/ca.crt + ca.key okur; yoksa yeni bir self-signed CA
// üretir ve 0600/0644 ile yazar. Coklu hub replikasi ayni paylasilan PKI
// dizinini kullanabildigi icin uretim yarissizdir: ca.key `O_EXCL` ile
// olusturulur, kaybeden replikalar kazananin yazdigi CA'yi okur (kisa bekleme).
func LoadOrCreateCA(dir string) (*CA, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	if ca, err := loadCA(certPath, keyPath, dir); err == nil {
		return ca, nil
	}

	// yaris kilidi: ca.key'i EXCL ile ac. Basarisiz → baska replika uretiyor.
	lockF, err := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			for i := 0; i < 100; i++ {
				time.Sleep(100 * time.Millisecond)
				if ca, err := loadCA(certPath, keyPath, dir); err == nil {
					return ca, nil
				}
			}
			return nil, fmt.Errorf("paylasilan CA 10 sn icinde hazir olmadi")
		}
		return nil, err
	}
	// bu noktadan sonra hata olursa kilit dosyasini birak (baska replika uretsin)
	cleanup := func() { lockF.Close(); os.Remove(keyPath) }

	priv, err := newKey()
	if err != nil {
		cleanup()
		return nil, err
	}
	serial, err := serialNumber()
	if err != nil {
		cleanup()
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "bazNTMS Agent CA", Organization: []string{"bazNTMS"}},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		cleanup()
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		cleanup()
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if _, err := lockF.Write(keyPEM); err != nil {
		cleanup()
		return nil, err
	}
	lockF.Close()
	// ca.crt en son yazilir: bekleyen replikalar bunun varligini bekler
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	return &CA{cert: cert, key: priv, certPEM: certPEM, dir: dir}, nil
}

// loadCA, ca.crt + ca.key diskte varsa CA'yi cozer.
func loadCA(certPath, keyPath, dir string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	cert, key, err := parseCertKey(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &CA{cert: cert, key: key, certPEM: certPEM, dir: dir}, nil
}

// CertPEM, CA sertifikasının PEM'ini döndürür (agent'lara dağıtmak için).
func (c *CA) CertPEM() []byte { return c.certPEM }

// Pool, CA sertifikasını içeren bir havuz döndürür (ClientCAs / RootCAs).
func (c *CA) Pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.cert)
	return p
}

// ServerTLSCertificate, `dir`/server.crt + server.key okur; yoksa `hosts`
// (DNS adları + IP'ler) için CA tarafından imzalı yeni bir sunucu
// sertifikası üretir. Süresi geçmiş sertifika yeniden üretilir.
func (c *CA) ServerTLSCertificate(hosts []string) (tls.Certificate, error) {
	certPath := filepath.Join(c.dir, "server.crt")
	keyPath := filepath.Join(c.dir, "server.key")

	if certPEM, err := os.ReadFile(certPath); err == nil {
		if keyPEM, err := os.ReadFile(keyPath); err == nil {
			if pair, err := tls.X509KeyPair(certPEM, keyPEM); err == nil {
				if leaf, err := x509.ParseCertificate(pair.Certificate[0]); err == nil {
					pair.Leaf = leaf
					if time.Now().Before(leaf.NotAfter.Add(-24 * time.Hour)) {
						return pair, nil
					}
				}
			}
		}
	}

	priv, err := newKey()
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := serialNumber()
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "bazNTMS hub", Organization: []string{"bazNTMS"}},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(serverValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, priv.Public(), c.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	_ = os.WriteFile(keyPath, keyPEM, 0o600)
	_ = os.WriteFile(certPath, certPEM, 0o644)

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	pair.Leaf, _ = x509.ParseCertificate(der)
	return pair, nil
}

// SignAgentCSR, agent'ın CSR'ını doğrular ve `bazntms-agent-<agentID>` CN'li,
// ClientAuth EKU'lu bir istemci sertifikası imzalar. CSR'daki CN/SAN yok
// sayılır — CN'i hub belirler (agent kimliğini agent iddia edemez).
func (c *CA) SignAgentCSR(csrPEM []byte, agentID int64) (certPEM []byte, notAfter time.Time, err error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || !strings.Contains(block.Type, "CERTIFICATE REQUEST") {
		return nil, time.Time{}, fmt.Errorf("geçersiz CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("CSR çözülemedi: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, time.Time{}, fmt.Errorf("CSR imzası geçersiz: %w", err)
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, time.Time{}, err
	}
	na := time.Now().Add(ClientValidity)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: fmt.Sprintf("%s%d", ClientCNPrefix, agentID)},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     na,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, csr.PublicKey, c.key)
	if err != nil {
		return nil, time.Time{}, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), na, nil
}

// AgentIDFromCN, "bazntms-agent-42" → 42. Eşleşmezse 0.
func AgentIDFromCN(cn string) int64 {
	if !strings.HasPrefix(cn, ClientCNPrefix) {
		return 0
	}
	var id int64
	if _, err := fmt.Sscanf(cn[len(ClientCNPrefix):], "%d", &id); err != nil {
		return 0
	}
	return id
}

func parseCertKey(certPEM, keyPEM []byte) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cb, _ := pem.Decode(certPEM)
	if cb == nil {
		return nil, nil, fmt.Errorf("ca.crt PEM değil")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, nil, err
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, nil, fmt.Errorf("ca.key PEM değil")
	}
	k, err := x509.ParsePKCS8PrivateKey(kb.Bytes)
	if err != nil {
		return nil, nil, err
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("ca.key ECDSA değil")
	}
	return cert, ec, nil
}
