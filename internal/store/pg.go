package store

// PostgreSQL/TimescaleDB arka ucu (Faz 4.1 + 4.3).
//
// Sema, dialect basina ayri gomulu migrasyon dosyalariyla kurulur
// (internal/store/migrations/postgres/*.sql — bkz. migrate.go). SQLite
// surumunden farklari:
//   - id kolonlari BIGSERIAL (SQLite AUTOINCREMENT karsiligi); hypertable'a
//     cevrilecek tablolarda birlesik PK (id, ts) zorunludur
//   - tamsayi sayclar BIGINT, gercek sayilar DOUBLE PRECISION
//   - zaman kolonlari yine unix saniye (BIGINT) — TimescaleDB integer-tabanli
//     hypertable destekler (chunk_time_interval ve time_bucket tamsayi)
//
// TimescaleDB kuruluysa (best-effort, migrasyon disi — setupTimescale):
//   - agir zaman serisi tablolari hypertable'a cevrilir
//   - samples uzerine 1dk (samples_1m) ve 1sa (samples_1h) continuous
//     aggregate'lari kurulur
//   - retention politikalari: ham 7g → 1dk 90g → 1sa 2y
//   - TimescaleDB yoksa duz PostgreSQL calisir; temizlik Prune ile yapilir

import (
	"database/sql"
	"log/slog"
	"strconv"
)

// migrateLockKey, coklu hub replikasinin (hub-controller + N x hub-ingest,
// bkz. deploy/docker-compose.scale.yml) ayni TAZE veritabanina karsi ayni
// anda baslamasi durumunda es zamanli DDL'leri serilestirmek icin
// kullanilan Postgres advisory lock anahtari (rastgele sabit bir sayi,
// baska hicbir anlami yok). Migrasyon runner'i (migrate.go) tum isini bu
// kilit altinda tek baglantida yapar.
const migrateLockKey = 8823001

// setupTimescale, TimescaleDB eklentisi varsa olcek altyapisini kurar:
// hypertable'lar, continuous aggregate'lar ve retention politikaları.
// Her adim best-effort'tur; TimescaleDB surumu bir ozellik desteklemiyorsa
// loglanir ve duz PostgreSQL moduyla devam edilir. Donduren deger eklentinin
// aktif olup olmadigidir.
func setupTimescale(db *sql.DB) bool {
	_, _ = db.Exec(`CREATE EXTENSION IF NOT EXISTS timescaledb`) // superuser gerekebilir
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pg_extension WHERE extname = 'timescaledb'`).Scan(&n); err != nil || n == 0 {
		slog.Info("timescaledb yok — duz PostgreSQL modunda calisiyor (temizlik Prune ile)")
		return false
	}

	// integer-tabanli hypertable politikalarinin "simdi" fonksiyonu
	tsTry(db, `CREATE OR REPLACE FUNCTION bazntms_ts_now() RETURNS BIGINT
		LANGUAGE SQL STABLE AS $f$ SELECT EXTRACT(EPOCH FROM clock_timestamp())::BIGINT $f$`,
		"now fonksiyonu")

	// ham zaman serisi tablolari → hypertable
	// chunk_time_interval saniye birimindedir (integer hypertable)
	for _, h := range []struct {
		table string
		chunk int64
	}{
		{"samples", 6 * 3600},
		{"endpoint_stats", 24 * 3600},
		{"dns_queries", 24 * 3600},
		{"connection_events", 24 * 3600},
		{"agent_iface_samples", 24 * 3600},
		{"process_traffic", 24 * 3600},
		{"l7_endpoints", 24 * 3600},
		{"agent_dns", 24 * 3600},
		{"device_iface_samples", 24 * 3600},
		{"flows", 3600},
		{"syslog_events", 3600},
	} {
		tsTry(db, `SELECT create_hypertable('`+h.table+`', 'ts',
			chunk_time_interval => `+strconv.FormatInt(h.chunk, 10)+`,
			if_not_exists => TRUE, migrate_data => TRUE)`,
			"hypertable "+h.table)
		tsTry(db, `SELECT set_integer_now_func('`+h.table+`', 'bazntms_ts_now')`,
			"integer_now "+h.table)
	}

	// continuous aggregate'lar (downsample): 1dk ve 1sa kovalar
	// materialized_only=false → gerçek-zamanlı agregasyon: ham veri hemen görünür
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS samples_1m
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(60, ts) AS bucket, device,
			AVG(bps_in) AS avg_bps_in, AVG(bps_out) AS avg_bps_out,
			AVG(bps_local) AS avg_bps_local, SUM(pps) AS pps, SUM(dropped) AS dropped
		FROM samples GROUP BY bucket, device WITH NO DATA`,
		"continuous aggregate samples_1m")
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS samples_1h
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(3600, ts) AS bucket, device,
			AVG(bps_in) AS avg_bps_in, AVG(bps_out) AS avg_bps_out,
			AVG(bps_local) AS avg_bps_local, SUM(pps) AS pps, SUM(dropped) AS dropped
		FROM samples GROUP BY bucket, device WITH NO DATA`,
		"continuous aggregate samples_1h")

	// NetFlow/IPFIX akis ozeti: (cihaz, protokol) basina saatlik toplam. Ham
	// `flows` 7 gunde dusuyor (ConfigureRetention); bu cagg protokol/hacim
	// trendini 1 yil tutar. src/dst KASITLI grupta degil — yuksek kardinalite
	// diski sisirir; "en yogun uc noktalar" ham tablonun 7 gunluk penceresinden
	// gelmeye devam eder.
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS flows_1h
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(3600, ts) AS bucket, device, proto,
			SUM(octets) AS octets, SUM(packets) AS packets, COUNT(*) AS flows
		FROM flows GROUP BY bucket, device, proto WITH NO DATA`,
		"continuous aggregate flows_1h")

	// Sürec bazli bant genisligi trendi: (agent, sürec) basina saatlik toplam
	// (S21.12). process_traffic 5000 agent'ta yuksek hacimli; ham tablo
	// `-retention-hours`'da (vars. 7g) duser, bu cagg trendi 1 yil tutar →
	// kapasite raporu (30/90g) sürec kirilimini kaybetmez. remote_ip KASITLI
	// grupta degil (kardinalite) — "en yogun uzak uç" ham 7g penceresinden.
	tsTry(db, `CREATE MATERIALIZED VIEW IF NOT EXISTS process_traffic_1h
		WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
		SELECT time_bucket(3600, ts) AS bucket, agent_id, process,
			SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out
		FROM process_traffic GROUP BY bucket, agent_id, process WITH NO DATA`,
		"continuous aggregate process_traffic_1h")

	// cagg yenileme politikalari
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1m',
		start_offset => 7200::BIGINT, end_offset => 60::BIGINT,
		schedule_interval => INTERVAL '5 minutes')`,
		"cagg policy samples_1m")
	tsTry(db, `SELECT add_continuous_aggregate_policy('samples_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy samples_1h")
	tsTry(db, `SELECT add_continuous_aggregate_policy('flows_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy flows_1h")
	tsTry(db, `SELECT add_continuous_aggregate_policy('process_traffic_1h',
		start_offset => 172800::BIGINT, end_offset => 3600::BIGINT,
		schedule_interval => INTERVAL '1 hour')`,
		"cagg policy process_traffic_1h")

	// downsample cagg'leri icin retention: 1dk kova 90g, 1sa kova 2y.
	// Param adi `drop_after` (eski `retain_after` TS 2.x'te YOK — sessizce
	// fail ediyordu). Ham hypertable'larin retention'i store.ConfigureRetention
	// tarafindan `-retention-hours`'a gore kurulur (acilista, hub main'den).
	tsTry(db, `SELECT add_retention_policy('samples_1m', drop_after => 7776000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention samples_1m (90g)")
	tsTry(db, `SELECT add_retention_policy('samples_1h', drop_after => 63072000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention samples_1h (2y)")
	tsTry(db, `SELECT add_retention_policy('flows_1h', drop_after => 31536000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention flows_1h (1y)")
	tsTry(db, `SELECT add_retention_policy('process_traffic_1h', drop_after => 31536000::BIGINT,
		schedule_interval => INTERVAL '1 hour', if_not_exists => true)`, "retention process_traffic_1h (1y)")

	slog.Info("timescaledb aktif — hypertable, downsample ve retention politikaları kuruldu")
	return true
}

// tsTry, TimescaleDB kurulum ifadesini calistirir; hata olursa loglayip
// devam eder (best-effort, surum farkliliklarina toleransli).
func tsTry(db *sql.DB, stmt, label string) {
	if _, err := db.Exec(stmt); err != nil {
		slog.Warn("timescale kurulum adimi atlandi", "adim", label, "err", err.Error())
	}
}
