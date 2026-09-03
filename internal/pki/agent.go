package pki

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"
)

// GenerateAgentKeyCSR, agent tarafında bir ECDSA P-256 özel anahtarı ve buna
// karşılık bir CSR üretir. CSR'daki CN yalnızca bilgi amaçlıdır; hub imzalarken
// gerçek CN'i (bazntms-agent-<id>) kendisi koyar. Dönüş: PKCS8 key PEM + CSR PEM.
func GenerateAgentKeyCSR(name string) (keyPEM, csrPEM []byte, err error) {
	priv, err := newKey()
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	if name == "" {
		name = "bazntms-agent"
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader,
		&x509.CertificateRequest{Subject: pkix.Name{CommonName: name}}, priv)
	if err != nil {
		return nil, nil, err
	}
	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	return keyPEM, csrPEM, nil
}

// ClientCertificate, key + cert PEM'inden bir tls.Certificate kurar ve leaf'i
// çözer (süre kontrolü için).
func ClientCertificate(keyPEM, certPEM []byte) (tls.Certificate, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	if len(pair.Certificate) > 0 {
		pair.Leaf, _ = x509.ParseCertificate(pair.Certificate[0])
	}
	return pair, nil
}

// CertNotAfter, PEM sertifikanın son geçerlilik anını döndürür.
func CertNotAfter(certPEM []byte) (time.Time, error) {
	b, _ := pem.Decode(certPEM)
	if b == nil {
		return time.Time{}, fmt.Errorf("cert PEM değil")
	}
	c, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return c.NotAfter, nil
}

// PoolFromPEM, bir CA sertifika PEM'inden RootCAs havuzu kurar.
func PoolFromPEM(caPEM []byte) (*x509.CertPool, error) {
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("CA PEM havuza eklenemedi")
	}
	return p, nil
}
