# bazNTMS

**bazNTMS**, merkezi hub + uç agent + ağ cihazı entegrasyonları üçlüsüne
kurulu bir ağ trafiği izleme platformudur.

- Canlı paket yakalama, süreç bazlı trafik atfı (eBPF/ETW hazır mimari)
- SNMP (LLDP/CDP/ARP topoloji keşfi), NetFlow v5, Syslog
- PostgreSQL/TimescaleDB + NATS JetStream ile 5.000 agent ölçeği
- RBAC + OIDC SSO + hash-zincirli denetim kaydı
- İmza doğrulamalı otomatik agent güncelleme kanalı
