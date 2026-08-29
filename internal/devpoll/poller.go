// Package devpoll, SNMP ile ag cihazlarini donemlik yoklar: arayuz
// sayaclari/durumu ve sistem bilgisi IF-MIB uzerinden alinir.
package devpoll

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
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

type Poller struct {
	store store.Store
	vault *vault.Vault
	stop  chan struct{}
	done  chan struct{}
}

func New(st store.Store, v *vault.Vault) *Poller {
	return &Poller{store: st, vault: v, stop: make(chan struct{}), done: make(chan struct{})}
}

func (p *Poller) Start() { go p.run() }
func (p *Poller) Stop()  { close(p.stop); <-p.done }

func (p *Poller) run() {
	defer close(p.done)
	ticker := time.NewTicker(10 * time.Second) // cihaz poll takvimi denetimi
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

func (p *Poller) pollAll() {
	devices, err := p.store.ListDevices()
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	for _, d := range devices {
		if !d.Enabled {
			continue
		}
		if time.Since(time.Unix(d.LastPoll, 0)) < time.Duration(d.PollSeconds)*time.Second {
			continue
		}
		wg.Add(1)
		go func(d store.Device) {
			defer wg.Done()
			p.pollDevice(d)
		}(d)
	}
	wg.Wait()
}

// pollDevice, tek cihazi yoklar; hata last_error'a yazilir.
func (p *Poller) pollDevice(d store.Device) {
	conn, err := connect(d, p.vault)
	if err != nil {
		p.store.UpdateDevicePoll(d.ID, d.SysName, d.SysDescr, err.Error())
		slog.Warn("snmp baglanamadi", "device", d.Name, "err", err)
		return
	}
	defer conn.Conn.Close()

	var ifaces []store.DeviceIface
	sysName, sysDescr := d.SysName, d.SysDescr

	// sistem bilgisi
	if res, err := conn.Get([]string{oidSysDescr, oidSysName}); err == nil {
		for _, pdu := range res.Variables {
			v := pduValueString(pdu)
			switch pdu.Name {
			case "." + oidSysDescr:
				sysDescr = truncate(v, 200)
			case "." + oidSysName:
				sysName = truncate(v, 100)
			}
		}
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
	for _, col := range cols {
		pdus, err := conn.BulkWalkAll(col.oid)
		if err != nil {
			continue // kolon desteklenmiyorsa atla
		}
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

	ts := time.Now().Unix()
	if err := p.store.SaveDeviceIfaceSamples(d.ID, ts, ifaces); err != nil {
		slog.Warn("cihaz ornek kaydi", "device", d.Name, "err", err)
	}
	if err := p.store.UpdateDevicePoll(d.ID, sysName, sysDescr, ""); err != nil {
		slog.Warn("cihaz durum guncellemesi", "device", d.Name, "err", err)
	}
	slog.Info("snmp poll tamam", "device", d.Name, "ifaces", len(ifaces))
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
