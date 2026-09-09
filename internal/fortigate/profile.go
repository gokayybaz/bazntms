package fortigate

// Sürüm profili (Faz 27).
//
// Toleranslı ayrıştırma (parse.go) `results` biçim/alan farklarının çoğunu
// kapatır. Profil, toleransla çözülemeyen — endpoint yolu değişmiş, bir query
// parametresinin değeri sürüme bağlı — az sayıda farkı taşır ve kullanıcının
// form'dan elle sabitleyebildiği (yarı-manuel override) knob'dur.
//
// Auto tespit: her FortiOS yanıtının zarfında `version` (`v7.2.11`) vardır;
// istemci ilk yanıttan profili kendisi seçer (pinlenmemişse).

import "strings"

// Profile, tek bir FortiOS sürüm ailesinin uç davranış farkları.
type Profile struct {
	ID    string // "7.0" | "7.2" | "7.4" | "7.6" | "default"
	Label string
	// sdwanPath, SD-WAN health-check ucunun yolu; boş → varsayılan.
	sdwanPath string
}

// SDWANPath, SD-WAN health-check endpoint yolu.
func (p Profile) SDWANPath() string {
	if p.sdwanPath != "" {
		return p.sdwanPath
	}
	return "/api/v2/monitor/virtual-wan/health-check"
}

const sdwanHealthCheck = "/api/v2/monitor/virtual-wan/health-check"

// Profiles, form dropdown'ında sunulan sıralı liste. Son eleman ID="default"
// (bilinmeyen/gelecek sürümler → en yeni bilinen davranış).
var Profiles = []Profile{
	{ID: "7.0", Label: "FortiOS 7.0", sdwanPath: sdwanHealthCheck},
	{ID: "7.2", Label: "FortiOS 7.2", sdwanPath: sdwanHealthCheck},
	{ID: "7.4", Label: "FortiOS 7.4", sdwanPath: sdwanHealthCheck},
	{ID: "7.6", Label: "FortiOS 7.6", sdwanPath: sdwanHealthCheck},
	{ID: "default", Label: "Otomatik / en yeni", sdwanPath: sdwanHealthCheck},
}

// ProfileByID, verilen ID'nin profilini döndürür (ikinci dönüş: bulundu mu).
// "auto" / "" bilinçli olarak bulunamadı sayılır (çağıran versiyondan çözsün).
func ProfileByID(id string) (Profile, bool) {
	id = strings.TrimSpace(id)
	if id == "" || id == "auto" {
		return Profile{}, false
	}
	for _, p := range Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// defaultProfile, ID="default" profilini döndürür.
func defaultProfile() Profile {
	p, _ := ProfileByID("default")
	return p
}

// ValidProfileID, bir profil ID'sinin (veya "auto"/"") kabul edilebilir olup
// olmadığını söyler — server katmanı form doğrulaması için.
func ValidProfileID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "auto" {
		return true
	}
	_, ok := ProfileByID(id)
	return ok
}

// ProfileForVersion, FortiOS sürüm dizesinden ("v7.2.11", "7.2.11", "7.2")
// major.minor eşleşen profili seçer; bilinmiyorsa "default".
func ProfileForVersion(version string) Profile {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		key := parts[0] + "." + parts[1]
		if p, ok := ProfileByID(key); ok {
			return p
		}
	}
	return defaultProfile()
}

// ResolveProfile, etkin profili seçer: kullanıcı pinlediyse (pinned ≠ ""/"auto")
// o kazanır; yoksa sürümden; sürüm de yoksa "default".
func ResolveProfile(pinned, version string) Profile {
	if p, ok := ProfileByID(pinned); ok {
		return p
	}
	if strings.TrimSpace(version) != "" {
		return ProfileForVersion(version)
	}
	return defaultProfile()
}
