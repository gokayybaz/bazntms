-- 0010_alert_lifecycle (PostgreSQL) — bkz. sqlite/0010_alert_lifecycle.sql.
-- Faz 22 S22.6: uyarı olayı → duruma. alert_events'e yaşam döngüsü alanları
-- (severity/state/site/count/first_ts/last_ts/resolved_ts/ack_*/note/group_id/
-- ext_ref). Kolonlar tümüyle yeni → her DB'de tam bir kez. Mevcut geçmiş
-- satırlar "resolved" işaretlenir; count/first_ts/last_ts geriye doldurulur.

ALTER TABLE alert_events ADD COLUMN severity    TEXT    NOT NULL DEFAULT 'warn';
ALTER TABLE alert_events ADD COLUMN state       TEXT    NOT NULL DEFAULT 'firing';
ALTER TABLE alert_events ADD COLUMN site        TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN count       BIGINT  NOT NULL DEFAULT 1;
ALTER TABLE alert_events ADD COLUMN first_ts    BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN last_ts     BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN resolved_ts BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN ack_by      TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN ack_ts      BIGINT  NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN note        TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN group_id    TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN ext_ref     TEXT    NOT NULL DEFAULT '';

UPDATE alert_events SET state = 'resolved', resolved_ts = ts, first_ts = ts, last_ts = ts
WHERE first_ts = 0;

CREATE INDEX IF NOT EXISTS idx_alert_events_state ON alert_events(state, last_ts DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_open  ON alert_events(kind, key, state);
CREATE INDEX IF NOT EXISTS idx_alert_events_group ON alert_events(group_id);
