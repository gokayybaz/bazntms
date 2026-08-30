package driver

// FortiDriver — FortiGate REST API toplaması (Faz 8.3/8.4).
//
// Poll turunda: sistem durumu, kaynak örnekleri, arayüz sayaçları,
// VPN (IPsec+SSL), SD-WAN health-check ve politika hit sayaçları toplanır.
// VDOM: d.VDOM boş → root; "all" → tüm vdomlar (sonuçlar vdom etiketli).
// Her uç best-effort'tur: bir uç başarısız olursa diğerleri devam eder.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gokayybaz/bazntms/internal/fortigate"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// slogPollWarn, tek uç başarısızlığını turu bozmadan loglar.
func slogPollWarn(device, endpoint string, err error) {
	slog.Debug("fortigate uç toplama hatası", "device", device, "endpoint", endpoint, "err", err)
}

// FortiDriver, FortiGate REST tabanlı cihaz toplaması.
type FortiDriver struct{}

// Poll, tek FortiGate cihazını toplar.
func (f *FortiDriver) Poll(ctx context.Context, d store.Device, v *vault.Vault) (Snapshot, error) {
	snap := Snapshot{}

	if d.APIURL == "" {
		err := errors.New("fortigate api_url yok (cihaz ayarlarını tamamlayın)")
		snap.LastError = err.Error()
		return snap, err
	}
	token, err := v.Decrypt(d.APIToken)
	if err != nil || token == "" {
		err = fmt.Errorf("api token decrypt: %w", err)
		snap.LastError = err.Error()
		return snap, err
	}

	fc := fortigate.New(fortigate.Options{
		BaseURL:   d.APIURL,
		Token:     token,
		VerifyTLS: d.APIVerifyTLS,
		Timeout:   20 * time.Second,
	})

	// sistem durumu: sysName/sysDescr
	if st, err := fc.SystemStatus(ctx); err == nil {
		snap.SysName = st.Hostname
		descr := "FortiOS"
		if st.Version != "" {
			descr += " " + st.Version
		}
		if st.Serial != "" {
			descr += ", serial " + st.Serial
		}
		snap.SysDescr = descr
	} else {
		slogPollWarn(d.Name, "system/status", err)
	}

	// vdom havuzu
	vdoms := []string{"root"}
	switch d.VDOM {
	case "all":
		if vs, err := fc.VDOMs(ctx); err == nil && len(vs) > 0 {
			vdoms = vs
		}
	case "", "root":
	default:
		vdoms = []string{d.VDOM}
	}

	now := time.Now().Unix()
	for _, vdom := range vdoms {
		f.pollVdom(ctx, fc, d, vdom, now, &snap)
	}

	if snap.Ifaces == nil && snap.Resources == nil && snap.VPN == nil {
		err := errors.New("fortigate toplama boş döndü (api_url/token/erişim kontrol edin)")
		snap.LastError = err.Error()
		return snap, err
	}
	return snap, nil
}

// pollVdom, tek vdom için tüm uçları toplar; hatalar debug amaçlı toplanıp
// yutulur (bir uç desteklenmiyorsa turu bozmamalı).
func (f *FortiDriver) pollVdom(ctx context.Context, fc *fortigate.Client, d store.Device, vdom string, now int64, snap *Snapshot) {
	// kaynak kullanımı: en güncel örnek (dizi döner; son elemanı alırız)
	if samples, err := fc.ResourceUsage(ctx, vdom, "1minute"); err == nil && len(samples) > 0 {
		last := samples[len(samples)-1]
		ts := last.Time
		if ts == 0 {
			ts = now
		}
		snap.Resources = append(snap.Resources, store.DeviceResource{
			Ts: ts, DeviceID: d.ID,
			CPUPct: last.CPU, MemPct: last.Mem, DiskPct: last.Disk,
			Sessions: int64(last.Session),
		})
	}

	// arayüzler
	if ifaces, err := fc.Interfaces(ctx, vdom); err == nil {
		for _, i := range ifaces {
			snap.Ifaces = append(snap.Ifaces, store.DeviceIface{
				IfIndex:     i.ID,
				Name:        i.Name,
				Alias:       i.Alias,
				Speed:       i.SpeedBps(),
				OperStatus:  operStatus(i.Status),
				RxBytes:     i.RxBytes,
				TxBytes:     i.TxBytes,
				InErrors:    i.RxErrors,
				OutErrors:   i.TxErrors,
				InDiscards:  i.RxDrops,
				OutDiscards: i.TxDrops,
			})
		}
	}

	// VPN: IPsec tüneller + SSL kullanıcıları
	if tunnels, err := fc.IPsecTunnels(ctx, vdom); err == nil {
		for _, t := range tunnels {
			snap.VPN = append(snap.VPN, store.FortiVPNStatus{
				DeviceID: d.ID, VDOM: vdom, Kind: "ipsec",
				Name: t.Name, Peer: t.Peer, Status: vpnStatus(t.Status),
				RxBytes: t.RxBytes, TxBytes: t.TxBytes, Ts: now,
			})
		}
	}
	if sessions, err := fc.SSLVPNSessions(ctx, vdom); err == nil {
		for _, s := range sessions {
			snap.VPN = append(snap.VPN, store.FortiVPNStatus{
				DeviceID: d.ID, VDOM: vdom, Kind: "ssl",
				Name: s.User, Peer: s.RemoteHost,
				Status: vpnStatus(orDefault(s.Status, "up")),
				Uptime: s.Uptime, RxBytes: s.RxBytes, TxBytes: s.TxBytes, Ts: now,
			})
		}
	}

	// SD-WAN health-check üyeleri
	if health, err := fc.SDWANHealth(ctx, vdom); err == nil {
		for hc, members := range health {
			for _, m := range members {
				snap.SDWAN = append(snap.SDWAN, store.FortiSDWANSample{
					Ts: now, DeviceID: d.ID, VDOM: vdom,
					Member: m.Member, HealthCheck: hc,
					LatencyMs: m.LatencyMs, JitterMs: m.JitterMs,
					PacketLossPct: m.PacketLossPct,
					State:         vpnStatus(orDefault(m.State, m.Status)),
				})
			}
		}
	}

	// politika hit sayaçları (kümülatif; delta sorguda hesaplanır)
	if policies, err := fc.Policies(ctx, vdom); err == nil {
		for _, p := range policies {
			snap.Policies = append(snap.Policies, store.FortiPolicyHit{
				Ts: now, DeviceID: d.ID, VDOM: vdom,
				PolicyID: p.PolicyID, Name: p.Name, Action: p.Action,
				Hits: p.Hits, Bytes: p.Bytes,
			})
		}
	}
}

func operStatus(s string) int {
	if strings.EqualFold(strings.TrimSpace(s), "up") {
		return 1
	}
	return 0
}

// vpnStatus, FortiGate durum değerlerini bizim şemamıza normalize eder;
// bilinmeyen değerler olduğu gibi geçer (UI renk kodu için).
func vpnStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "up", "connected", "ready":
		return "up"
	case "down", "disconnected":
		return "down"
	case "connecting", "pending", "dialing":
		return "connecting"
	case "":
		return "unknown"
	}
	return s
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
