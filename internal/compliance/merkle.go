// Package compliance, Faz 9.1/9.2: 5651 uyumlu log bütünlüğü motoru.
//
// İki katmanlı imza modeli:
//  1. Kayıt düzeyi: compliance_logs tablosundaki her kayıt prev_hash zinciri
//     ile sıra bütünlüğü taşır; saatlik dilimlerin kayıt hash'lerinden
//     Merkle kökü alınır ve checkpoint olarak zincirlenir
//  2. Günlük düzeyi: günün saatlik köklerinden günlük kök hesaplanır →
//     RFC 3161 nitelikli zaman damgası (TSA) + ed25519 manifest imzası →
//     imzalı paket WORM dizinine yazılır
//
// Sağlayıcı bağımsızlık: TSA istemcisi standart RFC 3161 konuşur (KamuSM,
// e-Tugra, TurkTrust vb.); mock sunucuyla test edilebilir. Nitelikli
// e-imza akıllı kart (PKCS#11) ileri adımdır; bu fazda manifest, PEM
// anahtarla (ed25519) imzalanır ve TSA zaman damgası hukuki zaman referansıdır.
package compliance

import (
	"crypto/sha256"
)

// MerkleRoot, hash listesinden Merkle kökünü hesaplar. Tek katman kopya
// ile doldurulur (standart Bitcoin benzeri yapı). Boş girdi → nil.
func MerkleRoot(hashes [][]byte) []byte {
	if len(hashes) == 0 {
		return nil
	}
	level := make([][]byte, len(hashes))
	copy(level, hashes)
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			h := sha256.New()
			h.Write(level[i])
			h.Write(level[i+1])
			next = append(next, h.Sum(nil))
		}
		level = next
	}
	return level[0]
}
