package driver

// SNMPDriver — Faz 3'ten gelen SNMPv2c/v3 poller'ının DeviceDriver
// gerçeklemesi (Faz 8.1 refactor): IF-MIB arayüz sayaçları, sistem bilgisi
// ve LLDP/CDP/ARP topoloji keşfi (topology.go).

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// OID'ler (MIB-2 + IF-MIB; 64-bit sayaclari onceliklidir).
const (
	oidSysDescr     = "1.3.6.1.2.1.1.1.0"
	oidSysName      = "1.3.6.1.2.1.1.5.0"
	oidIfIndex      = "1.3.6.1.2.1.2.2.1.1"
	oidIfDescr      = "1.3.6.1.2.1.2.2.1.2"
	oidIfSpeed      = "1.3.6.1.2.1.2.2.1.5"
	oidIfOperStatus = "1.3.6.1.2.1.2.2.1.8"
	oidIfInOctets   = "1.3.6.1.2.1.2.2.1.10"
	oidIfInErrors   = "1.3.6.1.2.1.2.2.1.14"
	oidIfOutOctets  = "1.3.6.1.2.1.2.2.1.16"
	oidIfOutErrors  = "1.3.6.1.2.1.2.2.1.20"
	oidIfName       = "1.3.6.1.2.1.31.1.1.1.1"
	oidIfAlias      = "1.3.6.1.2.1.31.1.1.1.18"
	oidIfHCInOctets = "1.3.6.1.2.1.31.1.1.1.6"
	oidIfHCOutOct   = "1.3.6.1.2.1.31.1.1.1.10"
)

// SNMPDriver, SNMP tabanli cihaz toplamasi.
type SNMPDriver struct{}

// Poll, tek cihazi yoklar; Snapshot'i dondurur (yazim yok).
func (s *SNMPDriver) Poll(ctx context.Context, d store.Device, v *vault.Vault) (Snapshot, error) {
	snap := Snapshot{}
	conn, err := connect(d, v)
	if err != nil {
		snap.LastError = err.Error()
		return snap, err
	}
	defer conn.Conn.Close()

	var ifaces []store.DeviceIface

	// sessiz arıza teşhisi: connect() UDP soketi açtığı için her zaman başarılıdır;
	// asıl "erişilebilir mi" sinyali ilk gerçek isteklerden gelir. sysOK/walkOK
	// hiç yanıt gelmediğini, pollErr son SNMP hatasını taşır (bkz. Poll sonu).
	var sysOK bool
	var pollErr error

	// sistem bilgisi
	if res, err := conn.Get([]string{oidSysDescr, oidSysName}); err == nil {
		sysOK = true
		for _, pdu := range res.Variables {
			v := pduValueString(pdu)
			switch pdu.Name {
			case "." + oidSysDescr:
				snap.SysDescr = truncate(v, 200)
			case "." + oidSysName:
				snap.SysName = truncate(v, 100)
			}
		}
	} else {
		pollErr = err
	}

	// IF-MIB kolonlarini yurut
	idx := map[string]store.DeviceIface{}
	cols := []struct {
		oid  string
		fill func(*store.DeviceIface, gosnmp.SnmpPDU)
	}{
		{oidIfIndex, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.IfIndex = pduInt(p) }},
		{oidIfDescr, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.Name = pduValueString(p) }},
		{oidIfName, func(i *store.DeviceIface, p gosnmp.SnmpPDU) {
			if v := pduValueString(p); v != "" {
				i.Name = v
			}
		}},
		{oidIfAlias, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.Alias = truncate(pduValueString(p), 120) }},
		{oidIfSpeed, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.Speed = pduUint(p) }},
		{oidIfOperStatus, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.OperStatus = int(pduInt(p)) }},
		{oidIfHCInOctets, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.RxBytes = pduUint(p) }},
		{oidIfInOctets, func(i *store.DeviceIface, p gosnmp.SnmpPDU) {
			if i.RxBytes == 0 {
				i.RxBytes = pduUint(p)
			}
		}},
		{oidIfHCOutOct, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.TxBytes = pduUint(p) }},
		{oidIfOutOctets, func(i *store.DeviceIface, p gosnmp.SnmpPDU) {
			if i.TxBytes == 0 {
				i.TxBytes = pduUint(p)
			}
		}},
		{oidIfInErrors, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.InErrors = pduUint(p) }},
		{oidIfOutErrors, func(i *store.DeviceIface, p gosnmp.SnmpPDU) { i.OutErrors = pduUint(p) }},
	}
	walkOK := 0
	for _, col := range cols {
		pdus, err := conn.BulkWalkAll(col.oid)
		if err != nil {
			pollErr = err
			continue // kolon desteklenmiyorsa (ya da yanıt yoksa) atla
		}
		walkOK++
		for _, pdu := range pdus {
			ifIndex := ifIndexFromOID(pdu.Name, col.oid)
			if ifIndex < 0 {
				continue
			}
			key := fmt.Sprint(ifIndex)
			i, ok := idx[key]
			if !ok {
				i = store.DeviceIface{IfIndex: ifIndex}
			}
			col.fill(&i, pdu)
			idx[key] = i
		}
	}
	for _, i := range idx {
		ifaces = append(ifaces, i)
	}
	snap.Ifaces = ifaces

	// Sessiz SNMP arızası: bağlantı kuruldu ama tek bir arayüz bile alınamadı.
	// Driver tüm OID hatalarını yuttuğu için bunu çağırana hata olarak bildir —
	// yoksa devpoll "poll tamam" sayıp last_error'ı temizler ve cihaz veri
	// gelmediği hâlde "online" görünür.
	if len(ifaces) == 0 {
		err := snmpNoTelemetryErr(d.Host, walkOK, sysOK, pollErr)
		snap.LastError = err.Error()
		return snap, err
	}

	// topoloji kesfi (Faz 6.1): LLDP/CDP/ARP — best-effort
	ifMap := map[int64]store.DeviceIface{}
	for _, i := range ifaces {
		ifMap[i.IfIndex] = i
	}
	source := snap.SysName
	if source == "" {
		source = d.Name
	}
	snap.Links = discoverTopology(conn, d, source, ifMap)

	return snap, nil
}

// snmpNoTelemetryErr, hiç arayüz alınamadığında tanılı hatayı üretir.
//   - walkOK: hatasız dönen IF-MIB kolon yürüyüşü sayısı
//   - sysOK:  sysDescr/sysName Get'i yanıt verdi mi
//   - cause:  görülen son SNMP hatası (nil olabilir)
//
// Hiçbir istek yanıtlanmadıysa "yanıt vermiyor" (yanlış community/sürüm, ACL,
// SNMP kapalı, erişilemiyor); bazı yanıtlar geldiyse IF-MIB'in kapalı/boş
// olduğu ayrımı yapılır.
func snmpNoTelemetryErr(host string, walkOK int, sysOK bool, cause error) error {
	if walkOK == 0 && !sysOK {
		if cause == nil {
			cause = errors.New("zaman aşımı / yanıt yok")
		}
		return fmt.Errorf("SNMP yanıt vermiyor (%s) — community/sürüm, erişim listesi (ACL) veya SNMP servisini kontrol edin: %w", host, cause)
	}
	return fmt.Errorf("SNMP yanıt veriyor ama arayüz tablosu (IF-MIB) boş (%s)", host)
}

// connect, cihaz SNMP ayarlarina gore baglanti kurar (v2c/v3, sifreli alanlar decrypt).
func connect(d store.Device, v *vault.Vault) (*gosnmp.GoSNMP, error) {
	community, err := v.Decrypt(d.Community)
	if err != nil {
		return nil, fmt.Errorf("community decrypt: %w", err)
	}
	g := &gosnmp.GoSNMP{
		Target:  d.Host,
		Port:    161,
		Timeout: 5 * time.Second,
		Retries: 1,
		MaxOids: 50,
	}
	switch d.SNMPVersion {
	case 3:
		authPass, err := v.Decrypt(d.V3AuthPass)
		if err != nil {
			return nil, err
		}
		privPass, err := v.Decrypt(d.V3PrivPass)
		if err != nil {
			return nil, err
		}
		g.Version = gosnmp.Version3
		g.SecurityModel = gosnmp.UserSecurityModel
		g.MsgFlags = gosnmp.AuthPriv
		g.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 d.V3User,
			AuthenticationProtocol:   authProto(d.V3AuthProto),
			AuthenticationPassphrase: authPass,
			PrivacyProtocol:          privProto(d.V3PrivProto),
			PrivacyPassphrase:        privPass,
		}
	default:
		g.Version = gosnmp.Version2c
		g.Community = community
	}
	if err := g.Connect(); err != nil {
		return nil, err
	}
	return g, nil
}

func authProto(s string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(s) {
	case "MD5":
		return gosnmp.MD5
	case "SHA224":
		return gosnmp.SHA224
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.SHA
	}
}

func privProto(s string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(s) {
	case "DES":
		return gosnmp.DES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.AES
	}
}

func pduInt(p gosnmp.SnmpPDU) int64 {
	switch v := p.Value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint64:
		return int64(v)
	}
	return 0
}

func pduUint(p gosnmp.SnmpPDU) uint64 {
	switch v := p.Value.(type) {
	case uint64:
		return v
	case int:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case int64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	}
	return 0
}

func pduValueString(p gosnmp.SnmpPDU) string {
	switch v := p.Value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func ifIndexFromOID(name, base string) int64 {
	suffix := strings.TrimPrefix(name, "."+base+".")
	n, err := strconv.ParseInt(suffix, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
