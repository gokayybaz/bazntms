-- 0007_attr_method (PostgreSQL) — bkz. sqlite/0007_attr_method.sql.
-- Agent'ın aktif süreç-atıf arka ucu: "ebpf" | "pcap" | "etw" | "off".
ALTER TABLE agents ADD COLUMN attr_method TEXT;
