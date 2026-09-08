-- 0017_topology_confidence (PostgreSQL) — bkz. sqlite/0017_topology_confidence.sql.
-- Faz 23-D: topology_links + confidence (discovered | inferred | manual).
-- Mevcut satırlar → 'discovered' (DEFAULT).
ALTER TABLE topology_links ADD COLUMN confidence TEXT NOT NULL DEFAULT 'discovered';
