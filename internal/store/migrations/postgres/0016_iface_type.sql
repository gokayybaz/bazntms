-- 0016_iface_type (PostgreSQL) — bkz. sqlite/0016_iface_type.sql.
-- Faz 23-C: device_iface_samples + if_type (IANAifType) + high_speed
-- (ifHighSpeed, Mbps). Kolonlar tümüyle yeni → her DB'de tam bir kez.
-- Migrasyonlar setupTimescale'den önce koşar → düz tablo.
ALTER TABLE device_iface_samples ADD COLUMN if_type    BIGINT NOT NULL DEFAULT 0;
ALTER TABLE device_iface_samples ADD COLUMN high_speed BIGINT NOT NULL DEFAULT 0;
