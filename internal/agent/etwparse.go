package agent

// ETW Kernel-Network + DNS-Client olaylarının saf (platformdan bağımsız)
// çözümü. Gerçek ETW oturumu yalnızca Windows'ta kurulur (etw_windows.go /
// etwattr.go) ama bu ayrıştırıcılar her platformda derlenir ve test edilir.

import (
	"encoding/binary"
	"net"
	"unicode/utf16"
)

type etwFlowKey struct {
	pid      uint32
	proto    string // "tcp" | "udp"
	remoteIP string
	port     uint16
}

// Kernel-Network manifest olay id'leri → (send?, udp?, v6?). TCP send/recv
// (10/11 v4, 26/27 v6), UDP send/recv (42/43 v4, 58/59 v6). Bağlantı /
// retransmit / disconnect vb. atlanır.
type etwNetShape struct{ send, udp, v6 bool }

var etwKernelNetIDs = map[uint16]etwNetShape{
	10: {send: true},
	11: {},
	26: {send: true, v6: true},
	27: {v6: true},
	42: {send: true, udp: true},
	43: {udp: true},
	58: {send: true, udp: true, v6: true},
	59: {udp: true, v6: true},
}

func kernelNetWanted(id uint16) bool { _, ok := etwKernelNetIDs[id]; return ok }

// kernelNetFlow, ham bir Kernel-Network olayını (event id + UserData blob'u +
// EventHeader PID'i) bir akış deltasına çözer. UserData düzeni sabittir:
//
//	v4:  PID(4) size(4) daddr(4) saddr(4) dport(2be) sport(2be) …
//	v6:  PID(4) size(4) daddr(16) saddr(16) dport(2be) sport(2be) …
//
// Uzak uç seçimi (canlı ETW verisiyle doğrulandı):
//   - TCP (10/11/26/27): bağlantı-yönelimli — peer HER ZAMAN daddr:dport'ta,
//     send de recv de. (İstemci-taraflı bağlantılar için; server soketinde
//     yanılabilir ama endpoint ajanında baskın durum giden bağlantıdır.)
//   - UDP send (42/58): peer = daddr:dport
//   - UDP recv (43/59): peer = saddr:sport (paket-yönelimli, uçlar ters)
//
// Bayt yönü (in/out) her durumda send/recv'e göre.
func kernelNetFlow(id uint16, data []byte, headerPID uint32) (k etwFlowKey, out, in uint64, ok bool) {
	sh, wanted := etwKernelNetIDs[id]
	if !wanted {
		return
	}

	need := 20
	if sh.v6 {
		need = 44
	}
	if len(data) < need {
		return
	}

	pid := binary.LittleEndian.Uint32(data[0:4])
	if pid == 0 {
		pid = headerPID
	}
	size := binary.LittleEndian.Uint32(data[4:8])
	if size == 0 {
		return
	}

	var dIP, sIP net.IP
	var dPort, sPort uint16
	if sh.v6 {
		dIP, sIP = net.IP(append([]byte(nil), data[8:24]...)), net.IP(append([]byte(nil), data[24:40]...))
		dPort = binary.BigEndian.Uint16(data[40:42])
		sPort = binary.BigEndian.Uint16(data[42:44])
	} else {
		dIP, sIP = net.IP(append([]byte(nil), data[8:12]...)), net.IP(append([]byte(nil), data[12:16]...))
		dPort = binary.BigEndian.Uint16(data[16:18])
		sPort = binary.BigEndian.Uint16(data[18:20])
	}

	proto := "tcp"
	if sh.udp {
		proto = "udp"
	}

	// Loopback / multicast / broadcast trafiği atlanır — "uzak host" kavramı
	// yok. Multicast'te (mDNS 224.0.0.251, SSDP 239.255.255.250, LLMNR, ff0x::)
	// paket kendine geri dönerse recv olayının saddr'ı bizim IP'miz olur; iki
	// ucu da kontrol et. eBPF arka ucu da loopback'i eler.
	if !usableRemote(dIP) || !usableRemote(sIP) {
		return etwFlowKey{}, 0, 0, false
	}

	// TCP → daima daddr:dport; UDP → send'de daddr, recv'de saddr.
	useDst := !sh.udp || sh.send
	if useDst {
		k = etwFlowKey{pid: pid, proto: proto, remoteIP: dIP.String(), port: dPort}
	} else {
		k = etwFlowKey{pid: pid, proto: proto, remoteIP: sIP.String(), port: sPort}
	}
	if sh.send {
		out = uint64(size)
	} else {
		in = uint64(size)
	}
	ok = k.remoteIP != "" && k.remoteIP != "<nil>"
	return
}

// usableRemote, bir IP'nin süreç atfında anlamlı bir "uzak uç" olup olmadığı.
// nil / loopback / belirsiz (0.0.0.0, ::) / broadcast / her tür multicast → hayır.
func usableRemote(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return !ip.Equal(net.IPv4bcast)
}

// DNS-Client: 3006 = sorgu başladı, 3008 = sorgu tamamlandı. İkisinde de
// QueryName UserData'nın başında null-sonlu UTF-16 dizedir.
func dnsClientWanted(id uint16) bool { return id == 3006 || id == 3008 }

// dnsClientDomain, bir DNS-Client olayından sorgulanan alan adını çıkarır.
// keepDomain filtresi (ters arama / .local / noktasız) pcap + eBPF ile ortaktır.
func dnsClientDomain(id uint16, data []byte) (domain string, isResp, ok bool) {
	if !dnsClientWanted(id) || len(data) < 2 {
		return "", false, false
	}
	dom := normalizeDomain(utf16zString(data))
	if !keepDomain(dom) {
		return "", false, false
	}
	return dom, id == 3008, true
}

// utf16zString, bir bayt diliminin başındaki null-sonlu little-endian UTF-16
// dizeyi çözer.
func utf16zString(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := uint16(b[i]) | uint16(b[i+1])<<8
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}
