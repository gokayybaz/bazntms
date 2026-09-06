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
	key, err := loadOrCreateKey(keyFile)
	if err != nil {
		return nil, err
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

// loadOrCreateKey, master key'i okur; yoksa yarışsız üretir (O_EXCL). İki
// controller replikası aynı volume'u paylaşıp aynı anda başlarsa yalnızca biri
// dosyayı yazar, diğeri onu okur (Faz 15 — panel HA).
func loadOrCreateKey(keyFile string) ([]byte, error) {
	if data, err := os.ReadFile(keyFile); err == nil {
		key, derr := hex.DecodeString(strings.TrimSpace(string(data)))
		if derr != nil {
			return nil, fmt.Errorf("key dosyasi hex degil: %w", derr)
		}
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		// başka bir süreç bizden önce yazdı — onun anahtarını oku
		data, rerr := os.ReadFile(keyFile)
		if rerr != nil {
			return nil, rerr
		}
		return hex.DecodeString(strings.TrimSpace(string(data)))
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.WriteString(hex.EncodeToString(key) + "\n"); err != nil {
		return nil, err
	}
	return key, nil
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
