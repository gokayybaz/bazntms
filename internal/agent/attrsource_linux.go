//go:build linux

package agent

import "fmt"

// platformAttrCaps, Linux atıf yeteneklerini ölçer.
//
// S20.3'te eBPF için gerçek kontrol eklenecek: kernel ≥ 5.8,
// /sys/kernel/btf/vmlinux ve effective CAP_BPF / CAP_SYS_ADMIN. Şimdilik
// yalnızca pcap kullanılabilir sayılır → "auto" bugünkü davranışı korur.
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
	case "ebpf":
		return nil, fmt.Errorf("eBPF atıf arka ucu henüz uygulanmadı (S20.5)")
	default:
		return nil, fmt.Errorf("linux'ta %q atıf yöntemi desteklenmiyor", method)
	}
}
