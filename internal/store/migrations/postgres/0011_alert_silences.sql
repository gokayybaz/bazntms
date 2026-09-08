-- 0011_alert_silences (PostgreSQL) — bkz. sqlite/0011_alert_silences.sql.
-- Faz 22 S22.10: bakım pencereleri (kind/site/key-substring eşleşmesi +
-- zaman aralığı). Yeni tablo → her DB'de bir kez.

CREATE TABLE IF NOT EXISTS alert_silences (
	id          BIGSERIAL PRIMARY KEY,
	match_kind  TEXT   NOT NULL DEFAULT '',
	match_site  TEXT   NOT NULL DEFAULT '',
	match_key   TEXT   NOT NULL DEFAULT '',
	starts_ts   BIGINT NOT NULL,
	ends_ts     BIGINT NOT NULL,
	reason      TEXT   NOT NULL DEFAULT '',
	created_by  TEXT   NOT NULL DEFAULT '',
	created_ts  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_silences_window ON alert_silences(ends_ts);
