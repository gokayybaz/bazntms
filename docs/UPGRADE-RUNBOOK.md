# Upgrade Runbook — Güncelleme ve Sürüm Atlama (Faz 7)

## Sürüm Matrisi

| Bileşen | Güncelleme yöntemi |
|---|---|
| Hub | İkili değişimi + yeniden başlatma (schema açılışta otomatik migrasyon) |
| Agent (fleet) | İmza doğrulamalı otomatik güncelleme kanalı (stable/beta) |
| Agent (tekil) | deb/rpm/msi/pkg paket yükseltmesi veya binary değişimi |
| PostgreSQL/TimescaleDB | Küçük sürüm: yerinde; büyük sürüm: yeni instance + `pg-restore.sh` (bkz. DR-RUNBOOK) |
| NATS | Yeniden başlatma yeterli (stream diskte) |

## 1) Hub Güncellemesi

```bash
# 1. Yedek al (pg modu)
deploy/scripts/pg-backup.sh "postgres://..." /var/backups/bazntms

# 2. Yeni binary'yi yanına koy
mv bazntms-hub-yeni /usr/local/bin/bazntms-hub.new

# 3. Değiştir ve yeniden başlat
systemctl stop bazntms-hub
mv /usr/local/bin/bazntms-hub.new /usr/local/bin/bazntms-hub
systemctl start bazntms-hub

# 4. Doğrula
curl -s localhost:8080/healthz && curl -s localhost:8080/readyz
curl -s localhost:8080/api/v1/version   # sürüm yükseldi mi?
```

Notlar:
- Şema migrasyonu `store.Open` içinde otomatiktir (`CREATE TABLE IF NOT
  EXISTS`); geriye dönük sürüm çalıştırılacaksa yedekten restore gerekir.
- TimescaleDB modunda hypertable/cagg politikaları `if_not_exists` ile
  yeniden kurulur; özel işlem gerekmez.

## 2) Agent Otomatik Güncelleme Kanalı (önerilen)

Tek seferlik kurulum:

```bash
# 1. İmzalama anahtarı (imzalama makinesinde, bir kez)
bazntmsctl update keygen -out updates/keys
# public key'i agent'lara dağıtın (config: update.public_key)

# 2. Sürüm imzala (release binary'leri ile)
bazntmsctl update sign -key updates/keys/seed.key \
  -out updates/stable -version v0.2.0 \
  bazntms-agent-linux-amd64 bazntms-agent-linux-arm64 \
  bazntms-agent-windows-amd64.exe bazntms-agent-darwin-arm64

# 3. Hub'da kanalı aç
./bazntms-hub -config hub.yml   # config: updates.dir: updates/
```

Agent tarafı (`/etc/bazntms/agent.yml`):

```yaml
update:
  enabled: true
  channel: stable
  public_key: "<ed25519-hex>"
  interval_hours: 6
```

Davranış: agent her `interval_hours`'ta manifest'i sorgular → sürüm yükselmişse
indirir → **SHA-256 + ed25519** doğrular → binary'yi atomik değiştirir →
çıkıp supervisor'ın (systemd/launchd/k8s) yeniden başlatmasını bekler.

Beta kanalı: `bazntmsctl update sign -out updates/beta ...` + agent'ta
`channel: beta`. Geri alma: kanalın `manifest.json`'unu önceki sürümle
yeniden imzalayın — agent'lar kendiliğinden "inmektedir".

## 3) Agent Paket Yükseltmesi (manuel)

```bash
# deb/rpm
sudo dpkg -i bazntms-agent-amd64.deb     # veya: rpm -U bazntms-agent-amd64.rpm
# MSI (Windows): çift tık / msiexec /i bazntms-agent-amd64.msi
# macOS: installer -pkg bazntms-agent-arm64.pkg -target /
```

## 4) K8s / Container

```bash
helm upgrade bazntms deploy/helm/bazntms \
  --set image.tag=v0.2.0 \
  --reuse-values
# DaemonSet agent'ları da günceller (agent.image.tag aynıysa pod'lar yeniden kurulur)
```

## 5) Sürüm Atlama Kontrol Listesi

1. `docs/API.md` içindeki sürüm notlarındaki kırıcı değişiklikleri okuyun
2. Veri tabanı yedeği (pg) veya SQLite dosya kopyası alın
3. `vault.key` yedeği yerinde mi doğrulayın
4. Pilot bir agent/CI ortamında yeni sürümü deneyin
5. Hub'ı güncelleyin → `/healthz` + `/readyz` + `/api/v1/version`
6. Fleet'i güncelleyin (kanal veya paket) → `/api/v1/agents` sürüm dağılımı
7. `/api/v1/audit/verify` ile denetim zinciri bütünlüğünü onaylayın
