-- 0022_fortigate_meta (PostgreSQL) — bkz. sqlite/0022_fortigate_meta.sql.
-- Faz 27: FortiGate REST driver sürüm-uyumu. devices tablosuna FortiGate
-- meta alanları — kolonlar tümüyle yeni → her DB'de tam bir kez.
-- Migrasyonlar setupTimescale'den önce koşar → düz tablo.
--
--   api_profile : kullanıcının pinlediği FortiOS sürüm profili ('' / 'auto' →
--                 otomatik tespit).
--   api_version : son poll/probe'da tespit edilen FortiOS sürümü ("v7.2.11").
--   api_caps    : son "Bağlantıyı Sına" / poll'un uç yetenek özeti (JSON).

ALTER TABLE devices ADD COLUMN api_profile TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN api_version TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN api_caps    TEXT NOT NULL DEFAULT '';
