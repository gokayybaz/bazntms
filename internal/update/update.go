// Package update, Faz 7.3 imza dogrulamali agent guncelleme kanalidir.
//
// Akis:
//   - bazntmsctl update keygen  → ed25519 anahtar cifti uretir
//   - bazntmsctl update sign    → binary'leri hash'ler, imzalar, manifest
//     uretir (updates/<channel>/manifest.json)
//   - hub, manifest + dosyaları /api/v1/agent/update/* uzerinden sunar
//   - agent periyodik kontrol eder: surum yukseldiyse indirir, SHA-256 ve
//     ed25519 imzasini dogrular, kendi binary'sini atomik degistirir
package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ManifestFile, manifest.json'daki tek dosya kaydidir.
type ManifestFile struct {
	Name      string `json:"name"` // dosya adi (bazntms-agent-linux-amd64)
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Version   string `json:"version"`
	SHA256    string `json:"sha256"` // hex
	Size      int64  `json:"size"`
	Signature string `json:"signature"` // ed25519 imzasi (hex) — imzalanan: SHA256 hex metni
}

// Manifest, kanal surum bildirimidir.
type Manifest struct {
	Channel   string         `json:"channel"`
	Version   string         `json:"version"`
	CreatedAt int64          `json:"created_at"`
	Files     []ManifestFile `json:"files"`
}

// ParseManifest, manifest JSON'ini cözumler.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if m.Version == "" || len(m.Files) == 0 {
		return nil, fmt.Errorf("manifest gecersiz: version/files bos")
	}
	return &m, nil
}

// FindFile, manifest'ten istenen os/arch dosyasini dondurur.
func (m *Manifest) FindFile(goos, goarch string) *ManifestFile {
	for i := range m.Files {
		if m.Files[i].OS == goos && m.Files[i].Arch == goarch {
			return &m.Files[i]
		}
	}
	return nil
}

// CompareVersions, dotted numerik surum karsilastirmasi: -1,0,1. "v" on eki
// ve numerik olmayan sonekler (dev, rc1) tolere edilir: numerik olmayan
// parcalar en dusuk onceliktir.
func CompareVersions(a, b string) int {
	pa, pb := splitVersion(a), splitVersion(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va, vb := 0, 0
		na, nb := true, true
		if i < len(pa) {
			var err error
			va, err = strconv.Atoi(pa[i])
			na = err == nil
		}
		if i < len(pb) {
			var err error
			vb, err = strconv.Atoi(pb[i])
			nb = err == nil
		}
		// numerik olmayan (dev/rc) parcalar numeriklerden kucuk kabul edilir
		switch {
		case na && nb:
			if va != vb {
				if va < vb {
					return -1
				}
				return 1
			}
		case na != nb:
			if nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

func splitVersion(v string) []string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return []string{"0"}
	}
	// sonekleri (rc1, dev) ayir: "1.2.3-rc1" → ["1","2","3","rc1"]
	parts := strings.SplitN(v, "-", 2)
	nums := strings.Split(parts[0], ".")
	if len(parts) == 2 {
		nums = append(nums, parts[1])
	}
	return nums
}

// FileSHA256, dosyanin sha256 hex ozetini hesaplar.
func FileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// VerifySignature, imzayi dogrular: imza sha256 hex METNI uzerindendir.
func VerifySignature(sha256Hex, sigHex, publicKeyHex string) error {
	if sigHex == "" || publicKeyHex == "" {
		return fmt.Errorf("imza veya public key yok")
	}
	pub, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("public key gecersiz")
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("imza gecersiz")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), []byte(sha256Hex), sig) {
		return fmt.Errorf("imza dogrulanmadi")
	}
	return nil
}

// VerifyFile, indirilen dosyanin ozet + imza dogrulamasini yapar.
func VerifyFile(path string, mf ManifestFile, publicKeyHex string) error {
	sum, size, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if size != mf.Size {
		return fmt.Errorf("boyut eslesmedi: %d != %d", size, mf.Size)
	}
	if !strings.EqualFold(sum, mf.SHA256) {
		return fmt.Errorf("sha256 eslesmedi")
	}
	if publicKeyHex != "" {
		return VerifySignature(mf.SHA256, mf.Signature, publicKeyHex)
	}
	return nil
}

// Install, dogrulanan dosyayi calisan binary ile degistirir.
// Unix: dogrudan atomik rename. Windows: calisan exe yeniden adlandirilir
// (taşınabilir), yeni dosya yerine konur; .old dosyasi sonraki acilista silinir.
func Install(newFile, exePath string) error {
	info, err := os.Stat(newFile)
	if err != nil {
		return err
	}
	if err := os.Chmod(newFile, info.Mode()|0o111); err != nil {
		return err
	}
	old := exePath + ".old"
	_ = os.Remove(old) // onceki degisimden kalinti olabilir
	if err := os.Rename(exePath, old); err != nil {
		return fmt.Errorf("eski binary tasima: %w", err)
	}
	if err := os.Rename(newFile, exePath); err != nil {
		_ = os.Rename(old, exePath) // en iyi cabayla geri al
		return fmt.Errorf("yeni binary yerlestirme: %w", err)
	}
	return nil
}

// CleanupOld, degisim sonrasi kalinti .old dosyasini siler (Windows uyumu).
func CleanupOld(exePath string) {
	_ = os.Remove(exePath + ".old")
}
