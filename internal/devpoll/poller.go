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
	"github.com/gokayybaz/bazntms/internal/metrics"
	"github.com/gokayybaz/bazntms/internal/store"
	"github.com/gokayybaz/bazntms/internal/vault"
)

// defaultConcurrency, tek poll döngüsünde eşzamanlı yoklanan cihaz sayısı
// tavanı (S21.10 — 1000 cihazda sınırsız goroutine yerine bounded havuz).
const defaultConcurrency = 96

// Poller, aktif cihazları takvime göre yoklar ve sonuçları kaydeder.
type Poller struct {
	store store.Store
	vault *vault.Vault
	// override > 0 ise tüm cihazlar için per-device PollSeconds yerine bu aralık
	// kullanılır (tek tip poll takvimi).
	override time.Duration
	// isLeader, çoklu controller replikasında yalnız liderin poll etmesi için
	// (C1, Faz 15). nil → daima poll eder.
	isLeader    func() bool
	concurrency int
	stop        chan struct{}
	done        chan struct{}
}

// SetLeaderCheck, poll öncesi çağrılacak liderlik denetimini bağlar.
func (p *Poller) SetLeaderCheck(fn func() bool) { p.isLeader = fn }

func New(st store.Store, v *vault.Vault) *Poller {
	return &Poller{store: st, vault: v, concurrency: defaultConcurrency, stop: make(chan struct{}), done: make(chan struct{})}
}

// SetConcurrency, tek poll döngüsündeki eşzamanlı yoklama tavanını ayarlar
// (0 veya negatif → varsayılan). Büyük filolarda (1000+ cihaz) SNMP timeout
// bütçesini poll döngüsünde tutmak için.
func (p *Poller) SetConcurrency(n int) {
	if n <= 0 {
		n = defaultConcurrency
	}
	p.concurrency = n
}

// SetInterval, tüm cihazlar için tek tip poll aralığı dayatır (0 = per-device).
// 5 sn'nin altı kabul edilmez.
func (p *Poller) SetInterval(d time.Duration) {
	if d > 0 && d < 5*time.Second {
		d = 5 * time.Second
	}
	p.override = d
}

// interval, bir cihaz için etkin poll aralığını döndürür.
func (p *Poller) interval(d store.Device) time.Duration {
	if p.override > 0 {
		return p.override
	}
	return time.Duration(d.PollSeconds) * time.Second
}

func (p *Poller) Start() { go p.run() }
func (p *Poller) Stop()  { close(p.stop); <-p.done }

func (p *Poller) run() {
	defer close(p.done)
	tick := 10 * time.Second // cihaz poll takvimi denetim sıklığı
	if p.override > 0 && p.override < 2*tick {
		tick = p.override / 2
	}
	ticker := time.NewTicker(tick)
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
	if p.isLeader != nil && !p.isLeader() {
		return // C1: çoklu replikada yalnız lider poll eder (cihaz başına tek poll)
	}
	devices, err := p.store.ListDevices("")
	if err != nil {
		return
	}
	defer metrics.ObserveDevpollCycle(time.Now())

	limit := p.concurrency
	if limit <= 0 {
		limit = defaultConcurrency
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, d := range devices {
		if !d.Enabled {
			continue
		}
		if time.Since(time.Unix(d.LastPoll, 0)) < p.interval(d) {
			continue
		}
		select {
		case <-p.stop:
			wg.Wait()
			return
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(d store.Device) {
			defer wg.Done()
			defer func() { <-sem }()
			p.pollDevice(d)
		}(d)
	}
	wg.Wait()
}

// pollDevice, driver'dan Snapshot alır ve depoya yazar (tek yazım noktası).
func (p *Poller) pollDevice(d store.Device) {
	metrics.AddDevpollInflight(1)
	defer metrics.AddDevpollInflight(-1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	drv := driver.For(d)

	// gosnmp context'i dinlemez; takılan bir poll'un tüm zamanlayıcıyı
	// dondurmaması için son teslim tarihini burada dayatıyoruz. Aşılırsa
	// goroutine sızar ama sonucu yok sayılır.
	type result struct {
		snap driver.Snapshot
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		snap, err := drv.Poll(ctx, d, p.vault)
		ch <- result{snap, err}
	}()

	var snap driver.Snapshot
	var err error
	select {
	case r := <-ch:
		snap, err = r.snap, r.err
	case <-ctx.Done():
		msg := "poll zaman aşımı (2 dk)"
		if err := p.store.UpdateDevicePoll(d.ID, "", "", msg); err != nil {
			slog.Warn("cihaz poll durumu yazilamadi", "device", d.Name, "err", err)
		}
		slog.Warn("cihaz poll basarisiz", "device", d.Name, "vendor", d.Vendor, "err", msg)
		return
	}

	if err != nil {
		if uerr := p.store.UpdateDevicePoll(d.ID, snap.SysName, snap.SysDescr, err.Error()); uerr != nil {
			slog.Warn("cihaz poll durumu yazilamadi", "device", d.Name, "err", uerr)
		}
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
	// FortiGate: tespit edilen FortiOS sürümünü tazele (sürüm profili + rozet).
	// Uç yetenek özeti "Bağlantıyı Sına"dan gelir — poll ezmesin diye boş geçilir.
	if snap.APIVersion != "" {
		if err := p.store.UpdateDeviceFortiMeta(d.ID, snap.APIVersion, ""); err != nil {
			slog.Warn("fortigate meta guncellemesi", "device", d.Name, "err", err)
		}
	}
	slog.Info("poll tamam",
		"device", d.Name, "vendor", d.Vendor,
		"ifaces", len(snap.Ifaces), "resources", len(snap.Resources),
		"vpn", len(snap.VPN), "sdwan", len(snap.SDWAN), "policies", len(snap.Policies),
		"links", len(snap.Links),
	)
}
