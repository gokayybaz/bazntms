---
sidebar_position: 2
---

# Agent Dağıtımı

## Paketler

| Platform | Paket | Kurulum |
|---|---|---|
| Linux deb | `bazntms-agent-<arch>.deb` | `dpkg -i ...` → `/etc/bazntms/agent.yml` doldur → `systemctl start bazntms-agent` |
| Linux rpm | `bazntms-agent-<arch>.rpm` | `rpm -U ...` (aynı adımlar) |
| Windows | `bazntms-agent-amd64.msi` | MSI servis olarak kurar + Npcap'i sessizce kurar; config: `C:\ProgramData\bazntms\agent.yml` |
| macOS | `bazntms-agent-<arch>.pkg` | `installer -pkg ...` → LaunchDaemon |
| Docker/k8s | `ghcr.io/gokayybaz/bazntms-agent` | Helm `agent.enabled=true` (DaemonSet) |

## İlk kayıt (enrollment)

```yaml
hub:
  url: https://hub.example.com
  token: <enroll-token>   # hub loglarından / UI'dan (bootstrap-only)
```

## Süreç atfı & derin toplama

**v1.3.0'dan beri varsayılan açık** — `collect.pcap: true` veya `-pcap`
gerekmez (yok sayılır ama kabul edilir).

```yaml
collect:
  method: auto   # varsayılan: eBPF (Linux) / ETW (Windows) / pcap (macOS),
                 # gerekirse pcap → procfs'e düşer
  # method: off  # derin toplamayı tamamen kapatmanın tek yolu
```

- **Linux**: eBPF için kernel ≥ 5.8 + BTF gerekir; yoksa yardımcı pcap →
  procfs'e düşülür. `CAP_NET_RAW` + `CAP_NET_ADMIN` (paket kurulumları verir).
- **Windows**: MSI Npcap'i sessizce kurar → `pcap` yöntemiyle gelir (L7 dahil).
  Npcap yoksa ETW'ye düşer — süreç trafiği + DNS akar, L7/SNI akmaz.
- **macOS**: `sudo` (BPF erişimi).

## Otomatik güncelleme

`update.enabled` varsayılan **`true`** — devre dışı bırakmak için
`-update-disabled` bayrağı veya:

```yaml
update:
  disabled: true          # opt-out
  # veya ayarları özelleştir:
  channel: stable
  public_key: "<ed25519-hex>"   # imza doğrulaması (opt-in; boşsa imzasız kanal)
  interval_hours: 6
```

Hub `GitHubSyncer` en son release'i çekip `updates/stable` manifestine
yazar; agent SHA-256 (+ opsiyonel ed25519) doğrulamalı atomik binary
değişimi yapar. Ayrıntı:
[Güncelleme (Upgrade) runbook'u](/docs/reference/upgrading).
