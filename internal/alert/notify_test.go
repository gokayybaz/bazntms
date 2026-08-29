package alert

// Faz 6.3: webhook v2 imza dogrulamasi (HMAC-SHA256).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHmacSHA256Hex(t *testing.T) {
	secret := "webhook-secret"
	body := []byte(`{"version":2,"kind":"anomaly"}`)

	got := hmacSHA256Hex(secret, body)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("imza uyusmadi: %s != %s", got, want)
	}

	// yanlis secret farkli imza uretir
	if wrong := hmacSHA256Hex("baska-secret", body); wrong == got {
		t.Fatal("farkli secret ayni imzayi uretti")
	}
}
