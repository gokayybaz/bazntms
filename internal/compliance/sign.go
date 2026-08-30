package compliance

// Manifest imzası (Faz 9.2): günlük checkpoint manifest'i ed25519 ile
// imzalanır. Anahtar PEM dosyasında saklanır; yoksa üretilir ve public
// key'i ayrı dosyaya yazılır (verify'da kullanılır).
//
// Not: nitelikli e-imza (akıllı kart/P12) ileri adımdır; hukuki zaman
// referansı bu fazda RFC 3161 zaman damgasıdır.

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// LoadOrCreateSignKey, PEM ed25519 özel anahtarını yükler; dosya yoksa
// üretir ve public key'i yanına yazar. Dönüş: signer + public key hex.
func LoadOrCreateSignKey(path string) (ed25519.PrivateKey, string, error) {
	if path == "" {
		return nil, "", errors.New("imza anahtar dosyası belirtilmedi")
	}
	if data, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "ED25519 PRIVATE KEY" {
			return nil, "", fmt.Errorf("imza anahtar dosyası geçersiz: %s", path)
		}
		seed := block.Bytes
		if len(seed) != ed25519.SeedSize {
			return nil, "", fmt.Errorf("imza anahtarı boyut hatası: %d", len(seed))
		}
		priv := ed25519.NewKeyFromSeed(seed)
		return priv, pubHex(priv), nil
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: priv.Seed(),
	})
	if err := os.WriteFile(path, privPEM, 0o600); err != nil {
		return nil, "", err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: priv.Public().(ed25519.PublicKey),
	})
	if err := os.WriteFile(path+".pub", pubPEM, 0o644); err != nil {
		return nil, "", err
	}
	return priv, pubHex(priv), nil
}

func pubHex(priv ed25519.PrivateKey) string {
	pub := priv.Public().(ed25519.PublicKey)
	return fmt.Sprintf("%x", pub)
}

// SignManifest, manifest baytlarını imzalar (base64 std döner).
func SignManifest(priv ed25519.PrivateKey, manifest []byte) string {
	sig := ed25519.Sign(priv, manifest)
	return fmt.Sprintf("%x", sig)
}
