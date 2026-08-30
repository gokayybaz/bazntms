// Package devpoll, cihaz poll takvimini yürütür (Faz 8.1 refactor):
// toplama işi vendor driver'larına (internal/driver) devredilmiştir;
// bu paket yalnızca zamanlama ve Snapshot'ın depoya yazımını yapar.
package devpoll

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gokayybaz/bazntms/internal/driver"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// Poller, aktif cihazları takvime göre yoklar ve sonuçları kaydeder.
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

// pollDevice, driver'dan Snapshot alır ve depoya yazar (tek yazım noktası).
func (p *Poller) pollDevice(d store.Device) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	drv := driver.For(d)
	snap, err := drv.Poll(ctx, d, p.vault)
	if err != nil {
		p.store.UpdateDevicePoll(d.ID, snap.SysName, snap.SysDescr, err.Error())
		slog.Warn("cihaz poll basarisiz", "device", d.Name, "vendor", d.Vendor, "err", err)
		return
	}

	ts := time.Now().Unix()
	if len(snap.Ifaces) > 0 {
		if err := p.store.SaveDeviceIfaceSamples(d.ID, ts, snap.Ifaces); err != nil {
			slog.Warn("cihaz arayuz kaydi", "device", d.Name, "err", err)
		}
	}
	for _, r := range snap.Resources {
		if err := p.store.SaveDeviceResources(r); err != nil {
			slog.Warn("kaynak kaydi", "device", d.Name, "err", err)
		}
	}
	if len(snap.VPN) > 0 {
		if err := p.store.SaveFortiVPNStatus(d.ID, ts, snap.VPN); err != nil {
			slog.Warn("vpn durum kaydi", "device", d.Name, "err", err)
		}
	}
	if len(snap.SDWAN) > 0 {
		if err := p.store.SaveFortiSDWAN(d.ID, ts, snap.SDWAN); err != nil {
			slog.Warn("sdwan kaydi", "device", d.Name, "err", err)
		}
	}
	if len(snap.Policies) > 0 {
		if err := p.store.SaveFortiPolicyHits(d.ID, ts, snap.Policies); err != nil {
			slog.Warn("politika kaydi", "device", d.Name, "err", err)
		}
	}
	for _, l := range snap.Links {
		l.SourceID = d.ID
		if err := p.store.UpsertTopologyLink(l); err != nil {
			slog.Warn("topoloji kenari yazilamadi", "device", d.Name, "err", err)
		}
	}
	if err := p.store.UpdateDevicePoll(d.ID, snap.SysName, snap.SysDescr, snap.LastError); err != nil {
		slog.Warn("cihaz durum guncellemesi", "device", d.Name, "err", err)
	}
	slog.Info("poll tamam",
		"device", d.Name, "vendor", d.Vendor,
		"ifaces", len(snap.Ifaces), "resources", len(snap.Resources),
		"vpn", len(snap.VPN), "sdwan", len(snap.SDWAN), "policies", len(snap.Policies),
		"links", len(snap.Links),
	)
}
