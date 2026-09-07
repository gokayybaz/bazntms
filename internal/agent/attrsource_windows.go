//go:build windows

package agent

import "fmt"

// platformAttrCaps, Windows atıf yeteneklerini ölçer.
//
// S20.3'te ETW için gerçek kontrol eklenecek: süreç yükseltilmiş mi
// (SYSTEM / yönetici). Şimdilik yalnızca pcap kullanılabilir sayılır →
// "auto" bugünkü davranışı (Npcap varsa pcap) korur.
func platformAttrCaps() attrCaps {
	return attrCaps{pcap: true}
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
