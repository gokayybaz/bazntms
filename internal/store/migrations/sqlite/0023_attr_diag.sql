-- 0023_attr_diag (SQLite) — süreç-atıf teşhisi: yakalama arayüzü + kapalı neden.
-- attr_iface: pcap arka ucunun dinlediği arayüz (yalnız method=pcap). "motor
--   çalışıyor ama panel boş → yanlış/sanal arayüz mü?" sorusunu UI'da yanıtlar.
-- attr_note: motor KAPALI/başlatılamadıysa insan-okur neden
--   ("collect.method=off" | "hub -agent-pcap=false" | pcap hata ipucu).
-- Her telemetri batch'iyle tazelenir (SetAgentAttrInfo). Yalnızca gösterim/
-- teşhis — hub davranışını etkilemez. Taşımayan eski agent → NULL (korunur).
ALTER TABLE agents ADD COLUMN attr_iface TEXT;
ALTER TABLE agents ADD COLUMN attr_note TEXT;
