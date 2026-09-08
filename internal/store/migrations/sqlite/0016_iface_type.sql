-- 0016_iface_type (SQLite) — bkz. postgres/0016_iface_type.sql.
-- Faz 23-C: arayüz kapasite farkındalığı. device_iface_samples'e IANAifType
-- (arayüz tipi sınıflandırma) + ifHighSpeed (>4 Gbps arayüzlerde ifSpeed
-- uint32'de taşar). Kolonlar tümüyle yeni → her DB'de bir kez düz ALTER;
-- taşımayan eski örneklerde 0 (bilinmeyen).
ALTER TABLE device_iface_samples ADD COLUMN if_type    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE device_iface_samples ADD COLUMN high_speed INTEGER NOT NULL DEFAULT 0;
