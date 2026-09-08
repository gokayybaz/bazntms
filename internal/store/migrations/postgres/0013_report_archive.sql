-- 0013_report_archive (PostgreSQL) — bkz. sqlite/0013_report_archive.sql.
-- Faz 22 S22.19: zamanlanmış rapor teslim geçmişi (dosya sistemi yol + meta).

CREATE TABLE IF NOT EXISTS report_archive (
	id           BIGSERIAL PRIMARY KEY,
	kind         TEXT    NOT NULL,
	site         TEXT    NOT NULL DEFAULT '',
	days         INTEGER NOT NULL DEFAULT 0,
	format       TEXT    NOT NULL DEFAULT 'html',
	path         TEXT    NOT NULL,
	size         BIGINT  NOT NULL DEFAULT 0,
	generated_ts BIGINT  NOT NULL,
	delivered_to TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'ok',
	job_id       BIGINT  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_report_archive_ts ON report_archive(generated_ts DESC);
