-- 0010_alert_lifecycle (SQLite) — bkz. postgres/0010_alert_lifecycle.sql.
-- Faz 22 S22.6: uyarı olayı → duruma. alert_events'e yaşam döngüsü alanları:
--   severity   : info | warn | crit
--   state      : firing | ack | resolved | silenced
--   site       : saha kapsamı (site-admin filtresi için)
--   count      : aynı (kind,key) açık olayın tekrar sayısı
--   first_ts   : ilk ateşlenme · last_ts: son tekrar · resolved_ts: çözülme
--   ack_by/ack_ts/note : kabul eden operatör + not
--   group_id   : korelasyon (S22.9) · ext_ref: bilet kimliği (S22.13/S22.14)
--
-- Kolonlar tümüyle yeni → her DB'de tam bir kez düz ALTER (SQLite'ta
-- "ADD COLUMN IF NOT EXISTS" yok ama 0001_init'te bu kolonlar hiç yoktu).
-- Mevcut geçmiş satırlar "resolved" işaretlenir (yaşam döngüsü onlardan sonra
-- başladı); count/first_ts/last_ts geriye doldurulur.

ALTER TABLE alert_events ADD COLUMN severity    TEXT    NOT NULL DEFAULT 'warn';
ALTER TABLE alert_events ADD COLUMN state       TEXT    NOT NULL DEFAULT 'firing';
ALTER TABLE alert_events ADD COLUMN site        TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN count       INTEGER NOT NULL DEFAULT 1;
ALTER TABLE alert_events ADD COLUMN first_ts    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN last_ts     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN resolved_ts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN ack_by      TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN ack_ts      INTEGER NOT NULL DEFAULT 0;
ALTER TABLE alert_events ADD COLUMN note        TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN group_id    TEXT    NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN ext_ref     TEXT    NOT NULL DEFAULT '';

UPDATE alert_events SET state = 'resolved', resolved_ts = ts, first_ts = ts, last_ts = ts
WHERE first_ts = 0;

CREATE INDEX IF NOT EXISTS idx_alert_events_state ON alert_events(state, last_ts DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_open  ON alert_events(kind, key, state);
CREATE INDEX IF NOT EXISTS idx_alert_events_group ON alert_events(group_id);
