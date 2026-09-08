-- 0017_topology_confidence (SQLite) — bkz. postgres/0017_topology_confidence.sql.
-- Faz 23-D: topoloji kenarına güven düzeyi. discovered (LLDP/CDP/ARP/subnet —
-- SNMP/agent keşfi) | inferred (trafik çıkarımı — açıkça işaretli) | manual
-- (operatör). Mevcut satırların hepsi keşiften geldi → 'discovered'.
ALTER TABLE topology_links ADD COLUMN confidence TEXT NOT NULL DEFAULT 'discovered';
