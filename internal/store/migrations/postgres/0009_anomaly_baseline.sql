-- 0009_anomaly_baseline (PostgreSQL) — bkz. sqlite/0009_anomaly_baseline.sql.
-- Faz 22 S22.1: anomali motorunun istatistiksel baseline'ı materyalize edilir.
-- Önceden checkAnomaly her değerlendirmede agent_iface_samples üstünde LAG
-- penceresi taraması yapıyordu (5.000 agent × 7 gün → pahalı). Artık
-- lider-kapılı saatlik rebuild doldurur, değerlendirme buradan okur.
--
--   dim    : 'fleet' | 'local' (S22.3'te 'site' | 'agent')
--   metric : 'bps' (S22.4'te 'dns_qps' | 'proc_bytes' | 'conn_count')
--   key    : boyut anahtarı — fleet/local için '' ; site adı / agent id (S22.3)
--   bucket : saat-of-day (S22.1) / haftaiçi-haftasonu × saat (S22.2)
--   n/mean/m2 : Welford momentleri — std = sqrt(m2/n)
CREATE TABLE IF NOT EXISTS anomaly_baseline (
	dim        TEXT             NOT NULL,
	metric     TEXT             NOT NULL,
	key        TEXT             NOT NULL DEFAULT '',
	bucket     INTEGER          NOT NULL,
	n          BIGINT           NOT NULL,
	mean       DOUBLE PRECISION NOT NULL,
	m2         DOUBLE PRECISION NOT NULL,
	updated_ts BIGINT           NOT NULL,
	PRIMARY KEY (dim, metric, key, bucket)
);
