-- 0021_ai (PostgreSQL) — bkz. sqlite/0021_ai.sql.
-- Faz 26: AI analiz sekmesi. Üç yeni tablo, her DB'de tam bir kez.
-- api_key_enc vault ile şifreli (server katmanı şifreler). Düşük hacim →
-- hypertable değil. archived=1 → gizli (hard delete yok); prune ayrı temizler.

CREATE TABLE IF NOT EXISTS ai_providers (
	id            BIGSERIAL PRIMARY KEY,
	name          TEXT   NOT NULL UNIQUE,
	kind          TEXT   NOT NULL,
	base_url      TEXT   NOT NULL DEFAULT '',
	api_key_enc   TEXT   NOT NULL DEFAULT '',
	default_model TEXT   NOT NULL DEFAULT '',
	opts_json     TEXT   NOT NULL DEFAULT '{}',
	enabled       BIGINT NOT NULL DEFAULT 1,
	created_by    TEXT   NOT NULL DEFAULT '',
	created_ts    BIGINT NOT NULL DEFAULT 0,
	updated_ts    BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ai_conversations (
	id          BIGSERIAL PRIMARY KEY,
	title       TEXT   NOT NULL DEFAULT '',
	created_by  TEXT   NOT NULL DEFAULT '',
	site        TEXT   NOT NULL DEFAULT '',
	scope_kind  TEXT   NOT NULL DEFAULT 'fleet',
	scope_ref   TEXT   NOT NULL DEFAULT '',
	provider_id BIGINT NOT NULL DEFAULT 0,
	model       TEXT   NOT NULL DEFAULT '',
	source      TEXT   NOT NULL DEFAULT 'user',
	created_ts  BIGINT NOT NULL DEFAULT 0,
	updated_ts  BIGINT NOT NULL DEFAULT 0,
	archived    BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ai_conv_recent ON ai_conversations(archived, updated_ts DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conv_scope  ON ai_conversations(scope_kind, scope_ref);

CREATE TABLE IF NOT EXISTS ai_messages (
	id              BIGSERIAL PRIMARY KEY,
	conversation_id BIGINT NOT NULL,
	role            TEXT   NOT NULL,
	content         TEXT   NOT NULL DEFAULT '',
	context_json    TEXT   NOT NULL DEFAULT '',
	tokens_in       BIGINT NOT NULL DEFAULT 0,
	tokens_out      BIGINT NOT NULL DEFAULT 0,
	error           TEXT   NOT NULL DEFAULT '',
	created_ts      BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ai_msg_conv ON ai_messages(conversation_id, created_ts);
