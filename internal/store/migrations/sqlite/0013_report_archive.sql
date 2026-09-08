-- 0013_report_archive (SQLite) — bkz. postgres/0013_report_archive.sql.
-- Faz 22 S22.19: zamanlanmış rapor teslim geçmişi. Üretilen rapor dosya
-- sistemine yazılır (<data>/reports/), yol + meta burada tutulur (Karar 4 —
-- DB'yi PDF blob'larıyla şişirme). Yeni tablo → her DB'de bir kez.

CREATE TABLE IF NOT EXISTS report_archive (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	kind         TEXT    NOT NULL,           -- enterprise | compliance | traffic
	site         TEXT    NOT NULL DEFAULT '',
	days         INTEGER NOT NULL DEFAULT 0,
	format       TEXT    NOT NULL DEFAULT 'html',
	path         TEXT    NOT NULL,           -- <data>/reports/... (mutlak)
	size         INTEGER NOT NULL DEFAULT 0,
	generated_ts INTEGER NOT NULL,
	delivered_to TEXT    NOT NULL DEFAULT '',
	status       TEXT    NOT NULL DEFAULT 'ok',
	job_id       INTEGER NOT NULL DEFAULT 0  -- scheduled_jobs.id (0 = elle)
);
CREATE INDEX IF NOT EXISTS idx_report_archive_ts ON report_archive(generated_ts DESC);
