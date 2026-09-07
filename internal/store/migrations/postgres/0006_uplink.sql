-- 0006_uplink (PostgreSQL) — bkz. sqlite/0006_uplink.sql.
-- Canlı Akış agent gruplama: agent "şu switch/AP cihazının arkasında".
-- Kolonlar tümüyle yeni — her DB'de tam bir kez. FK yok; silinen cihazın
-- referansları DeleteDevice içinde NULL'lanır.

ALTER TABLE agents  ADD COLUMN uplink_device_id BIGINT;
ALTER TABLE devices ADD COLUMN uplink_device_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_agents_uplink ON agents(uplink_device_id);
