-- 0008_perf_indexes (PostgreSQL) — bkz. sqlite/0008_perf_indexes.sql.
-- Faz 21 S21.11: ölçekte yavaş kalan panel sorguları.
--
-- `/api/v1/flows` (TopFlows): `WHERE ts >= ? ORDER BY octets DESC LIMIT N`.
-- 50k flow/sn'de 15 dk penceresi milyonlarca satır → tam tarama + sıralama
-- (p95 ~2.4 sn). octets üzerinde azalan bir indeks üst-N taramasını indekse
-- alır. TimescaleDB hypertable'da chunk başına otomatik yayılır.
CREATE INDEX IF NOT EXISTS idx_flows_octets ON flows (octets DESC);
