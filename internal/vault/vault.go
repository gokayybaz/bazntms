// Package vault, hassas kimlik bilgilerini AES-256-GCM ile sifreler.
// Master key: -vault-key-file ile verilen dosyada (32 bayt hex); yoksa
// ilk calistirmada otomatik uretilir ve 0600 ile yazilir.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Vault struct {
	gcm cipher.AEAD
}

// Open, master key dosyasini acar; yoksa uretir.
func Open(keyFile string) (*Vault, error) {
	var key []byte
	if data, err := os.ReadFile(keyFile); err == nil {
		key, err = hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("key dosyasi hex degil: %w", err)
		}
	} else {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
			return nil, err
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key 32 bayt olmali, gelen: %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{gcm: gcm}, nil
}

// Encrypt, "v1:<base64(nonce+ct)>" formatinda sifreli metin dondurur.
func (v *Vault) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := v.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt, Encrypt ciktisini cozer. v1 on eki yoksa duz metin kabul eder
// (eski veri uyumlulugu).
func (v *Vault) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	if !strings.HasPrefix(enc, "v1:") {
		return enc, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(enc, "v1:"))
	if err != nil {
		return "", err
	}
	if len(raw) < v.gcm.NonceSize() {
		return "", errors.New("sifreli veri cok kisa")
	}
	nonce, ct := raw[:v.gcm.NonceSize()], raw[v.gcm.NonceSize():]
	plain, err := v.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
