-- 0014_sla_targets (PostgreSQL) — bkz. sqlite/0014_sla_targets.sql.
-- Faz 22 S22.21: SLA hedefleri (global + per-site). Yeni tablo → bir kez.

CREATE TABLE IF NOT EXISTS sla_targets (
	scope             TEXT             NOT NULL DEFAULT 'global',
	site              TEXT             NOT NULL DEFAULT '',
	agent_uptime_pct  DOUBLE PRECISION NOT NULL DEFAULT 0,
	device_health_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
	iface_err_ceiling BIGINT           NOT NULL DEFAULT 0,
	updated_ts        BIGINT           NOT NULL DEFAULT 0,
	PRIMARY KEY (scope, site)
);
