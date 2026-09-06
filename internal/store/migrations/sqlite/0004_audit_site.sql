-- 0004_audit_site (SQLite) — Faz 14 S14.B2 (çoklu-saha).
-- Denetim olayına aktörün sahası eklenir; site-admin yalnız kendi sahasının
-- olaylarını görür. site alanı hash zincirine DAHİL DEĞİLDİR (yönlendirme
-- meta verisi; zincir bütünlüğü ts|username|role|action|target|detail|ip üzerinden).
-- Kolon tümüyle yeni — her DB'de tam bir kez çalışır.

ALTER TABLE audit_events ADD COLUMN site TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_site ON audit_events(site, id);
