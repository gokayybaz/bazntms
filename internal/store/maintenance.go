package store

import (
	"log/slog"
	"strconv"
	"time"
)

// Maintainer, veritabani bakimini paket yakalamasindan BAGIMSIZ yurutur:
// eski zaman-serisi satirlarini (Prune) ve artik gorulmeyen topoloji
// kenarlarini (PruneTopology) periyodik siler. Eskiden bu is yalnizca
// Collector'in 10 dk tick'inde yapiliyordu; Collector ise sadece
// -capture=true iken calisir. Coklu-hub kurulumunda tum hub'lar
// -capture=false oldugu icin temizlik hic yapilmiyor, filo tablolari
// (agent_iface_samples, flows, process_traffic, device_iface_samples,
// syslog_events) sinirsiz buyuyordu.
//
// Coklu-replika kurulumda yalnizca TEK hub calistirmali (-prune bayragi
// ingest replikalarinda kapatilir); es zamanli DELETE'ler zararsiz ama
// gereksiz yuktur.
type Maintainer struct {
	st            Store
	retention     time.Duration
	topoRetention time.Duration
	interval      time.Duration
	stopCh        chan struct{}
	doneCh        chan struct{}
}

// NewMaintainer, verilen ham-veri saklama suresiyle bir bakim dongusu kurar.
// Topoloji kenarlari icin sabit 7 gun kullanilir (UI 24 saatlik pencere
// gosterir; 7 gun rahat bir tampon).
func NewMaintainer(st Store, retention time.Duration) *Maintainer {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &Maintainer{
		st:            st,
		retention:     retention,
		topoRetention: 7 * 24 * time.Hour,
		interval:      15 * time.Minute,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
}

func (m *Maintainer) Start() { go m.run() }

func (m *Maintainer) Stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *Maintainer) run() {
	defer close(m.doneCh)
	m.once() // acilista bir kez: uzun sure kapali kalmis kurulumda birikeni hemen temizle
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-t.C:
			m.once()
		}
	}
}

func (m *Maintainer) once() {
	start := time.Now()
	if err := m.st.Prune(m.retention); err != nil {
		slog.Warn("bakim: prune hatasi", "err", err)
	}
	if err := m.st.PruneTopology(m.topoRetention); err != nil {
		slog.Warn("bakim: topoloji prune hatasi", "err", err)
	}
	slog.Debug("bakim tamamlandi", "sure", time.Since(start), "retention_saat", int(m.retention.Hours()))
}

// ConfigureRetention, TimescaleDB modunda tum ham zaman-serisi hypertable'lari
// icin native retention politikasi (verimli chunk-drop) kurar. Politika zaten
// varsa pencere degismis olabilir → kaldirilip yeniden eklenir. Plain
// PostgreSQL / SQLite modunda no-op — o modlarda Maintainer.Prune yeterli
// (ama TS'te DELETE chunk'lari birakmaz, bu yuzden native politika sart).
func (s *sqlStore) ConfigureRetention(retention time.Duration) error {
	if !s.ts {
		return nil
	}
	secs := int64(retention.Seconds())
	if secs < 3600 {
		secs = 3600
	}
	// caggs (samples_1m/1h) ve dusuk hacimli cihaz/forti tablolari haric ham
	// yuksek hacimli hypertable'lar — hepsi ayni pencere (-retention-hours).
	for _, tbl := range []string{
		"samples", "endpoint_stats", "connection_events", "dns_queries",
		"agent_iface_samples", "process_traffic", "flows",
		"device_iface_samples", "syslog_events",
	} {
		_, _ = s.db.Exec(`SELECT remove_retention_policy('` + tbl + `', if_exists => true)`)
		if _, err := s.db.Exec(
			`SELECT add_retention_policy('` + tbl + `', drop_after => ` + strconv.FormatInt(secs, 10) +
				`::BIGINT, schedule_interval => INTERVAL '1 hour')`); err != nil {
			slog.Warn("retention politikasi kurulamadi", "tablo", tbl, "err", err.Error())
		}
	}
	slog.Info("timescaledb retention politikalari guncellendi", "pencere_saat", secs/3600)
	return nil
}
