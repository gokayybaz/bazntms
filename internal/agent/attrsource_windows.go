//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// etwBackendBuilt, "auto" modunun ETW arka ucunu seçip seçmeyeceği. Consumer
// S20.8'de geldi; DNS (S20.9) henüz ETW tarafında boş döner ve session yaşam
// döngüsü Windows VM'de doğrulanmadı — bu yüzden "auto" hâlâ pcap kullanır,
// ETW yalnız `-collect-method=etw` ile açıkça seçilir. S20.10'da true olur.
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
		src, err := newEtwAttrSource(cfg)
		if err != nil {
			return nil, err
		}
		return src, nil
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
