-- 0005_sessions (SQLite) — Faz 15 S15.3 (A4).
-- Paylaşımlı oturum deposu: -session-store=db ile çoklu hub replikası aynı
-- oturumları görür (panel HA). Anahtar sha256(çerez token'ı) — ham token
-- diskte tutulmaz. Bkz. docs/decisions/0004-shared-sessions.md.
-- Bellek-içi mod (varsayılan) bu tabloyu kullanmaz.

CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT    NOT NULL PRIMARY KEY,
	username   TEXT    NOT NULL DEFAULT '',
	role       TEXT    NOT NULL DEFAULT '',
	site       TEXT    NOT NULL DEFAULT '',
	kind       TEXT    NOT NULL DEFAULT '',
	expires_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_exp ON sessions(expires_at);
