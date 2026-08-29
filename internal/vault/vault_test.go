package vault

import "testing"

func TestVaultRoundTrip(t *testing.T) {
	dir := t.TempDir()
	v, err := Open(dir + "/vault.key")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	enc, err := v.Encrypt("gizli-community")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "gizli-community" || len(enc) < 20 {
		t.Fatalf("sifreli metin beklentiyi karsilamadi: %s", enc)
	}
	got, err := v.Decrypt(enc)
	if err != nil || got != "gizli-community" {
		t.Fatalf("decrypt: %v %q", err, got)
	}
	// bos deger saydam gecer
	if e2, _ := v.Encrypt(""); e2 != "" {
		t.Fatal("bos deger sifrelenmemeli")
	}
}

func TestVaultKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	v1, _ := Open(dir + "/vault.key")
	enc, _ := v1.Encrypt("veri")
	v2, _ := Open(dir + "/vault.key") // ayni key ile yeniden ac
	got, err := v2.Decrypt(enc)
	if err != nil || got != "veri" {
		t.Fatalf("key persistence bozuk: %v %q", err, got)
	}
}

func TestVaultTamper(t *testing.T) {
	dir := t.TempDir()
	v, _ := Open(dir + "/vault.key")
	enc, _ := v.Encrypt("veri")
	tampered := enc[:len(enc)-4] + "AAAA"
	if _, err := v.Decrypt(tampered); err == nil {
		t.Fatal("bozulmus veri hata vermeliydi")
	}
}
