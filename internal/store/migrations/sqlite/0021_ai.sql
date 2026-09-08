-- 0021_ai (SQLite) — bkz. postgres/0021_ai.sql.
-- Faz 26: AI analiz sekmesi. Monolit döneminde (d92d0fb) internal/ai vardı,
-- 9d22e7a'da silindi. Geri getirilip çoklu-sağlayıcı + kalıcı sohbet modeline
-- yükseltiliyor. Üç yeni tablo, her DB'de bir kez.
--
--   ai_providers      : yapılandırılmış AI sağlayıcıları. api_key_enc vault ile
--                       şifreli (server katmanı şifreler — devices.go deseni).
--                       kind: openai|anthropic|ollama|lmstudio|openai-compat.
--   ai_conversations  : sohbet oturumu. scope_kind/scope_ref sayfa-farkında
--                       bağlam (fleet|agent|incident|anomaly|device).
--                       source: user|nightly|triage. archived=1 → gizli (hard
--                       delete yok — proje kuralı; prune ayrı temizler).
--   ai_messages       : oturum mesajları. context_json bu mesaja iliştirilen
--                       anlık görüntü. conversation silinince Go tarafı cascade
--                       (DeleteAgent deseni — FK yok).

CREATE TABLE IF NOT EXISTS ai_providers (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT    NOT NULL UNIQUE,
	kind          TEXT    NOT NULL,
	base_url      TEXT    NOT NULL DEFAULT '',
	api_key_enc   TEXT    NOT NULL DEFAULT '',
	default_model TEXT    NOT NULL DEFAULT '',
	opts_json     TEXT    NOT NULL DEFAULT '{}',
	enabled       INTEGER NOT NULL DEFAULT 1,
	created_by    TEXT    NOT NULL DEFAULT '',
	created_ts    INTEGER NOT NULL DEFAULT 0,
	updated_ts    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ai_conversations (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	title       TEXT    NOT NULL DEFAULT '',
	created_by  TEXT    NOT NULL DEFAULT '',
	site        TEXT    NOT NULL DEFAULT '',
	scope_kind  TEXT    NOT NULL DEFAULT 'fleet',
	scope_ref   TEXT    NOT NULL DEFAULT '',
	provider_id INTEGER NOT NULL DEFAULT 0,
	model       TEXT    NOT NULL DEFAULT '',
	source      TEXT    NOT NULL DEFAULT 'user',
	created_ts  INTEGER NOT NULL DEFAULT 0,
	updated_ts  INTEGER NOT NULL DEFAULT 0,
	archived    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ai_conv_recent ON ai_conversations(archived, updated_ts DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conv_scope  ON ai_conversations(scope_kind, scope_ref);

CREATE TABLE IF NOT EXISTS ai_messages (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	conversation_id INTEGER NOT NULL,
	role            TEXT    NOT NULL,
	content         TEXT    NOT NULL DEFAULT '',
	context_json    TEXT    NOT NULL DEFAULT '',
	tokens_in       INTEGER NOT NULL DEFAULT 0,
	tokens_out      INTEGER NOT NULL DEFAULT 0,
	error           TEXT    NOT NULL DEFAULT '',
	created_ts      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ai_msg_conv ON ai_messages(conversation_id, created_ts);
