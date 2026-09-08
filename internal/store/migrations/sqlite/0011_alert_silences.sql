-- 0011_alert_silences (SQLite) — bkz. postgres/0011_alert_silences.sql.
-- Faz 22 S22.10: bakım pencereleri. Bir eşleşme (kind / site / key-substring)
-- ve zaman aralığı; pencere aktifken eşleşen yeni uyarılar state='silenced'
-- kaydedilir ve bildirilmez. Yeni tablo → her DB'de bir kez.

CREATE TABLE IF NOT EXISTS alert_silences (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	match_kind  TEXT    NOT NULL DEFAULT '', -- boş = her tür
	match_site  TEXT    NOT NULL DEFAULT '', -- boş = her saha
	match_key   TEXT    NOT NULL DEFAULT '', -- boş = her anahtar; aksi alt-dize
	starts_ts   INTEGER NOT NULL,
	ends_ts     INTEGER NOT NULL,
	reason      TEXT    NOT NULL DEFAULT '',
	created_by  TEXT    NOT NULL DEFAULT '',
	created_ts  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alert_silences_window ON alert_silences(ends_ts);
