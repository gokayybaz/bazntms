package driver

// Faz 6.1: topoloji kesfi yardimcilarinin birim testleri (SNMP'siz).

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

func TestTrimBaseAndSuffix(t *testing.T) {
	if got := trimBase(".1.0.8802.1.1.2.1.4.1.1.9.1.3.2", oidLldpRemSysName); got != "1.3.2" {
		t.Fatalf("trimBase: %q", got)
	}
	if got := trimBase(".1.3.6.1.2.1", oidLldpRemSysName); got != "" {
		t.Fatalf("farklı tablo bos donmeli: %q", got)
	}
	if got := suffixAt(".1.0.8802.1.1.2.1.3.7.1.1.5", oidLldpLocPortId, 1); got != 5 {
		t.Fatalf("suffixAt: %d", got)
	}
}

func TestRemIndexKey(t *testing.T) {
	// lldpRem index: timeMark.portNum.remIndex → "portNum.remIndex"
	key := remIndexKey(".1.0.8802.1.1.2.1.4.1.1.9.0.25.7", oidLldpRemSysName)
	if key != "25.7" {
		t.Fatalf("remIndexKey: %q", key)
	}
	if got := remPortNum(key); got != 25 {
		t.Fatalf("remPortNum: %d", got)
	}
}

func TestCdpAddressAndMac(t *testing.T) {
	pdu := gosnmp.SnmpPDU{Value: []byte{10, 0, 0, 9}}
	if got := cdpAddress(pdu); got != "10.0.0.9" {
		t.Fatalf("cdpAddress: %q", got)
	}
	if got := cdpAddress(gosnmp.SnmpPDU{Value: "not-bytes"}); got != "" {
		t.Fatalf("gecersiz adres bos olmali: %q", got)
	}

	mac := macString(gosnmp.SnmpPDU{Value: []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}})
	if mac != "de:ad:be:ef:00:01" {
		t.Fatalf("macString: %q", mac)
	}
}

func TestParseNumAndSplit(t *testing.T) {
	if got := parseNum("42"); got != 42 {
		t.Fatalf("parseNum: %d", got)
	}
	if got := parseNum("4x2"); got != -1 {
		t.Fatalf("gecersiz num -1: %d", got)
	}
	parts := splitSuffix("1.1.10.0.0.1")
	if len(parts) != 6 || parts[0] != "1" || parts[5] != "1" {
		t.Fatalf("splitSuffix: %v", parts)
	}
}
