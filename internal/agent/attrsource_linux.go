//go:build linux

package agent

import (
	"fmt"
	"os"
	"strings"
)

// platformAttrCaps, Linux'ta eBPF atıf motorunun kullanılabilirliğini ölçer:
// kernel ≥ 5.8, CONFIG_DEBUG_INFO_BTF (/sys/kernel/btf/vmlinux) ve effective
// CAP_BPF / CAP_SYS_ADMIN (veya root). Yanlış-negatif zararsızdır — seçici
// pcap'e düşer. eBPF modunda L7 (SNI/Host) için CAP_NET_RAW da gerekir; yoksa
// yalnız L7 paneli boş kalır (sayım + DNS çalışır).
func platformAttrCaps() attrCaps {
	env := linuxCapEnv{
		osRelease:  readFileTrim("/proc/sys/kernel/osrelease"),
		btfPresent: fileReadable("/sys/kernel/btf/vmlinux"),
		euid:       os.Geteuid(),
		procStatus: readFileTrim("/proc/self/status"),
	}
	c := attrCaps{pcap: true}
	if ok, reason := linuxEBPFEnvOK(env); ok {
		c.ebpf = true
	} else {
		c.note = reason
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
	case "ebpf":
		src, err := newEbpfAttrSource(cfg)
		if err != nil {
			return nil, err
		}
		return src, nil
	default:
		return nil, fmt.Errorf("linux'ta %q atıf yöntemi desteklenmiyor", method)
	}
}

// linuxCapEnv, linuxEBPFEnvOK'un ihtiyaç duyduğu ortam okumaları — gerçek
// dosya erişimi platformAttrCaps'te yapılır, karar mantığı saf kalır.
type linuxCapEnv struct {
	osRelease  string // /proc/sys/kernel/osrelease ("6.8.0-51-generic")
	btfPresent bool   // /sys/kernel/btf/vmlinux okunabilir mi
	euid       int    // effective uid (0 = root)
	procStatus string // /proc/self/status (CapEff satırı için)
}

// linuxEBPFEnvOK, ortamın eBPF atıf motorunu taşıyıp taşımadığını ve
// taşımıyorsa nedenini döndürür.
func linuxEBPFEnvOK(env linuxCapEnv) (ok bool, reason string) {
	maj, min, parsed := parseKernelVersion(env.osRelease)
	if !parsed {
		return false, "eBPF atlandı: kernel sürümü okunamadı"
	}
	if !kernelAtLeast(maj, min, 5, 8) {
		return false, fmt.Sprintf("eBPF atlandı: kernel %d.%d < 5.8", maj, min)
	}
	if !env.btfPresent {
		return false, "eBPF atlandı: /sys/kernel/btf/vmlinux yok (CONFIG_DEBUG_INFO_BTF kapalı)"
	}
	if !linuxHasBPFCap(env.euid, env.procStatus) {
		return false, "eBPF atlandı: CAP_BPF / CAP_SYS_ADMIN yok"
	}
	return true, ""
}

// linuxHasBPFCap, süreç eBPF programı yükleyebilir mi: root veya effective
// CAP_SYS_ADMIN / CAP_BPF.
func linuxHasBPFCap(euid int, procStatus string) bool {
	if euid == 0 {
		return true
	}
	capEff, ok := parseCapEff(procStatus)
	if !ok {
		return false
	}
	const capSysAdmin, capBPF = 21, 39
	return capBit(capEff, capSysAdmin) || capBit(capEff, capBPF)
}

func readFileTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func fileReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
