-- 0008_perf_indexes (SQLite) — bkz. postgres/0008_perf_indexes.sql.
-- Faz 21 S21.11: `/api/v1/flows` TopFlows üst-N taraması (ORDER BY octets DESC).
CREATE INDEX IF NOT EXISTS idx_flows_octets ON flows (octets DESC);
