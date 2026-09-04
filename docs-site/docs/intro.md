---
slug: /
---

# bazNTMS

**bazNTMS**, merkezi hub + uç agent + ağ cihazı entegrasyonları üçlüsüne
kurulu bir ağ trafiği izleme platformudur.

- Canlı paket yakalama, süreç bazlı trafik atfı; **L7 uygulama görünürlüğü**
  (TLS SNI + HTTP Host) ve **DNS görünürlüğü**, ikisi de süreç bazlı
- SNMPv3 (LLDP/CDP/ARP topoloji keşfi) + **NetFlow v5/v9, IPFIX, sFlow v5** +
  Syslog + FortiGate REST API
- Agent↔hub **karşılıklı TLS (mTLS)**: hub CA'sı, kısa ömürlü istemci
  sertifikaları, otomatik yenileme
- PostgreSQL/TimescaleDB + NATS JetStream ile 5.000 agent ölçeği; site-scope RBAC
- **Coğrafi trafik haritası** (GeoIP), **SIEM/ITSM connector** (CEF/LEEF/JSON
  → syslog veya HTTP), **tehdit istihbaratı (IOC)** eşleştirmesi
- Elle bakımlı **OpenAPI 3.1** şeması + gömülü tarayıcı (`/api/docs`)
- RBAC + OIDC SSO + hash-zincirli denetim kaydı
- İmza doğrulamalı otomatik agent güncelleme kanalı
- 5651 log imzalama + ISO 27001 ISMS yönetişim modülü

Süreç bazlı derin telemetri bugün platforma özgü soket→PID eşlemesiyle
çalışır (Linux `/proc`, macOS `lsof`, Windows `netstat`); eBPF (Linux) ve
ETW (Windows) sağlayıcıları aynı arayüzün arkasına eklenecek ileri faz.
