package driver

// Topoloji kesfi (Faz 6.1): SNMP uzerinden LLDP-MIB, CISCO-CDP-MIB ve
// IP-MIB (ARP) tablolari yurutulerek komsuluklar cikarilir. Her protokol
// best-effort'tur: desteklenmeyen cihazlarda sessizce atlanir.
// Faz 8.1 refactor: kenarlar yazilmaz, Snapshot'a dondurulur.

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/gokayybaz/bazntms/internal/store"
)

// LLDP-MIB (IEEE 802.1AB)
const (
	oidLldpLocPortId   = "1.0.8802.1.1.2.1.3.7.1.1" // index: portNum
	oidLldpLocPortDesc = "1.0.8802.1.1.2.1.3.7.1.3" // index: portNum
	oidLldpRemPortId   = "1.0.8802.1.1.2.1.4.1.1.4" // index: timeMark.portNum.remIndex
	oidLldpRemSysName  = "1.0.8802.1.1.2.1.4.1.1.9" // index: timeMark.portNum.remIndex
)

// CISCO-CDP-MIB
const (
	oidCdpCacheDeviceId   = "1.3.6.1.4.1.9.9.23.1.2.1.1.6" // index: ifIndex.cacheIndex
	oidCdpCacheDevicePort = "1.3.6.1.4.1.9.9.23.1.2.1.1.7" // index: ifIndex.cacheIndex
	oidCdpCacheAddress    = "1.3.6.1.4.1.9.9.23.1.2.1.1.4" // index: ifIndex.cacheIndex
)

// IP-MIB (ARP)
const oidIpNetToMediaPhys = "1.3.6.1.2.1.4.22.1.2" // index: ifIndex.1.b1.b2.b3.b4

const maxArpLinksPerDevice = 256

func timeNowUnix() int64 { return time.Now().Unix() }

// discoverTopology, tek cihaz icin komsuluklari toplar ve dondurur (yazim scheduler'da).
func discoverTopology(conn *gosnmp.GoSNMP, d store.Device, sourceName string, ifaces map[int64]store.DeviceIface) []store.TopologyLink {
	var links []store.TopologyLink

	if l := discoverLLDP(conn, d, sourceName, ifaces); len(l) > 0 {
		links = append(links, l...)
	}
	if c := discoverCDP(conn, d, sourceName, ifaces); len(c) > 0 {
		links = append(links, c...)
	}
	if a := discoverARP(conn, d, sourceName, ifaces); len(a) > 0 {
		links = append(links, a...)
	}

	if len(links) > 0 {
		slog.Info("topoloji kesfi", "device", d.Name, "kenar", len(links))
	}
	return links
}

// discoverLLDP, lldpRemTable + lldpLocPortId tablolarindan komsu cihazlari cikarir.
func discoverLLDP(conn *gosnmp.GoSNMP, d store.Device, sourceName string, ifaces map[int64]store.DeviceIface) []store.TopologyLink {
	// yerel port numarasi → port adi
	portName := map[int64]string{}
	if rows, err := conn.BulkWalkAll(oidLldpLocPortId); err == nil {
		for _, pdu := range rows {
			if port := suffixAt(pdu.Name, oidLldpLocPortId, 1); port >= 0 {
				portName[port] = pduValueString(pdu)
			}
		}
	}
	if rows, err := conn.BulkWalkAll(oidLldpLocPortDesc); err == nil {
		for _, pdu := range rows {
			if port := suffixAt(pdu.Name, oidLldpLocPortDesc, 1); port >= 0 {
				if _, ok := portName[port]; !ok {
					portName[port] = pduValueString(pdu)
				}
			}
		}
	}

	// komsular
	remPort := map[string]string{} // "portNum.remIndex" → komsu port id
	if rows, err := conn.BulkWalkAll(oidLldpRemPortId); err == nil {
		for _, pdu := range rows {
			if key := remIndexKey(pdu.Name, oidLldpRemPortId); key != "" {
				remPort[key] = pduValueString(pdu)
			}
		}
	}

	now := timeNowUnix()
	var out []store.TopologyLink
	if rows, err := conn.BulkWalkAll(oidLldpRemSysName); err == nil {
		for _, pdu := range rows {
			key := remIndexKey(pdu.Name, oidLldpRemSysName)
			if key == "" {
				continue
			}
			peerName := truncate(pduValueString(pdu), 100)
			if peerName == "" {
				continue
			}
			portNum := remPortNum(key)
			localPort := portName[portNum]
			if localPort == "" {
				if iface, ok := ifaces[portNum]; ok {
					localPort = iface.Name
				}
			}
			remotePort := remPort[key]
			out = append(out, store.TopologyLink{
				Ts: now, Kind: "lldp",
				SourceType: "device", SourceID: d.ID, SourceName: sourceName,
				LocalPort: localPort, PeerType: "device",
				PeerName: peerName, PeerIP: "",
			})
			if remotePort != "" {
				out[len(out)-1].LocalPort = localPort
				out[len(out)-1].PeerName = fmt.Sprintf("%s [%s]", peerName, remotePort)
			}
		}
	}
	return out
}

// discoverCDP, Cisco cdpCacheTable'dan komsulari cikarir (adres + port).
func discoverCDP(conn *gosnmp.GoSNMP, d store.Device, sourceName string, ifaces map[int64]store.DeviceIface) []store.TopologyLink {
	// komsu cihaz portu (ayni index uzayinda): ifIndex.cacheIndex → port
	peerPorts := map[string]string{}
	if rows, err := conn.BulkWalkAll(oidCdpCacheDevicePort); err == nil {
		for _, pdu := range rows {
			if key := trimBase(pdu.Name, oidCdpCacheDevicePort); key != "" {
				peerPorts[key] = pduValueString(pdu)
			}
		}
	}
	addrs := map[string]string{}
	if rows, err := conn.BulkWalkAll(oidCdpCacheAddress); err == nil {
		for _, pdu := range rows {
			if key := trimBase(pdu.Name, oidCdpCacheAddress); key == "" {
				continue
			} else if ip := cdpAddress(pdu); ip != "" {
				addrs[key] = ip
			}
		}
	}

	now := timeNowUnix()
	var out []store.TopologyLink
	rows, err := conn.BulkWalkAll(oidCdpCacheDeviceId)
	if err != nil {
		return nil
	}
	for _, pdu := range rows {
		key := trimBase(pdu.Name, oidCdpCacheDeviceId)
		if key == "" {
			continue
		}
		parts := splitSuffix(key)
		if len(parts) < 1 {
			continue
		}
		ifIndex := parseNum(parts[0])
		peerName := truncate(pduValueString(pdu), 100)
		if peerName == "" {
			continue
		}
		if rp := peerPorts[key]; rp != "" {
			peerName = fmt.Sprintf("%s [%s]", peerName, rp)
		}
		localPort := ""
		if iface, ok := ifaces[ifIndex]; ok {
			localPort = iface.Name
		}
		out = append(out, store.TopologyLink{
			Ts: now, Kind: "cdp",
			SourceType: "device", SourceID: d.ID, SourceName: sourceName,
			LocalPort: localPort, PeerType: "device",
			PeerName: peerName, PeerIP: addrs[key],
		})
	}
	return out
}

// discoverARP, ipNetToMedia tablosundan host-mac eslemelerini cikarir
// (switch portlarina bagli uc noktalar; gurultuyu sinirlamak icin ust sinirli).
func discoverARP(conn *gosnmp.GoSNMP, d store.Device, sourceName string, ifaces map[int64]store.DeviceIface) []store.TopologyLink {
	rows, err := conn.BulkWalkAll(oidIpNetToMediaPhys)
	if err != nil {
		return nil
	}
	now := timeNowUnix()
	var out []store.TopologyLink
	for _, pdu := range rows {
		if len(out) >= maxArpLinksPerDevice {
			break
		}
		key := trimBase(pdu.Name, oidIpNetToMediaPhys)
		parts := splitSuffix(key)
		if len(parts) < 6 {
			continue // ifIndex.1.b1.b2.b3.b4
		}
		ifIndex := parseNum(parts[0])
		ip := fmt.Sprintf("%s.%s.%s.%s", parts[2], parts[3], parts[4], parts[5])
		if net.ParseIP(ip) == nil {
			continue
		}
		mac := macString(pdu)
		if mac == "" {
			continue
		}
		localPort := ""
		if iface, ok := ifaces[ifIndex]; ok {
			localPort = iface.Name
		}
		out = append(out, store.TopologyLink{
			Ts: now, Kind: "arp",
			SourceType: "device", SourceID: d.ID, SourceName: sourceName,
			LocalPort: localPort, PeerType: "host",
			PeerName: mac, PeerIP: ip,
		})
	}
	return out
}

// --- yardimcilar ---

// trimBase, ".<base>.<suffix>" ifadesinden <suffix> dondurur.
func trimBase(name, base string) string {
	suffix := strings.TrimPrefix(name, "."+base+".")
	if suffix == name || suffix == "" {
		return ""
	}
	return suffix
}

// splitSuffix, "a.b.c" bicimindeki index'i noktalardan boler.
func splitSuffix(key string) []string {
	return strings.Split(key, ".")
}

func parseNum(s string) int64 {
	n := int64(0)
	ok := false
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return -1
		}
		n = n*10 + int64(ch-'0')
		ok = true
	}
	if !ok {
		return -1
	}
	return n
}

// suffixAt, OID suffix'inin sondan n. bileşenini dondurur (1 = sonuncu).
func suffixAt(name, base string, n int) int64 {
	key := trimBase(name, base)
	if key == "" {
		return -1
	}
	parts := splitSuffix(key)
	if len(parts) < n {
		return -1
	}
	return parseNum(parts[len(parts)-n])
}

// remIndexKey, lldpRem index'inden "portNum.remIndex" anahtarini uretir
// (timeMark bileşenini atar).
func remIndexKey(name, base string) string {
	key := trimBase(name, base)
	parts := splitSuffix(key)
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// remPortNum, remIndexKey'in ilk bileşeni = lldpRemLocalPortNum.
func remPortNum(key string) int64 {
	parts := splitSuffix(key)
	if len(parts) < 2 {
		return -1
	}
	return parseNum(parts[0])
}

// cdpAddress, cdpCacheAddress degerini (4 bayt) IP'ye cevirir.
func cdpAddress(pdu gosnmp.SnmpPDU) string {
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) != 4 {
		return ""
	}
	return net.IP(b).String()
}

// macString, fiziksel adres degerini "aa:bb:cc:.." formatina cevirir.
func macString(pdu gosnmp.SnmpPDU) string {
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) != 6 {
		return ""
	}
	s := hex.EncodeToString(b)
	var sb strings.Builder
	for i := 0; i < len(s); i += 2 {
		if i > 0 {
			sb.WriteByte(':')
		}
		sb.WriteString(s[i : i+2])
	}
	return sb.String()
}
