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
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/compliance"
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
	case "verify":
		if err := cmdVerify(args[1:]); err != nil {
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
  verify -bundle FILE [-pubkey KEY]  5651 delil paketini offline doğrular
                                     (zincir + Merkle + TSA token + manifest imzası)
`)
}

// cmdVerify, delil paketini offline doğrular (Faz 9.4, A.5.28).
func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	bundlePath := fs.String("bundle", "", "delil paketi JSON dosyası (zorunlu)")
	pubPath := fs.String("pubkey", "", "manifest imza doğrulaması için ed25519 public key PEM")
	out := fs.String("out", "", "doğrulama raporu dosyası (boş → stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *bundlePath == "" {
		return fmt.Errorf("-bundle zorunlu")
	}
	data, err := os.ReadFile(*bundlePath)
	if err != nil {
		return err
	}
	var bundle compliance.EvidenceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("paket okuma: %w", err)
	}

	var pub ed25519.PublicKey
	if *pubPath != "" {
		pemData, err := os.ReadFile(*pubPath)
		if err != nil {
			return err
		}
		block, _ := pem.Decode(pemData)
		if block == nil || len(block.Bytes) != ed25519.PublicKeySize {
			return fmt.Errorf("public key PEM geçersiz")
		}
		pub = ed25519.PublicKey(block.Bytes)
	}

	rep := compliance.VerifyBundle(&bundle, pub)

	var b strings.Builder
	fmt.Fprintf(&b, "bazNTMS delil paketi doğrulaması\n")
	fmt.Fprintf(&b, "  paket        : %s\n", *bundlePath)
	fmt.Fprintf(&b, "  aralık       : %s .. %s\n",
		time.Unix(bundle.From, 0).Format("02.01.2006 15:04"),
		time.Unix(bundle.To, 0).Format("02.01.2006 15:04"))
	fmt.Fprintf(&b, "  kayıt sayısı : %d (doğrulanan: %d)\n", len(bundle.Logs), rep.CheckedRecords)
	fmt.Fprintf(&b, "  checkpoint   : %d\n", len(bundle.Checkpoints))
	fmt.Fprintf(&b, "  zincir       : %s\n", verdict(rep.OK && rep.BrokenSeq == 0, rep.BrokenSeq))
	fmt.Fprintf(&b, "  merkle       : %s\n", passFail(rep.MerkleOK))
	fmt.Fprintf(&b, "  günlük mühür : %s\n", passFail(rep.DailyOK))
	if rep.SigChecked {
		fmt.Fprintf(&b, "  imza         : %s\n", passFail(rep.SigOK))
	}
	for _, n := range rep.Notes {
		fmt.Fprintf(&b, "  • %s\n", n)
	}
	if rep.OK {
		fmt.Fprintln(&b, "\nSONUÇ: Paket bütünlüğü DOĞRULANDI ✓")
	} else {
		fmt.Fprintln(&b, "\nSONUÇ: Paket bütünlüğü DOĞRULANAMADI ✗")
	}

	if *out != "" {
		if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
			return err
		}
		fmt.Printf("rapor yazıldı: %s\n", *out)
	}
	fmt.Print(b.String())
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}

func verdict(ok bool, brokenSeq int64) string {
	if ok && brokenSeq == 0 {
		return "SAĞLAM ✓"
	}
	return fmt.Sprintf("BOZUK ✗ (kayıt #%d)", brokenSeq)
}

func passFail(ok bool) string {
	if ok {
		return "GEÇTİ ✓"
	}
	return "BAŞARISIZ ✗"
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
	enroll := ask("Bootstrap enrollment token (boş = rastgele üretilir; kalıcı token'ları panelden yönetin)", "")
	updatesDir := ask("Güncelleme kanalı dizini (boş = kapalı; bkz. bazntmsctl update sign)", "")

	var b strings.Builder
	b.WriteString("# bazNTMS hub yapılandırması — bazntmsctl setup üretimi\n")
	fmt.Fprintf(&b, "port: %s\n", port)
	b.WriteString("database:\n")
	fmt.Fprintf(&b, "  path: %s\n", dbPath)
	if password != "" {
		b.WriteString("auth:\n")
		fmt.Fprintf(&b, "  password: %q\n", password)
	}
	if nats != "" {
		b.WriteString("nats:\n")
		fmt.Fprintf(&b, "  url: %s\n", nats)
	}
	if enroll != "" {
		b.WriteString("# BOOTSTRAP token — yalnizca ilk kurulum icin. Sizarsa hub'i yeniden\n")
		b.WriteString("# baslatmadan iptal edilemez. Kalici, isimli, iptal-edilebilir token'lar\n")
		b.WriteString("# icin: panel > Yonetim > Agent Ekle (POST /api/v1/enroll-tokens).\n")
		fmt.Fprintf(&b, "enroll_token: %q\n", enroll)
	}
	if updatesDir != "" {
		b.WriteString("updates:\n")
		fmt.Fprintf(&b, "  dir: %s\n", updatesDir)
	}
	b.WriteString("log:\n  level: info\n  format: json\n")

	if err := os.WriteFile(*out, []byte(b.String()), 0o600); err != nil {
		return err
	}
	fmt.Printf("\n✔ %s yazıldı. Başlatma:\n  ./bazntms-hub -config %s\n", *out, *out)
	fmt.Println("\nAgent eklemek için: panel > Yönetim > Agent Ekle — isimli/süreli/")
	fmt.Println("iptal-edilebilir enrollment token'ı üretin. Yukarıdaki bootstrap token'ı")
	fmt.Println("yalnızca ilk kurulum içindir.")
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
