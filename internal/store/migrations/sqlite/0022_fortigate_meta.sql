-- 0022_fortigate_meta (SQLite) — bkz. postgres/0022_fortigate_meta.sql.
-- Faz 27: FortiGate REST driver sürüm-uyumu. devices tablosuna FortiGate
-- meta alanları — kolonlar tümüyle yeni → her DB'de bir kez düz ALTER;
-- taşımayan mevcut satırlarda '' (otomatik tespit).
--
--   api_profile : kullanıcının pinlediği FortiOS sürüm profili ('' / 'auto' →
--                 yanıt zarfındaki version'dan otomatik seçilir).
--   api_version : son poll/probe'da tespit edilen FortiOS sürümü ("v7.2.11").
--   api_caps    : son "Bağlantıyı Sına" / poll'un uç yetenek özeti
--                 (JSON: {"interface":"ok","sdwan":"empty","policy_mon":"denied"}).

ALTER TABLE devices ADD COLUMN api_profile TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN api_version TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN api_caps    TEXT NOT NULL DEFAULT '';
