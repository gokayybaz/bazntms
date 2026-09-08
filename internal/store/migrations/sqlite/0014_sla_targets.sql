-- 0014_sla_targets (SQLite) — bkz. postgres/0014_sla_targets.sql.
-- Faz 22 S22.21: SLA hedefleri. scope='global' tek satır (site=''); scope='site'
-- her saha için bir satır (site dolu). Kurumsal rapor hedef-vs-gerçek + ihlal
-- vurgusu; alert motoru günlük 'sla_breach' uyarısı. Yeni tablo → bir kez.
--
--   agent_uptime_pct    : online agent oranı tabanı (0 = kontrol yok)
--   device_health_pct   : sağlıklı SNMP/Forti cihaz oranı tabanı
--   iface_err_ceiling   : 24 saatte izin verilen toplam arayüz iskarta+hata (0 = yok)

CREATE TABLE IF NOT EXISTS sla_targets (
	scope             TEXT    NOT NULL DEFAULT 'global',
	site              TEXT    NOT NULL DEFAULT '',
	agent_uptime_pct  REAL    NOT NULL DEFAULT 0,
	device_health_pct REAL    NOT NULL DEFAULT 0,
	iface_err_ceiling INTEGER NOT NULL DEFAULT 0,
	updated_ts        INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (scope, site)
);
