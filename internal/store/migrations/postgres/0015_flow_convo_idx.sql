-- 0015_flow_convo_idx (PostgreSQL) — bkz. sqlite/0015_flow_convo_idx.sql.
-- Faz 23-B: NetFlow konuşma toplama. FlowConversations bir zaman penceresinde
-- `flows`'u 5'li ya da uç-çifti bazında toplar. Migrasyonlar setupTimescale'den
-- ÖNCE koşar (store.go) — `flows` o an düz tablo; hypertable dönüşümü
-- migrate_data => TRUE ile indeksleri chunk'lara taşır (0008 idx_flows_octets
-- ile aynı yol).
--   idx_flows_convo : pencere taraması + GROUP BY için bileşik anahtar
--   idx_flows_pair  : drill-down (belirli src↔dst'nin ham akışları)
CREATE INDEX IF NOT EXISTS idx_flows_convo ON flows (ts, src, dst, proto);
CREATE INDEX IF NOT EXISTS idx_flows_pair  ON flows (src, dst, ts);
