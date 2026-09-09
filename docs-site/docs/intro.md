---
slug: /
---

# bazNTMS

**bazNTMS**, merkezi hub + uç agent + ağ cihazı entegrasyonları üçlüsüne
kurulu bir ağ trafiği izleme platformudur. Tek makinede başlayan bir kurulum
5.000 agent ölçeğine aynı binary ile taşınır.

- Canlı paket ölçümü, süreç bazlı trafik atfı; **L7 uygulama görünürlüğü**
  (TLS SNI + HTTP Host) ve **DNS görünürlüğü**, ikisi de süreç bazlı
- SNMPv3 (LLDP/CDP/ARP topoloji keşfi) + **NetFlow v5/v9, IPFIX, sFlow v5** +
  Syslog + FortiGate REST API
- Agent↔hub **karşılıklı TLS (mTLS)**: hub CA'sı, kısa ömürlü istemci
  sertifikaları, otomatik yenileme
- PostgreSQL/TimescaleDB + NATS JetStream ile **doğrulanmış 5.000 agent**
  ölçeği; site-scope RBAC
- **Coğrafi trafik haritası** (GeoIP), **SIEM/ITSM connector** (CEF/LEEF/JSON
  → syslog veya HTTP), **tehdit istihbaratı (IOC)** eşleştirmesi
- **Anomali tespiti** (mevsimsel + EWMA baseline), **olay motoru**,
  **sağlık skoru**; **zamanlı / PDF / SLA raporları**
- **AI analiz**: çoklu sağlayıcı (OpenAI-uyumlu + Anthropic), `/ai` sohbet
  sekmesi, gecelik analiz ve olay-tetikli triyaj — opt-in (`-ai`) + egress kilidi
- Elle bakımlı **OpenAPI 3.1** şeması + gömülü tarayıcı (`/api/docs`)
- RBAC + OIDC SSO + hash-zincirli denetim kaydı
- İmza doğrulamalı otomatik agent güncelleme kanalı
- 5651 log imzalama + ISO 27001 ISMS yönetişim modülü

## Süreç atfı sağlayıcıları

Süreç bazlı derin telemetri (süreç trafiği + DNS + L7/SNI) **v1.3.0'dan beri
tüm kurulum yollarında varsayılan açık**. Atıf motoru `AttrSource` arayüzünün
arkasında platforma göre seçilir:

| Platform | Sağlayıcı | Not |
|---|---|---|
| Linux | **eBPF** (fentry CO-RE) | Kernel ≥ 5.8 + BTF gerekir; L7 (SNI/Host) için dar filtreli yardımcı pcap açılır |
| Windows | **ETW** (elle yazılmış sağlayıcı) | MSI kurulumu Npcap'i sessizce kurar → `pcap` yöntemiyle gelir (L7 dahil); Npcap yoksa ETW'ye düşer (L7 hariç akar) |
| macOS | **pcap** | L7 dahil; `sudo` (BPF erişimi) gerekir |

`collect.method: auto` (varsayılan) uygun sağlayıcıyı seçip gerekirse pcap /
procfs'e düşer. Kapatmanın tek yolu `collect.method: off`.
