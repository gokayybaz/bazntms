-- 0003_agent_machine_id (SQLite) — Faz 13 S13.6 (C3).
-- Agent state dosyası kaybında filo listesi şişmesin: hub eşleşen çevrimdışı
-- kaydı yeni token'la günceller. machine_id = hex(sha256(hostID + hostname))[:16]
-- (bkz. docs/decisions/0003-agent-machine-id.md). Kolon tümüyle yeni — her DB'de
-- tam bir kez çalışır.

ALTER TABLE agents ADD COLUMN machine_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_agents_machine ON agents(machine_id);
