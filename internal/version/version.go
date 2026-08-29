// Package version, hub ve agent binary'leri arasinda paylasilan surum
// bilgilerini tutar. Release build'lerinde ldflags ile set edilir:
//
//	-X github.com/gokayybaz/bazntms/internal/version.Version=vX.Y.Z
package version

import "runtime"

// Version, binary surumu (SemVer). Varsayilan dev; release ldflags ile override edilir.
var Version = "dev"

// ProtocolVersion, agent ↔ hub telgraf protokolu surumudur. Handshake'te
// karsilikli uyumluluk kontrolu icin kullanilir (Faz 1). Getiri kurallari:
// geriye uyumlu ekleme -> minor artis, kirici degisiklik -> major artis.
const ProtocolVersion = 1

// Info, /api/v1/version ve baslangic banner'i icin surum bilgisini dondurur.
func Info() map[string]any {
	return map[string]any{
		"version":          Version,
		"protocol_version": ProtocolVersion,
		"go_version":       runtime.Version(),
	}
}
