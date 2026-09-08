-- 0012_scheduled_jobs (PostgreSQL) — bkz. sqlite/0012_scheduled_jobs.sql.
-- Faz 22 S22.18: hub-içi lider-kapılı zamanlayıcı (zamanlanmış rapor + SLA).
-- Yeni tablo → her DB'de bir kez. Bkz. docs/decisions/0009-scheduled-jobs.md.

CREATE TABLE IF NOT EXISTS scheduled_jobs (
	id           BIGSERIAL PRIMARY KEY,
	kind         TEXT   NOT NULL,
	spec         TEXT   NOT NULL,
	payload_json TEXT   NOT NULL DEFAULT '{}',
	enabled      INTEGER NOT NULL DEFAULT 1,
	last_run_ts  BIGINT NOT NULL DEFAULT 0,
	next_run_ts  BIGINT NOT NULL DEFAULT 0,
	last_status  TEXT   NOT NULL DEFAULT '',
	created_by   TEXT   NOT NULL DEFAULT '',
	created_ts   BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_due ON scheduled_jobs(enabled, next_run_ts);
