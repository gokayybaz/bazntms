package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gokayybaz/bazntms/internal/compliance"
)

// withStdin, verilen metni os.Stdin olarak sağlar ve testin sonunda eski
// os.Stdin'i geri yükler (cmdSetup gibi doğrudan os.Stdin okuyan fonksiyonları
// pipe ile beslemek için).
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })
	go func() {
		_, _ = w.WriteString(input)
		_ = w.Close()
	}()
}

// captureStdout, fn çalışırken os.Stdout'a yazılanları yakalar.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	os.Stdout = orig
	_ = w.Close()
	out := <-done
	return out
}

func TestParseAgentName(t *testing.T) {
	cases := []struct {
		name       string
		wantGOOS   string
		wantGOARCH string
	}{
		{"bazntms-agent-windows-amd64.exe", "windows", "amd64"},
		{"bazntms-agent-linux-amd64", "linux", "amd64"},
		{"bazntms-agent-linux-arm64", "linux", "arm64"},
		{"bazntms-agent-darwin-amd64", "darwin", "amd64"},
		{"bazntms-agent-darwin-arm64", "darwin", "arm64"},
		{"bazntms-agent-freebsd-amd64", "", ""}, // desteklenmeyen platform
		{"bazntms-hub-linux-amd64", "", ""},     // yanlış önek
		// .exe soneki her zaman kırpılır (yalnızca windows'a özgü değil) —
		// zararsız, elle çalıştırıldığında dosya kopyalanırken eklenmiş
		// olabilir.
		{"bazntms-agent-linux-amd64.exe", "linux", "amd64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			goos, goarch := parseAgentName(c.name)
			if goos != c.wantGOOS || goarch != c.wantGOARCH {
				t.Fatalf("parseAgentName(%q) = (%q,%q), want (%q,%q)", c.name, goos, goarch, c.wantGOOS, c.wantGOARCH)
			}
		})
	}
}

func TestVerdictAndPassFail(t *testing.T) {
	if got := verdict(true, 0); got != "SAĞLAM ✓" {
		t.Fatalf("verdict(true,0) = %q", got)
	}
	if got := verdict(false, 42); !strings.Contains(got, "#42") {
		t.Fatalf("verdict(false,42) = %q, beklenen kayıt #42 içermeli", got)
	}
	if got := passFail(true); got != "GEÇTİ ✓" {
		t.Fatalf("passFail(true) = %q", got)
	}
	if got := passFail(false); got != "BAŞARISIZ ✗" {
		t.Fatalf("passFail(false) = %q", got)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	content := []byte("bazNTMS test içeriği")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("kopyalanan içerik farklı: %q != %q", got, content)
	}
}

// TestUpdateKeygenSignVerifyRoundTrip — keygen → sign → verify tam döngüsü,
// ardından tahrif edilmiş binary'nin reddedildiğini doğrular.
func TestUpdateKeygenSignVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")
	if err := cmdUpdateKeygen([]string{"-out", keysDir}); err != nil {
		t.Fatalf("keygen: %v", err)
	}
	seedPath := filepath.Join(keysDir, "seed.key")
	pubPath := filepath.Join(keysDir, "public.hex")
	if _, err := os.Stat(seedPath); err != nil {
		t.Fatalf("seed.key üretilmedi: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public.hex üretilmedi: %v", err)
	}

	agentBin := filepath.Join(dir, "bazntms-agent-linux-amd64")
	if err := os.WriteFile(agentBin, []byte("sahte agent binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "stable")
	if err := cmdUpdateSign([]string{
		"-key", seedPath, "-out", outDir, "-version", "v9.9.9", "-channel", "stable", agentBin,
	}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest.json üretilmedi: %v", err)
	}

	out := captureStdout(t, func() {
		if err := cmdUpdateVerify([]string{"-pubkey", pubPath, manifestPath}); err != nil {
			t.Fatalf("verify (geçerli manifest): %v", err)
		}
	})
	if !strings.Contains(out, "manifest imzası geçerli") {
		t.Fatalf("çıktıda başarı mesajı yok: %q", out)
	}
	if !strings.Contains(out, "SHA256 + boyut eşleşti") {
		t.Fatalf("çıktıda dosya doğrulaması yok: %q", out)
	}

	// tahrif edilmiş binary → SHA256 uyuşmazlığı
	if err := os.WriteFile(filepath.Join(outDir, "bazntms-agent-linux-amd64"), []byte("kurcalanmis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmdUpdateVerify([]string{"-pubkey", pubPath, manifestPath}); err == nil {
		t.Fatal("tahrif edilmiş binary sessizce kabul edildi, hata bekleniyordu")
	}

	// yanlış public key → imza doğrulaması başarısız
	if err := cmdUpdateKeygen([]string{"-out", filepath.Join(dir, "otherkeys")}); err != nil {
		t.Fatal(err)
	}
	wrongPub := filepath.Join(dir, "otherkeys", "public.hex")
	if err := cmdUpdateVerify([]string{"-pubkey", wrongPub, manifestPath}); err == nil {
		t.Fatal("yanlış public key ile imza doğrulaması geçti, hata bekleniyordu")
	}
}

func TestCmdVerifyEvidenceBundle(t *testing.T) {
	dir := t.TempDir()

	t.Run("bundle zorunlu", func(t *testing.T) {
		if err := cmdVerify(nil); err == nil {
			t.Fatal("-bundle verilmeden hata bekleniyordu")
		}
	})

	t.Run("var olmayan dosya", func(t *testing.T) {
		if err := cmdVerify([]string{"-bundle", filepath.Join(dir, "yok.json")}); err == nil {
			t.Fatal("var olmayan dosya için hata bekleniyordu")
		}
	})

	t.Run("bozuk JSON", func(t *testing.T) {
		bad := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(bad, []byte("{ gecersiz"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := cmdVerify([]string{"-bundle", bad}); err == nil {
			t.Fatal("bozuk JSON için hata bekleniyordu")
		}
	})

	t.Run("boş ama geçerli paket", func(t *testing.T) {
		bundle := &compliance.EvidenceBundle{From: 1000, To: 2000}
		raw, err := compliance.BundleToJSON(bundle)
		if err != nil {
			t.Fatal(err)
		}
		bundlePath := filepath.Join(dir, "evidence.json")
		if err := os.WriteFile(bundlePath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		reportPath := filepath.Join(dir, "report.txt")

		out := captureStdout(t, func() {
			if err := cmdVerify([]string{"-bundle", bundlePath, "-out", reportPath}); err != nil {
				t.Fatalf("geçerli boş paket doğrulaması başarısız: %v", err)
			}
		})
		if !strings.Contains(out, "DOĞRULANDI") {
			t.Fatalf("çıktıda başarı sonucu yok: %q", out)
		}
		report, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatalf("-out raporu yazılmadı: %v", err)
		}
		if !strings.Contains(string(report), "DOĞRULANDI") {
			t.Fatalf("rapor dosyası başarı içermiyor: %q", report)
		}
	})
}

func TestCmdSetupExistingFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hub.yml")
	if err := os.WriteFile(out, []byte("mevcut"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdSetup([]string{"-out", out}); err == nil {
		t.Fatal("dosya zaten varken -force olmadan hata bekleniyordu")
	}
}

func TestCmdSetupInteractiveDefaults(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hub.yml")

	// 6 prompt (port, sifre, db, nats, enroll, updatesDir) — hepsi bos =
	// varsayilanlar kullanilir.
	withStdin(t, "\n\n\n\n\n\n")
	captureStdout(t, func() {
		if err := cmdSetup([]string{"-out", out}); err != nil {
			t.Fatalf("cmdSetup: %v", err)
		}
	})

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("config yazılmadı: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "port: 8080") {
		t.Errorf("varsayılan port yok:\n%s", s)
	}
	if !strings.Contains(s, "path: bazntms.db") {
		t.Errorf("varsayılan db yolu yok:\n%s", s)
	}
	if strings.Contains(s, "auth:") {
		t.Errorf("şifre boşken auth bloğu yazılmamalı:\n%s", s)
	}
	if strings.Contains(s, "nats:") || strings.Contains(s, "enroll_token") || strings.Contains(s, "updates:") {
		t.Errorf("boş yanıtlar için opsiyonel bloklar yazılmamalı:\n%s", s)
	}
}

func TestCmdSetupInteractiveCustomAnswers(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hub.yml")

	answers := strings.Join([]string{
		"9090",             // port
		"s3cr3t",           // arayüz şifresi
		"postgres://x/db",  // db
		"nats://localhost", // nats
		"boot-token-123",   // enroll
		"/var/lib/updates", // updates dir
	}, "\n") + "\n"
	withStdin(t, answers)
	captureStdout(t, func() {
		if err := cmdSetup([]string{"-out", out}); err != nil {
			t.Fatalf("cmdSetup: %v", err)
		}
	})

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("config yazılmadı: %v", err)
	}
	s := string(got)
	for _, want := range []string{
		"port: 9090",
		`password: "s3cr3t"`,
		"path: postgres://x/db",
		"url: nats://localhost",
		`enroll_token: "boot-token-123"`,
		"dir: /var/lib/updates",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("beklenen satır yok (%q):\n%s", want, s)
		}
	}

	// dosya izinleri: sırlar içerebileceği için 0600 olmalı. Windows'ta Go'nun
	// os paketi POSIX izin bitlerini gerçek NTFS ACL'lerine çeviremiyor —
	// os.WriteFile(...,0o600) orada yalnızca "salt-okunur değil" bayrağına
	// indirgenir ve Stat().Mode().Perm() her zaman 0666/0444 döner (bkz.
	// internal/update/update_test.go'daki aynı desen). Gerçek tek-kullanıcı
	// kısıtlaması Windows'ta ancak ACL/DACL ile mümkün — bu kod tabanında
	// henüz yok, bu yüzden kontrol yalnızca POSIX platformlarında anlamlı.
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(out)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("hub.yml izinleri 0600 değil: %v", fi.Mode().Perm())
		}
	}
}

func TestCmdSetupForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hub.yml")
	if err := os.WriteFile(out, []byte("eski icerik"), 0o644); err != nil {
		t.Fatal(err)
	}
	withStdin(t, "\n\n\n\n\n\n")
	captureStdout(t, func() {
		if err := cmdSetup([]string{"-out", out, "-force"}); err != nil {
			t.Fatalf("-force ile üzerine yazma başarısız: %v", err)
		}
	})
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "eski icerik") {
		t.Fatal("-force ile dosya üzerine yazılmadı")
	}
}
