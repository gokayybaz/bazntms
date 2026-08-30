// Package driver, vendor bazlı cihaz toplama soyutlamasıdır (Faz 8.1).
//
// DeviceDriver sözleşmesi: poll sonucu Snapshot'ta toplanır; kalıcılığı
// scheduler (devpoll) tek elden yapar. Böylece her driver yalnızca
// toplamaya odaklanır ve testleri store'suz yazılabilir.
//
// Mevcut implementasyonlar:
//   - SNMPDriver  : SNMPv2c/v3 IF-MIB poller + LLDP/CDP/ARP keşfi (Faz 3/6)
//   - FortiDriver : FortiGate REST API (Faz 8) — VPN, SD-WAN, politika, kaynak
//
// Yeni vendor eklemek: Driver arayüzünü gerçekleyin ve For() kaydına ekleyin.
package driver

import (
	"context"

	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// Snapshot, tek poll turunun tüm çıktısıdır. Alanlar vendor'a göre opsiyoneldir
// (ör. SNMP sadece Ifaces+Links doldurur).
type Snapshot struct {
	SysName   string
	SysDescr  string
	LastError string

	// ortak (snmp + fortigate)
	Ifaces []store.DeviceIface

	// FortiGate (Faz 8)
	Resources []store.DeviceResource
	VPN       []store.FortiVPNStatus
	SDWAN     []store.FortiSDWANSample
	Policies  []store.FortiPolicyHit

	// SNMP topoloji keşfi (Faz 6)
	Links []store.TopologyLink
}

// Driver, tek cihazın periyodik toplama sözleşmesidir.
// Poll hatasında hatayı döndürür; Snapshot'ta kısmi veri olabilir.
type Driver interface {
	Poll(ctx context.Context, d store.Device, v *vault.Vault) (Snapshot, error)
}

// For, cihaz vendor alanına göre uygun driver'ı döndürür.
// Bilinmeyen/boş vendor → SNMP (geriye uyumlu).
func For(d store.Device) Driver {
	switch d.Vendor {
	case "fortigate":
		return &FortiDriver{}
	default:
		return &SNMPDriver{}
	}
}
