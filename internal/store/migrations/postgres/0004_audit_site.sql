-- 0004_audit_site (PostgreSQL) — Faz 14 S14.B2 (çoklu-saha).
-- Bkz. sqlite/0004_audit_site.sql. Kolon tümüyle yeni — her DB'de tam bir kez.

ALTER TABLE audit_events ADD COLUMN site TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_site ON audit_events(site, id);
