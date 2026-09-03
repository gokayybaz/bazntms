package pki

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"
)

// TestLoadOrCreateCARace, coklu hub replikasinin ayni paylasilan PKI dizinini
// es zamanli actigi senaryoyu taklit eder: hepsi AYNI CA sertifikasini almali.
func TestLoadOrCreateCARace(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	results := make([][]byte, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ca, err := LoadOrCreateCA(dir)
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
				return
			}
			results[i] = ca.CertPEM()
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if string(results[i]) != string(results[0]) || len(results[0]) == 0 {
			t.Fatalf("goroutine %d farkli/bos CA aldi", i)
		}
	}
}

func TestCARoundTrip(t *testing.T) {
	dir := t.TempDir()
	ca, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("CA üretilemedi: %v", err)
	}
	// ikinci çağrı diskten okumalı, aynı sertifika
	ca2, err := LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("CA yeniden okunamadı: %v", err)
	}
	if string(ca.CertPEM()) != string(ca2.CertPEM()) {
		t.Fatal("CA sertifikası yeniden okumada değişti")
	}

	// server cert: SAN'lar ve CA imzası doğrulanmalı
	srv, err := ca.ServerTLSCertificate([]string{"hub.example.com", "127.0.0.1"})
	if err != nil {
		t.Fatalf("server cert: %v", err)
	}
	if err := srv.Leaf.VerifyHostname("hub.example.com"); err != nil {
		t.Fatalf("SAN hub.example.com yok: %v", err)
	}
	roots := ca.Pool()
	if _, err := srv.Leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatalf("server cert CA'ya karşı doğrulanamadı: %v", err)
	}

	// agent CSR → istemci sertifikası
	_, csrPEM, err := GenerateAgentKeyCSR("iddia-edilen-ad")
	if err != nil {
		t.Fatalf("CSR: %v", err)
	}
	certPEM, notAfter, err := ca.SignAgentCSR(csrPEM, 42)
	if err != nil {
		t.Fatalf("CSR imzalanamadı: %v", err)
	}
	if time.Until(notAfter) < 80*24*time.Hour {
		t.Fatalf("istemci sertifikası ömrü çok kısa: %v", time.Until(notAfter))
	}
	blk, _ := pem.Decode(certPEM)
	b, _ := x509.ParseCertificate(blk.Bytes)
	if b.Subject.CommonName != "bazntms-agent-42" {
		t.Fatalf("CN hub tarafından belirlenmeliydi, gelen: %q", b.Subject.CommonName)
	}
	if AgentIDFromCN(b.Subject.CommonName) != 42 {
		t.Fatalf("AgentIDFromCN 42 dönmeliydi")
	}
	if _, err := b.Verify(x509.VerifyOptions{Roots: ca.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("istemci sertifikası doğrulanamadı: %v", err)
	}
}

func TestSignAgentCSRRejectsGarbage(t *testing.T) {
	ca, _ := LoadOrCreateCA(t.TempDir())
	if _, _, err := ca.SignAgentCSR([]byte("not a csr"), 1); err == nil {
		t.Fatal("bozuk CSR kabul edildi")
	}
}

func TestClientCertificateUsableInTLSConfig(t *testing.T) {
	ca, _ := LoadOrCreateCA(t.TempDir())
	keyPEM, csrPEM, _ := GenerateAgentKeyCSR("a1")
	certPEM, _, err := ca.SignAgentCSR(csrPEM, 7)
	if err != nil {
		t.Fatalf("imza: %v", err)
	}
	pair, err := ClientCertificate(keyPEM, certPEM)
	if err != nil {
		t.Fatalf("ClientCertificate: %v", err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{pair}, RootCAs: ca.Pool()}
	if len(cfg.Certificates) != 1 || cfg.Certificates[0].Leaf == nil {
		t.Fatal("tls.Certificate leaf çözülmedi")
	}
	if na, err := CertNotAfter(certPEM); err != nil || na.IsZero() {
		t.Fatalf("CertNotAfter: %v", err)
	}
}
