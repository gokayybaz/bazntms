-- 0018_incidents (PostgreSQL) — bkz. sqlite/0018_incidents.sql.
-- Faz 24-B: incident korelasyon motoru. alert_events.agent_id + incidents +
-- incident_evidence.
ALTER TABLE alert_events ADD COLUMN agent_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_alert_events_agent ON alert_events(agent_id, last_ts DESC);

CREATE TABLE IF NOT EXISTS incidents (
	id                 BIGSERIAL PRIMARY KEY,
	title              TEXT   NOT NULL,
	severity           TEXT   NOT NULL DEFAULT 'warn',
	status             TEXT   NOT NULL DEFAULT 'open',
	site               TEXT   NOT NULL DEFAULT '',
	agent_id           BIGINT NOT NULL DEFAULT 0,
	correlation_key    TEXT   NOT NULL,
	correlation_reason TEXT   NOT NULL DEFAULT '',
	summary            TEXT   NOT NULL DEFAULT '',
	risk_score         BIGINT NOT NULL DEFAULT 0,
	created_ts         BIGINT NOT NULL,
	updated_ts         BIGINT NOT NULL,
	first_seen         BIGINT NOT NULL,
	last_seen          BIGINT NOT NULL,
	ack_by             TEXT   NOT NULL DEFAULT '',
	ack_ts             BIGINT NOT NULL DEFAULT 0,
	resolved_ts        BIGINT NOT NULL DEFAULT 0,
	ext_ref            TEXT   NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status, last_seen DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_incidents_open_corr
	ON incidents(correlation_key) WHERE status IN ('open','investigating');

CREATE TABLE IF NOT EXISTS incident_evidence (
	incident_id BIGINT NOT NULL,
	kind        TEXT   NOT NULL,
	ref         TEXT   NOT NULL,
	ts          BIGINT NOT NULL,
	summary     TEXT   NOT NULL DEFAULT '',
	PRIMARY KEY (incident_id, kind, ref)
);
