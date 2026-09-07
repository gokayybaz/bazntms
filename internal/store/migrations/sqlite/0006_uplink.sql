-- 0006_uplink (SQLite) — Canlı Akış agent gruplama.
-- Agent "şu switch/AP cihazının arkasında" bilgisi: nullable uplink_device_id.
-- Şema agent'ları bu alana göre gruplar, her grup için bir switch/AP düğümü
-- çizer. Yönetici Canlı Akış sayfasında satır içi atar. devices.uplink_device_id
-- switch → router zinciri için (şimdilik yalnız API'de). Kolonlar tümüyle yeni —
-- her DB'de tam bir kez. FK yok; silinen cihazın referansları DeleteDevice
-- içinde NULL'lanır.

ALTER TABLE agents  ADD COLUMN uplink_device_id INTEGER;
ALTER TABLE devices ADD COLUMN uplink_device_id INTEGER;
CREATE INDEX IF NOT EXISTS idx_agents_uplink ON agents(uplink_device_id);
