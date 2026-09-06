package vault

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestEnvProvider(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// hex
	t.Setenv("BAZNTMS_VAULT_MASTER_KEY", hex.EncodeToString(key))
	v, err := OpenWith(NewEnvProvider("BAZNTMS_VAULT_MASTER_KEY"))
	if err != nil {
		t.Fatalf("hex env: %v", err)
	}
	enc, _ := v.Encrypt("sır")
	if got, _ := v.Decrypt(enc); got != "sır" {
		t.Fatalf("hex env round-trip: %q", got)
	}

	// base64 — aynı anahtar, aynı vault davranışı
	t.Setenv("BAZNTMS_VAULT_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	v2, err := OpenWith(NewEnvProvider("BAZNTMS_VAULT_MASTER_KEY"))
	if err != nil {
		t.Fatalf("base64 env: %v", err)
	}
	if got, _ := v2.Decrypt(enc); got != "sır" {
		t.Fatalf("base64 anahtar hex ile aynı olmalı: %q", got)
	}
}

func TestEnvProviderErrors(t *testing.T) {
	t.Setenv("BAZNTMS_VAULT_MASTER_KEY", "")
	if _, err := OpenWith(NewEnvProvider("BAZNTMS_VAULT_MASTER_KEY")); err == nil {
		t.Fatal("boş env → hata beklenirdi")
	}
	t.Setenv("BAZNTMS_VAULT_MASTER_KEY", "kısa")
	if _, err := OpenWith(NewEnvProvider("BAZNTMS_VAULT_MASTER_KEY")); err == nil {
		t.Fatal("geçersiz env → hata beklenirdi")
	}
}

func TestProviderFor(t *testing.T) {
	if p, _ := ProviderFor("", "/x/vault.key"); p.Name() != "file:/x/vault.key" {
		t.Fatalf("boş → file bekleniyordu: %s", p.Name())
	}
	if p, _ := ProviderFor("env", ""); p.Name() != "env:BAZNTMS_VAULT_MASTER_KEY" {
		t.Fatalf("env bekleniyordu: %s", p.Name())
	}
	if _, err := ProviderFor("kms", ""); err == nil {
		t.Fatal("bilinmeyen kaynak → hata beklenirdi")
	}
}
