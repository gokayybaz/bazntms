// bazntmsctl — bazNTMS yönetim CLI'ı (Faz 7).
//
// Komutlar:
//
//	bazntmsctl setup                       → etkileşimli hub kurulum sihirbazı (YAML üretir)
//	bazntmsctl update keygen -out DIR      → ed25519 güncelleme anahtar çifti üretir
//	bazntmsctl update sign -key SEED -out DIR -version vX -channel stable FILE...
//	                                       → binary'leri hash'ler/imzalar, manifest.json üretir
package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/update"
	"github.com/gokayybaz/bazntms/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "setup":
		if err := cmdSetup(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "hata:", err)
			os.Exit(1)
		}
	case "update":
		if err := cmdUpdate(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "hata:", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Printf("bazntmsctl %s\n", version.Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `bazntmsctl — bazNTMS yönetim aracı

Komutlar:
  setup                              etkileşimli hub kurulum sihirbazı (hub.yaml üretir)
  update keygen -out DIR             ed25519 güncelleme anahtar çifti üretir
  update sign -key SEED -out DIR -version vX.y.z -channel stable FILE...
                                     güncelleme dosyalarını imzalar ve manifest üretir
`)
}

// --- setup sihirbazi ---

func cmdSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	out := fs.String("out", "bazntms-hub.yml", "yazilacak config dosyasi")
	force := fs.Bool("force", false, "varsa dosyanin uzerine yaz")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*out); err == nil && !*force {
		return fmt.Errorf("%s zaten var — -force ile uzerine yazin", *out)
	}

	in := bufio.NewScanner(os.Stdin)
	ask := func(prompt, def string) string {
		fmt.Printf("%s [%s]: ", prompt, def)
		if !in.Scan() {
			return def
		}
		v := strings.TrimSpace(in.Text())
		if v == "" {
			return def
		}
		return v
	}

	fmt.Println("bazNTMS hub kurulum sihirbazı — Enter = varsayilan")
	fmt.Println()
	port := ask("HTTP portu", "8080")
	password := ask("Arayüz şifresi (boş = kimlik doğrulama kapalı, önerilmez)", "")
	dbPath := ask("Veri deposu: SQLite dosya yolu veya postgres:// DSN", "bazntms.db")
	nats := ask("NATS JetStream adresi (boş = kuyruk kapalı)", "")
	enroll := ask("Agent enrollment token (boş = rastgele üretilir)", "")
	updatesDir := ask("Güncelleme kanalı dizini (boş = kapalı; bkz. bazntmsctl update sign)", "")

	var b strings.Builder
	b.WriteString("# bazNTMS hub yapılandırması — bazntmsctl setup üretimi\n")
	b.WriteString(fmt.Sprintf("port: %s\n", port))
	b.WriteString("database:\n")
	b.WriteString(fmt.Sprintf("  path: %s\n", dbPath))
	if password != "" {
		b.WriteString("auth:\n")
		b.WriteString(fmt.Sprintf("  password: %q\n", password))
	}
	if nats != "" {
		b.WriteString("nats:\n")
		b.WriteString(fmt.Sprintf("  url: %s\n", nats))
	}
	if enroll != "" {
		b.WriteString("# enrollment token'i flag ile de gecilebilir: -enroll-token\n")
		b.WriteString(fmt.Sprintf("enroll_token: %q\n", enroll))
	}
	if updatesDir != "" {
		b.WriteString("updates:\n")
		b.WriteString(fmt.Sprintf("  dir: %s\n", updatesDir))
	}
	b.WriteString("log:\n  level: info\n  format: json\n")

	if err := os.WriteFile(*out, []byte(b.String()), 0o600); err != nil {
		return err
	}
	fmt.Printf("\n✔ %s yazıldı. Başlatma:\n  ./bazntms-hub -config %s\n", *out, *out)
	return nil
}

// --- update komutlari ---

func cmdUpdate(args []string) error {
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "keygen":
		return cmdUpdateKeygen(args[1:])
	case "sign":
		return cmdUpdateSign(args[1:])
	default:
		usage()
		os.Exit(2)
	}
	return nil
}

func cmdUpdateKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "updates/keys", "anahtar dosyalarinin dizini")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privPath := filepath.Join(*out, "seed.key")
	pubPath := filepath.Join(*out, "public.hex")
	if err := os.WriteFile(privPath, []byte(hex.EncodeToString(priv.Seed())), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte(hex.EncodeToString(pub)), 0o644); err != nil {
		return err
	}
	fmt.Printf("✔ gizli anahtar : %s (SIZDIR — imzalama makinesinde kalir)\n", privPath)
	fmt.Printf("✔ public key    : %s\n", pubPath)
	fmt.Printf("  agent'lara verilecek hex: %s\n", hex.EncodeToString(pub))
	return nil
}

// cmdUpdateSign, verilen dosyalari cikti dizinine kopyalar, SHA-256 + ed25519
// imzasi uretir ve manifest.json yazar. Dosya adi sozlesmesi:
// bazntms-agent-<os>-<arch>[.exe]
func cmdUpdateSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyPath := fs.String("key", "updates/keys/seed.key", "ed25519 seed dosyasi")
	out := fs.String("out", "updates/stable", "cikti dizini (updates/<channel>)")
	ver := fs.String("version", "", "surum (zorunlu, ex: v0.2.0)")
	channel := fs.String("channel", "stable", "kanal (stable|beta)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ver == "" {
		return fmt.Errorf("-version zorunlu")
	}
	files := fs.Args()
	if len(files) == 0 {
		return fmt.Errorf("imzalanacak dosya verin (bazntms-agent-<os>-<arch>)")
	}
	seedHex, err := os.ReadFile(*keyPath)
	if err != nil {
		return fmt.Errorf("anahtar okuma: %w", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(seedHex)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return fmt.Errorf("gizli anahtar gecersiz")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	manifest := update.Manifest{
		Channel:   *channel,
		Version:   *ver,
		CreatedAt: time.Now().Unix(),
	}
	for _, f := range files {
		name := filepath.Base(f)
		goos, goarch := parseAgentName(name)
		if goos == "" {
			return fmt.Errorf("%s: ad sozlesmesine uymuyor (bazntms-agent-<os>-<arch>)", name)
		}
		sum, size, err := update.FileSHA256(f)
		if err != nil {
			return err
		}
		sig := ed25519.Sign(priv, []byte(sum))
		dst := filepath.Join(*out, name)
		if err := copyFile(f, dst); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, update.ManifestFile{
			Name: name, OS: goos, Arch: goarch, Version: *ver,
			SHA256: sum, Size: size, Signature: hex.EncodeToString(sig),
		})
		fmt.Printf("✔ %-40s %s/%s sha256=%s…\n", name, goos, goarch, sum[:12])
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	mp := filepath.Join(*out, "manifest.json")
	if err := os.WriteFile(mp, raw, 0o644); err != nil {
		return err
	}
	fmt.Printf("✔ manifest: %s (kanal=%s surum=%s)\n", mp, *channel, *ver)
	fmt.Println("→ manifest.json + dosyaları hub'daki updates dizinine kopyalayın ve agent'lara public key'i dağıtın")
	return nil
}

// parseAgentName, "bazntms-agent-linux-amd64" → ("linux","amd64").
func parseAgentName(name string) (goos, goarch string) {
	n := strings.TrimSuffix(name, ".exe")
	if !strings.HasPrefix(n, "bazntms-agent-") {
		return "", ""
	}
	rest := strings.TrimPrefix(n, "bazntms-agent-")
	switch rest {
	case "windows-amd64":
		return "windows", "amd64"
	case "linux-amd64":
		return "linux", "amd64"
	case "linux-arm64":
		return "linux", "arm64"
	case "darwin-amd64":
		return "darwin", "amd64"
	case "darwin-arm64":
		return "darwin", "arm64"
	}
	return "", ""
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}
