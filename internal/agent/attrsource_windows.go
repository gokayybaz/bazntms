//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// platformAttrCaps, Windows'ta ETW atıf motorunun kullanılabilirliğini ölçer.
// ETW Kernel-Network + DNS-Client sağlayıcıları yükseltilmiş (SYSTEM / yönetici)
// süreç ister; agent normalde SYSTEM servis olarak çalışır. Yükseltilmemişse
// "auto" pcap'e (Npcap kuruluysa) düşer. ETW modunda L7 (SNI/Host) yoktur —
// payload gerektirir, `-collect-method=pcap` + Npcap ile alınır.
func platformAttrCaps() attrCaps {
	c := attrCaps{pcap: true}
	if windowsElevated() {
		c.etw = true
	} else {
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
