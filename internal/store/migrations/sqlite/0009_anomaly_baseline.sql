-- 0009_anomaly_baseline (SQLite) — bkz. postgres/0009_anomaly_baseline.sql.
-- Faz 22 S22.1: anomali motorunun istatistiksel baseline'ı materyalize edilir.
-- Önceden checkAnomaly her değerlendirmede (5 dk) agent_iface_samples üstünde
-- LAG penceresi taraması yapıyordu — 5.000 agent × 7 gün ölçekte ciddi maliyet.
-- Artık lider-kapılı saatlik bir rebuild bu tabloyu doldurur, değerlendirme
-- yalnızca buradan okur.
--
--   dim    : baseline boyutu — 'fleet' (agent telemetrisi) | 'local' (hub yerel
--            yakalaması). S22.3'te 'site' | 'agent' eklenecek.
--   metric : ölçülen büyüklük — 'bps'. S22.4'te 'dns_qps' | 'proc_bytes' |
--            'conn_count' eklenecek.
--   key    : boyut anahtarı — fleet/local için '' ; site adı / agent id (S22.3).
--   bucket : mevsimsel kova — S22.1'de saat-of-day (0-23); S22.2'de
--            haftaiçi/haftasonu × saat.
--   n/mean/m2 : Welford momentleri — std = sqrt(m2/n) (popülasyon varyansı,
--            mevcut AVG(x^2)-AVG(x)^2 ile aynı).
CREATE TABLE IF NOT EXISTS anomaly_baseline (
	dim        TEXT    NOT NULL,
	metric     TEXT    NOT NULL,
	key        TEXT    NOT NULL DEFAULT '',
	bucket     INTEGER NOT NULL,
	n          INTEGER NOT NULL,
	mean       REAL    NOT NULL,
	m2         REAL    NOT NULL,
	updated_ts INTEGER NOT NULL,
	PRIMARY KEY (dim, metric, key, bucket)
);
