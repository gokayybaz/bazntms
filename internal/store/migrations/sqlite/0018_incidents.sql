-- 0018_incidents (SQLite) — bkz. postgres/0018_incidents.sql.
-- Faz 24-B: TELEMETRY → EVENT → ALERT → INCIDENT. İlişkili uyarılar/olaylar
-- deterministik kurallarla bir olaya (incident) toplanır.
--
-- alert_events.agent_id: korelasyon "aynı agent" ölçütü için. Uyarı keyleri
-- (proc → "ad:süreç", bw → "agent-in:ad", ioc → "id|domain") tutarsız →
-- fireCtx artık agent id'yi doğrudan yazar. Eski satırlar 0.
ALTER TABLE alert_events ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_alert_events_agent ON alert_events(agent_id, last_ts DESC);

CREATE TABLE IF NOT EXISTS incidents (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	title              TEXT    NOT NULL,
	severity           TEXT    NOT NULL DEFAULT 'warn',  -- info | warn | crit
	status             TEXT    NOT NULL DEFAULT 'open',  -- open | investigating | resolved | closed
	site               TEXT    NOT NULL DEFAULT '',
	agent_id           INTEGER NOT NULL DEFAULT 0,
	correlation_key    TEXT    NOT NULL,                 -- dedup: kural + kapsam
	correlation_reason TEXT    NOT NULL DEFAULT '',
	summary            TEXT    NOT NULL DEFAULT '',
	risk_score         INTEGER NOT NULL DEFAULT 0,       -- 0-100
	created_ts         INTEGER NOT NULL,
	updated_ts         INTEGER NOT NULL,
	first_seen         INTEGER NOT NULL,
	last_seen          INTEGER NOT NULL,
	ack_by             TEXT    NOT NULL DEFAULT '',
	ack_ts             INTEGER NOT NULL DEFAULT 0,
	resolved_ts        INTEGER NOT NULL DEFAULT 0,
	ext_ref            TEXT    NOT NULL DEFAULT ''        -- bilet kimliği
);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status, last_seen DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_open_corr
	ON incidents(correlation_key) WHERE status IN ('open','investigating');

CREATE TABLE IF NOT EXISTS incident_evidence (
	incident_id INTEGER NOT NULL,
	kind        TEXT    NOT NULL,   -- alert | event
	ref         TEXT    NOT NULL,   -- alert_events.id | olay tanımlayıcısı
	ts          INTEGER NOT NULL,
	summary     TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (incident_id, kind, ref)
);
