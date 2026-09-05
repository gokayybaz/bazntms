package compliance

// Delil paketi + offline doğrulama (Faz 9.4).
//
// EvidenceBundle, tarih aralığındaki imzalı logların taşınabilir halidir:
// ham kayıtlar (hash'leriyle) + checkpoint'ler (Merkle kökleri, TSA token,
// imza) + sunucuda hesaplanmış ilk doğrulama raporu. VerifyBundle, bu
// paketi BAĞIMSIZ olarak yeniden hesaplayarak doğrular (adli kullanım).

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/store"
)

// EvidenceBundle, dışa aktarılabilir delil paketi.
type EvidenceBundle struct {
	GeneratedAt  int64                 `json:"generated_at"`
	From         int64                 `json:"from"`
	To           int64                 `json:"to"`
	Masked       bool                  `json:"masked"`
	Logs         []store.ComplianceLog `json:"logs"`
	Checkpoints  []store.LogCheckpoint `json:"checkpoints"`
	Verification Report                `json:"verification"`
}

// Report, doğrulama sonucu.
type Report struct {
	OK             bool     `json:"ok"`
	CheckedRecords int      `json:"checked_records"`
	BrokenSeq      int64    `json:"broken_seq"` // 0 = zincir sağlam
	MerkleOK       bool     `json:"merkle_ok"`
	FailedBucket   int64    `json:"failed_bucket"` // 0 = sorun yok
	DailyOK        bool     `json:"daily_ok"`
	SigOK          bool     `json:"sig_ok"`
	SigChecked     bool     `json:"sig_checked"`
	Notes          []string `json:"notes"`
}

// BuildEvidence, store'dan aralık paketini üretir; PII maskeleme opsiyonlu
// (A.5.34/A.8.11 — maskeleme yalnızca pakettedir, ham kayıt değişmez).
func BuildEvidence(st store.Store, from, to int64, maskPII bool) (*EvidenceBundle, error) {
	logs, err := st.ComplianceLogsBetween(from, to)
	if err != nil {
		return nil, err
	}
	cps, err := st.LogCheckpointsBetween(from, to)
	if err != nil {
		return nil, err
	}
	b := &EvidenceBundle{
		GeneratedAt: time.Now().Unix(), From: from, To: to, Masked: maskPII,
		Logs: logs, Checkpoints: cps,
	}
	if maskPII {
		for i := range b.Logs {
			b.Logs[i].SrcIP = MaskIP(b.Logs[i].SrcIP)
			b.Logs[i].SrcMAC = MaskMAC(b.Logs[i].SrcMAC)
			if b.Logs[i].UserID != "" {
				b.Logs[i].UserID = string(b.Logs[i].UserID[0]) + "***"
			}
		}
	}
	b.Verification = VerifyBundle(b, nil)
	return b, nil
}

// MaskIP, IPv4 son oktetini (diğer durumlarda son grubu) maskeleyebilir
// (10.1.2.3 → 10.1.2.x, 2001:db8::1 → 2001:db8::x).
func MaskIP(ip string) string {
	if ip == "" {
		return ip
	}
	if strings.Count(ip, ".") == 3 {
		if idx := strings.LastIndex(ip, "."); idx > 0 {
			return ip[:idx+1] + "x"
		}
	}
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		return ip[:idx+1] + "x"
	}
	return ip
}

// MaskMAC, MAC son iki oktetini maskeler.
func MaskMAC(mac string) string {
	if mac == "" {
		return mac
	}
	if len(mac) >= 17 {
		return mac[:14] + "xx:xx"
	}
	return "xx:xx:xx:xx:xx:xx"
}

// VerifyBundle, paketi doğrular. Maskesiz paketlerde kayıt hash'leri
// YENİDEN HESAPLANARAK içerik bütünlüğü kanıtlanır; maskeli pakette
// hash'ler orijinal içerik üzerindendir — yalnızca zincir bağlantısı,
// Merkle kökleri, TSA ve imza doğrulanır (maskeleme bir görünümdür).
func VerifyBundle(b *EvidenceBundle, pub ed25519.PublicKey) Report {
	rep := Report{OK: true, MerkleOK: true, DailyOK: true, SigOK: true}

	// 1) kayıt zinciri
	prev := ""
	for i, l := range b.Logs {
		if b.Masked {
			if l.PrevHash != prev || l.Hash == "" {
				rep.OK = false
				rep.BrokenSeq = l.Seq
				rep.Notes = append(rep.Notes, fmt.Sprintf("zincir bağlantısı kopuk: kayıt #%d (sıra %d)", i+1, l.Seq))
				break
			}
		} else if l.PrevHash != prev || store.ComplianceHash(prev, l) != l.Hash {
			rep.OK = false
			rep.BrokenSeq = l.Seq
			rep.Notes = append(rep.Notes, fmt.Sprintf("zincir kopuk: kayıt #%d (sıra %d)", i+1, l.Seq))
			break
		}
		prev = l.Hash
		rep.CheckedRecords++
	}

	// 2) saatlik Merkle kökleri
	byBucket := map[int64][][]byte{}
	counts := map[int64]int{}
	for _, l := range b.Logs {
		bucket := time.Unix(l.Ts, 0).Truncate(time.Hour).Unix()
		h, _ := hexDecode(l.Hash)
		byBucket[bucket] = append(byBucket[bucket], h)
		counts[bucket]++
	}
	var hourlyRoots [][]byte
	for _, cp := range b.Checkpoints {
		if cp.Kind != "hourly" {
			continue
		}
		root, _ := hexDecode(cp.Root)
		got := MerkleRoot(byBucket[cp.BucketStart])
		if cp.RecordCount != counts[cp.BucketStart] || !equalBytes(root, got) {
			rep.OK = false
			rep.MerkleOK = false
			rep.FailedBucket = cp.BucketStart
			rep.Notes = append(rep.Notes, fmt.Sprintf("saatlik Merkle uyuşmıyor: bucket %d", cp.BucketStart))
		}
		hourlyRoots = append(hourlyRoots, root)
	}

	// 3) checkpoint zinciri + günlük kök + TSA + imza
	prevRoot := ""
	for _, cp := range b.Checkpoints {
		if cp.Kind != "hourly" {
			continue
		}
		if cp.PrevRoot != prevRoot {
			rep.OK = false
			rep.MerkleOK = false
			rep.Notes = append(rep.Notes, fmt.Sprintf("checkpoint zinciri kopuk: bucket %d", cp.BucketStart))
		}
		prevRoot = cp.Root
	}
	for _, cp := range b.Checkpoints {
		if cp.Kind != "daily" {
			continue
		}
		root, _ := hexDecode(cp.Root)
		if !equalBytes(root, MerkleRoot(hourlyRoots)) {
			rep.OK = false
			rep.DailyOK = false
			rep.Notes = append(rep.Notes, "günlük kök saatlik köklerle uyuşmuyor")
		}
		if cp.TSAStatus == "ok" && len(cp.TSAToken) > 0 {
			// TSA token'ı mesaj özetini (sha256(kök)) messageImprint olarak taşır
			if !TokenContainsHash(cp.TSAToken, sha256Sum(root)) {
				rep.OK = false
				rep.Notes = append(rep.Notes, fmt.Sprintf("TSA token kökü içermiyor: bucket %d", cp.BucketStart))
			}
		} else {
			rep.Notes = append(rep.Notes, fmt.Sprintf("günlük mühürde TSA yok (status=%s): bucket %d", cp.TSAStatus, cp.BucketStart))
		}
		if pub != nil && cp.Signature != "" {
			// manifest'i checkpoint alanlarından birebir yeniden kur:
			// Hours = güne denk gelen saatlik checkpoint sayısı
			hours := 0
			for _, h := range b.Checkpoints {
				if h.Kind == "hourly" && h.BucketStart >= cp.BucketStart && h.BucketStart < cp.BucketEnd {
					hours++
				}
			}
			m := manifest{
				Day: time.Unix(cp.BucketStart, 0).Format("2006-01-02"), Root: cp.Root,
				RecordCount: cp.RecordCount, Hours: hours, PrevDaily: cp.PrevRoot,
				CreatedAt: cp.SignedAt,
			}
			mBytes, _ := json.Marshal(m)
			sig, err := hexDecode(cp.Signature)
			if err != nil || !ed25519.Verify(pub, mBytes, sig) {
				rep.OK = false
				rep.SigOK = false
				rep.Notes = append(rep.Notes, fmt.Sprintf("manifest imzası doğrulanamadı: bucket %d", cp.BucketStart))
			}
			rep.SigChecked = true
		}
	}
	return rep
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// BundleToJSON, paketi dışa aktarılabilir JSON'a çevirir.
func BundleToJSON(b *EvidenceBundle) ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}
