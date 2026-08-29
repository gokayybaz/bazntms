---
sidebar_position: 2
---

# Agent Dağıtımı

## Paketler

| Platform | Paket | Kurulum |
|---|---|---|
| Linux deb | `bazntms-agent-<arch>.deb` | `dpkg -i ...` → `/etc/bazntms/agent.yml` doldur → `systemctl start bazntms-agent` |
| Linux rpm | `bazntms-agent-<arch>.rpm` | `rpm -U ...` (aynı adımlar) |
| Windows | `bazntms-agent-amd64.msi` | MSI servis olarak kurar; config: `C:\ProgramData\bazntms\agent.yml` |
| macOS | `bazntms-agent-<arch>.pkg` | `installer -pkg ...` → LaunchDaemon |
| Docker/k8s | `ghcr.io/gokayybaz/bazntms-agent` | Helm `agent.enabled=true` (DaemonSet) |

## İlk kayıt (enrollment)

```yaml
hub:
  url: https://hub.example.com
  token: <enroll-token>   # hub loglarından / UI'dan
```

## Otomatik güncelleme

```yaml
update:
  enabled: true
  channel: stable
  public_key: "<ed25519-hex>"
  interval_hours: 6
```

Ayrıntı: `docs/UPGRADE-RUNBOOK.md`.
