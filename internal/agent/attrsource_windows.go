//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// etwBackendBuilt, ETW atıf arka ucunun bu derlemede mevcut olup olmadığı.
// S20.10'da gerçek implementasyon eklenince true olur; o zamana dek "auto"
// modu yükseltilmiş süreçte bile pcap kullanır.
var etwBackendBuilt = false

// platformAttrCaps, Windows'ta ETW atıf motorunun kullanılabilirliğini ölçer.
// ETW Kernel-Network sağlayıcısı yükseltilmiş (SYSTEM / yönetici) süreç ister;
// agent normalde SYSTEM servis olarak çalışır.
func platformAttrCaps() attrCaps {
	c := attrCaps{pcap: true}
	elevated := windowsElevated()
	switch {
	case elevated && etwBackendBuilt:
		c.etw = true
	case elevated:
		// yükseltilmiş ama arka uç henüz derlenmedi — sessiz (geçici, S20.10)
	default:
		c.note = "ETW atlandı: süreç yükseltilmemiş (SYSTEM / yönetici gerekir)"
	}
	return c
}

func platformBuildAttrSource(method string, cfg AttrConfig) (AttrSource, error) {
	switch method {
	case "pcap":
		src, err := newPcapAttrSource(cfg.Iface)
		if err != nil {
			return nil, err
		}
		return src, nil
	case "etw":
		return nil, fmt.Errorf("ETW atıf arka ucu henüz uygulanmadı (S20.10)")
	default:
		return nil, fmt.Errorf("windows'ta %q atıf yöntemi desteklenmiyor", method)
	}
}

// windowsElevated, sürecin ETW kernel oturumu açabilecek yetkide olup
// olmadığını söyler: UAC-yükseltilmiş veya LocalSystem (S-1-5-18).
func windowsElevated() bool {
	tok := windows.GetCurrentProcessToken()
	if tok.IsElevated() {
		return true
	}
	// SYSTEM servisi UAC'ye tabi değildir; bazı yapılandırmalarda IsElevated
	// false döner — kullanıcı SID'ini doğrudan kontrol et.
	if u, err := tok.GetTokenUser(); err == nil && u.User.Sid != nil {
		return u.User.Sid.String() == "S-1-5-18"
	}
	return false
}
