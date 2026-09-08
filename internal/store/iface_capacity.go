package store

import "strings"

// --- arayüz kapasite farkındalığı (Faz 23-C) ---

// ifaceUtil, arayüz kullanım yüzdesini (0-100) hesaplar. Yalnız güvenilir hız
// (speedBps > 0) ve oper=up iken; aksi (-1, -1) — UI/uyarı bunu "bilinmiyor"
// olarak ele alır ve yanıltıcı uyarı üretmez.
func ifaceUtil(rxBps, txBps float64, speedBps uint64, operStatus int) (rxPct, txPct float64) {
	if speedBps == 0 || operStatus != 1 {
		return -1, -1
	}
	capBps := float64(speedBps)
	rxPct = 100 * (rxBps * 8) / capBps
	txPct = 100 * (txBps * 8) / capBps
	if rxPct < 0 {
		rxPct = 0
	}
	if txPct < 0 {
		txPct = 0
	}
	return rxPct, txPct
}

// vpnHints, tünel arayüzünü VPN olarak sınıflandıran ad/alias ipuçları.
var vpnHints = []string{"vpn", "ipsec", "wireguard", "wg-", "l2tp", "pptp", "ssl.root", "ssl-vpn", "anyconnect", "openvpn"}

// classifyIfType, IANAifType (RFC 1213 / IANA ifType MIB) tam sayısını
// operatörün tanıdığı bir sınıfa eşler. Bilinmeyen/eşleşmeyen → "unknown".
// Tünel arayüzleri ad ipucu VPN'e işaret ediyorsa "vpn".
func classifyIfType(t int, name, alias string) string {
	switch t {
	case 6, 7, 26, 62, 117, 161: // ethernetCsmacd, iso88023Csmacd, ethernet3Mbit, fastEther, gigabitEthernet, ieee8023adLag
		return "ethernet"
	case 71, 188, 254: // ieee80211, radioMAC, ...
		return "wifi"
	case 24: // softwareLoopback
		return "loopback"
	case 131, 150: // tunnel, mplsTunnel
		if hasHint(name, alias, vpnHints) {
			return "vpn"
		}
		return "tunnel"
	case 23, 108: // ppp, pppMultilinkBundle
		return "ppp"
	case 209: // bridge
		return "bridge"
	case 135, 136, 137: // l2vlan, l3ipvlan, l3ipxvlan
		return "vlan"
	case 53: // propVirtual — genelde VLAN/sanal
		return "vlan"
	}
	return "unknown"
}

func hasHint(name, alias string, hints []string) bool {
	s := strings.ToLower(name + " " + alias)
	for _, h := range hints {
		if strings.Contains(s, h) {
			return true
		}
	}
	return false
}

// IfaceAlertSkip, kullanım uyarısı üretilmeyecek sınıflar (yanıltıcı olur):
// loopback her zaman doludur; tünel arayüzü fiziksel hattın alt kümesidir.
var IfaceAlertSkip = map[string]bool{"loopback": true, "tunnel": true}
