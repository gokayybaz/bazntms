-- 0019_audit_v2 (PostgreSQL) — bkz. sqlite/0019_audit_v2.sql.
-- Faz 25-C: denetim kaydı v2. Kolonlar tümüyle yeni → her DB'de tam bir kez.
-- Mevcut satırlarda hepsi '' → append-only hash zinciri BOZULMAZ (auditHash
-- v2 segmentini yalnız bir alan doluyken ekler). Yeni satırlarda actor_type
-- + result her zaman dolu → tamper-evident.
--   actor_type  : user | legacy | token | oidc | system  (role'den ayrı — aktör sınıfı)
--   request_id  : observe middleware'inin ürettiği X-Request-Id (log korelasyonu)
--   user_agent  : aktörün istemci imzası (256'ya kırpılı)
--   result      : ok | error | denied
--   before_json : işlem öncesi durum (sır alanları "•••" maskeli)
--   after_json  : işlem sonrası durum (maskeli)
ALTER TABLE audit_events ADD COLUMN actor_type  TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN request_id  TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN user_agent  TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN result      TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN before_json TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN after_json  TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action, id DESC);
