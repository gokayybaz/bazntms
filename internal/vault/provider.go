package vault

// KeyProvider, master anahtarın kaynağını soyutlar (B8 / Faz 16 S16.3).
//
//   - file: -vault-key-file'daki 32 baytlık hex anahtar (yoksa üretilir).
//     Varsayılan; tek-node ve dev kurulumlar için.
//   - env:  BAZNTMS_VAULT_MASTER_KEY (hex veya base64, 32 bayt). Disk'e hiç
//     yazılmaz — anahtar bir bulut secret manager'ı (AWS Secrets Manager,
//     GCP Secret Manager), HashiCorp Vault agent'ı veya k8s Secret tarafından
//     ortam değişkeni olarak enjekte edilir. KMS entegrasyonunun pratik yolu:
//     bazNTMS bulut SDK'sı taşımaz.
//
// Zarf şifrelemesi (veri anahtarı DB'de, master rotasyonu re-encrypt'siz) ayrı
// bir adım — bkz. docs/decisions/0006-vault-key-provider.md.

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type KeyProvider interface {
	// Master, 32 baytlık AES-256 master anahtarını döndürür.
	Master() ([]byte, error)
	// Name, log/tanı için kaynak adı.
	Name() string
}

// --- file ---

type fileProvider struct{ path string }

// NewFileProvider, master'ı -vault-key-file'dan okur; yoksa yarışsız üretir.
func NewFileProvider(path string) KeyProvider { return &fileProvider{path: path} }

func (f *fileProvider) Name() string            { return "file:" + f.path }
func (f *fileProvider) Master() ([]byte, error) { return loadOrCreateKey(f.path) }

// --- env ---

type envProvider struct{ varName string }

// NewEnvProvider, master'ı bir ortam değişkeninden okur (hex veya base64).
func NewEnvProvider(varName string) KeyProvider { return &envProvider{varName: varName} }

func (e *envProvider) Name() string { return "env:" + e.varName }

func (e *envProvider) Master() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(e.varName))
	if raw == "" {
		return nil, fmt.Errorf("%s ortam değişkeni boş — env anahtar kaynağı için zorunlu", e.varName)
	}
	if key, err := hex.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	if key, err := base64.StdEncoding.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	return nil, fmt.Errorf("%s: 32 baytlık hex veya base64 master anahtarı bekleniyor", e.varName)
}

// ProviderFor, -vault-key-source değerine göre bir KeyProvider seçer.
func ProviderFor(source, keyFile string) (KeyProvider, error) {
	switch source {
	case "", "file":
		return NewFileProvider(keyFile), nil
	case "env":
		return NewEnvProvider("BAZNTMS_VAULT_MASTER_KEY"), nil
	default:
		return nil, fmt.Errorf("bilinmeyen -vault-key-source: %q (file | env)", source)
	}
}
