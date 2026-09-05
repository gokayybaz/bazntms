-- 0003_agent_machine_id (PostgreSQL) — Faz 13 S13.6 (C3).
-- Bkz. docs/decisions/0003-agent-machine-id.md. Kolon tümüyle yeni — her DB'de
-- tam bir kez çalışır.

ALTER TABLE agents ADD COLUMN machine_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_agents_machine ON agents(machine_id);
