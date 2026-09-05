package agent

import (
	"fmt"
	"net"
	"runtime"
	"strings"

	"github.com/google/gopacket/pcap"
)

// ResolvePcapDevice, kullanicinin verdigi arayuz adini libpcap'in
// OpenLive/Activate'e verebilecegi cihaz adina cevirir.
//
// Linux/macOS: net.Interface.Name (eth0/en0) zaten pcap cihaz adidir — ad
// oldugu gibi doner.
//
// Windows: pcap cihazlari `\Device\NPF_{GUID}` biciminde adlanir; Go'nun
// net.Interfaces()'i ise friendly ad ("Ethernet", "Wi-Fi") verir. Friendly
// ad dogrudan pcap.OpenLive'a verilince "Error opening adapter" (Windows
// hata 123, ERROR_INVALID_NAME) olusur — atif/L7 motoru hic baslamaz. Burada
// pcap.FindAllDevs() ile gercek cihaz, friendly arayuzun IP adreslerinden
// eslestirilir. Eslesme yoksa ad oldugu gibi geri doner (cagiran taraf ham
// pcap hatasini gorur; Npcap hic kurulu degilse FindAllDevs zaten hata verir).
func ResolvePcapDevice(name string) (string, error) {
	if runtime.GOOS != "windows" || name == "" {
		return name, nil
	}
	if strings.HasPrefix(name, `\Device\`) {
		return name, nil // zaten pcap cihaz adi
	}

	ifi, err := net.InterfaceByName(name)
	if err != nil {
		return name, fmt.Errorf("arayuz %q bulunamadi: %w", name, err)
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return name, fmt.Errorf("arayuz %q adresleri okunamadi: %w", name, err)
	}
	want := make(map[string]struct{}, len(addrs))
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			want[ipn.IP.String()] = struct{}{}
		}
	}
	if len(want) == 0 {
		return name, fmt.Errorf("arayuz %q uzerinde IP adresi yok", name)
	}

	devs, err := pcap.FindAllDevs()
	if err != nil {
		return name, fmt.Errorf("pcap cihaz listesi alinamadi (Npcap kurulu mu?): %w", err)
	}
	for _, d := range devs {
		for _, a := range d.Addresses {
			if _, ok := want[a.IP.String()]; ok {
				return d.Name, nil
			}
		}
	}
	return name, fmt.Errorf("%q arayuzune karsilik gelen pcap cihazi bulunamadi", name)
}
