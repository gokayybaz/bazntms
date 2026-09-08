-- 0012_scheduled_jobs (SQLite) — bkz. postgres/0012_scheduled_jobs.sql.
-- Faz 22 S22.18: hub-içi lider-kapılı zamanlayıcı. İlk tüketici zamanlanmış
-- rapor teslimi (S22.19), sonra günlük SLA değerlendirmesi (S22.21). Yeni
-- tablo → her DB'de bir kez. Bkz. docs/decisions/0009-scheduled-jobs.md.
--
--   kind         : "report" | "sla_eval" | ... (handler kaydı)
--   spec         : yineleme — "interval:<dk>" | "daily:<HH:MM>" |
--                  "weekly:<mon..sun>:<HH:MM>" | "monthly:<1..31>:<HH:MM>"
--   payload_json : handler'a geçen parametreler (rapor türü/pencere/alıcılar …)
--   next_run_ts  : bir sonraki koşum (unix); lider geçmiş bir değer görürse
--                  bir kez telafi eder ve ileriye kaydırır
CREATE TABLE IF NOT EXISTS scheduled_jobs (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	kind         TEXT    NOT NULL,
	spec         TEXT    NOT NULL,
	payload_json TEXT    NOT NULL DEFAULT '{}',
	enabled      INTEGER NOT NULL DEFAULT 1,
	last_run_ts  INTEGER NOT NULL DEFAULT 0,
	next_run_ts  INTEGER NOT NULL DEFAULT 0,
	last_status  TEXT    NOT NULL DEFAULT '',
	created_by   TEXT    NOT NULL DEFAULT '',
	created_ts   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_due ON scheduled_jobs(enabled, next_run_ts);
