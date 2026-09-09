-- 0023_attr_diag (PostgreSQL) — bkz. sqlite/0023_attr_diag.sql.
-- Kolonlar tümüyle yeni → her DB'de tam bir kez. Migrasyonlar setupTimescale'den
-- önce koşar → düz tablo.
ALTER TABLE agents ADD COLUMN attr_iface TEXT;
ALTER TABLE agents ADD COLUMN attr_note TEXT;
