// Package proctraffic, paketleri sureclere atfeder (nethogs yontemi):
// pcap ile yakalanan 4'lu celiski, donemlik soket-tablosu -> PID eslemesiyle
// surece donusturulur. Platform saglayicilari:
//
//	linux  : /proc/net/* + /proc/[pid]/fd (soket inode)
//	darwin : lsof -F pcnPf (root tum surecleri gorur; protokol buyuk P alani)
//	windows: netstat -ano (pid) + gopsutil surec adi
//
// Not: eBPF (Linux) ve ETW (Windows) ile saglayici daha hassas surumleri
// ileri fazda bu arayuzun arkasina eklenebilir.
package proctraffic

import "net"

// Key, bir baglantinin yonetsiz 4'lusunu tanimlar (her iki yon icin de
// eslemede kullanilir).
type Key struct {
	Proto      string
	LocalIP    string
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
}

// ProcInfo, bir baglantiya ait surec bilgisidir. inode alanı Linux
// saglayicisinin ara esleme amaclidir (same-package).
type ProcInfo struct {
	PID     int32
	Process string
	inode   string
}

// Provider, baglanti -> surec esleme tablosunun donemlik anlik gorunumunu
// saglar. Donen haritada HER IKI yonlu anahtar bulunur (packet src/dst
// hangi sira gelirse gelsin bulunur).
type Provider interface {
	// Snapshot, guncel esleme tablosunu dondurur. Ucuz olmasi icin cagiran
	// taraf donemlik (2-5 sn) cagirir.
	Snapshot() map[Key]ProcInfo
}

// LocalIPs, atif yonu tespiti icin makinenin yerel IP'lerini dondurur.
func LocalIPs() map[string]struct{} {
	out := map[string]struct{}{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				out[ipnet.IP.String()] = struct{}{}
			} else if ip, ok := a.(*net.IPAddr); ok {
				out[ip.IP.String()] = struct{}{}
			}
		}
	}
	return out
}

// NewProvider, calistigi platforma uygun saglayiciyi dondurur.
func NewProvider() Provider { return newPlatformProvider() }
