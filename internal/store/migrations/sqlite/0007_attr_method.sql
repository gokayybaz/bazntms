-- 0007_attr_method (SQLite) — Faz 20: agent'ın aktif süreç-atıf arka ucu.
-- "ebpf" | "pcap" | "etw" | "off". Her telemetri batch'iyle tazelenir
-- (TouchAgent'ın yanında SetAgentAttrMethod). Yalnızca gösterim/teşhis —
-- hub davranışını etkilemez. Taşımayan eski agent → NULL (korunur).
ALTER TABLE agents ADD COLUMN attr_method TEXT;
