package agent

import (
	"fmt"
	"runtime"
	"time"

	"github.com/google/gopacket/pcap"
)

// pcapIfLoopback, libpcap PCAP_IF_LOOPBACK bayragidir (gopacket sabiti disari
// vermez ama Interface.Flags degerini oldugu gibi tasir).
const pcapIfLoopback = 0x00000001

// loopbackDNSFilter, loopback handle'inin BPF filtresi. Ana atif handle'i tek
// bir loopback-olmayan arayuzu dinledigi icin stub-resolver'a (systemd-resolved
// 127.0.0.53, dnsmasq/Pi-hole, Docker gomulu DNS 127.0.0.11) giden sorgulari
// hic gormez — bu handle o boslugu kapatir.
//
// "port 53" degil sade "udp": Docker gomulu DNS, konteyner-ici iptables ile
// sorgunun HEDEF portunu 53'ten rastgele bir porta DNAT eder (yanit :53'e
// SNAT'lanir) — port filtresi sorguyu kacirirdi. Loopback UDP hacmi zaten
// dusuk; DNS olmayan paketleri parseDNSNames eler.
const loopbackDNSFilter = "udp"

// openLoopbackDNS, loopback arayuzunde yalnizca UDP/53 yakalayan bir pcap
// handle acar. Acilamazsa hata doner; cagiran taraf bunu olumcul saymaz
// (DNS gorunurlugu stub-resolver uclarinda kisitli kalir, telemetri aksamaz).
func openLoopbackDNS() (*pcap.Handle, error) {
	dev, err := loopbackDevice()
	if err != nil {
		return nil, err
	}
	h, err := pcap.OpenLive(dev, attrSnapLen, false, time.Second)
	if err != nil {
		return nil, fmt.Errorf("loopback %q acilamadi: %w", dev, err)
	}
	if err := h.SetBPFFilter(loopbackDNSFilter); err != nil {
		h.Close()
		return nil, fmt.Errorf("loopback BPF filtresi: %w", err)
	}
	return h, nil
}

// loopbackDevice, libpcap cihaz listesinden loopback cihazini bulur: once
// PCAP_IF_LOOPBACK bayragi, sonra 127/8 · ::1 adresi. Bulamazsa platform
// varsayilan adina duser (Linux "lo", BSD/macOS "lo0"). Windows'ta ayri bir
// Npcap loopback adaptoru gerekir — yoksa hata doner (best-effort).
func loopbackDevice() (string, error) {
	devs, err := pcap.FindAllDevs()
	if err == nil {
		for _, d := range devs {
			if d.Flags&pcapIfLoopback != 0 {
				return d.Name, nil
			}
			for _, a := range d.Addresses {
				if a.IP != nil && a.IP.IsLoopback() {
					return d.Name, nil
				}
			}
		}
	}
	switch runtime.GOOS {
	case "linux":
		return "lo", nil
	case "darwin", "freebsd", "openbsd", "netbsd", "dragonfly":
		return "lo0", nil
	}
	if err != nil {
		return "", fmt.Errorf("loopback cihazi bulunamadi: %w", err)
	}
	return "", fmt.Errorf("loopback cihazi bulunamadi")
}
