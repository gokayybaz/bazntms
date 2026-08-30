package compliance

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func fromHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
