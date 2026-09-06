-- 0005_sessions (PostgreSQL) — Faz 15 S15.3 (A4).
-- Bkz. sqlite/0005_sessions.sql + docs/decisions/0004-shared-sessions.md.

CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT    NOT NULL PRIMARY KEY,
	username   TEXT    NOT NULL DEFAULT '',
	role       TEXT    NOT NULL DEFAULT '',
	site       TEXT    NOT NULL DEFAULT '',
	kind       TEXT    NOT NULL DEFAULT '',
	expires_at BIGINT  NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_exp ON sessions(expires_at);
