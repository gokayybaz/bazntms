# Upgrade Runbook — Güncelleme ve Sürüm Atlama

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
curl -s localhost:8080/readyz                    # {"status":"ready"}
curl -s localhost:8080/healthz | jq .version     # sürüm yükseldi mi?
```

Notlar:
- Şema migrasyonu `store.Open` içinde otomatiktir: gömülü sıralı migrasyon
  dosyaları (`internal/store/migrations/{sqlite,postgres}/NNNN_*.sql`),
  uygulanmış sürümler `schema_migrations` tablosunda izlenir. Faz 13 öncesi
  bir veritabanı ilk açılışta `0001_init` baseline olarak işaretlenir (yeniden
  çalıştırılmaz). Geriye dönük sürüm çalıştırılacaksa yedekten restore gerekir.
- Çoklu hub replikası aynı veritabanına karşı aynı anda başlarsa migrasyon
  Postgres advisory lock ile sıraya alınır.
- TimescaleDB modunda hypertable/cagg politikaları `if_not_exists` ile
  yeniden kurulur; özel işlem gerekmez.
- **v1.0'a yükseltme — continuous aggregate geri-doldurması (S21.11).** Bu
  sürüm yüksek hacimli tablolar için yeni cagg'ler ekler
  (`flows_dst_1h`, `flows_src_1h`, `agent_iface_1h`, `process_traffic_1h`).
  Yeni cagg'ler boş oluşturulur; yenileme politikası yalnızca son ~2 günü
  geriye doldurur. **Mevcut geçmişi (2 gün → retention) raporlarda görmek
  için** hub'lar ayağa kalktıktan sonra bir kez (tercihen düşük trafik
  penceresinde) çalıştırın:

  ```sql
  CALL refresh_continuous_aggregate('flows_dst_1h', NULL, NULL);
  CALL refresh_continuous_aggregate('flows_src_1h', NULL, NULL);
  CALL refresh_continuous_aggregate('agent_iface_1h', NULL, NULL);
  CALL refresh_continuous_aggregate('process_traffic_1h', NULL, NULL);
  ```

  Bu, retention penceresi kadar ham veriyi bir kez tarar — büyük filolarda
  saatler sürebilir ve I/O yoğundur. Yeni kurulumlarda gerekmez (veri
  biriktikçe politika doldurur). Geri-doldurma tamamlanana dek `/api/report`
  ve `/api/v1/geo` eski dönemi eksik gösterir ama yavaşlamaz.
- **v1.3.0'a yükseltme — derin toplama & L7 varsayılan açık.** Hub
  `-agent-pcap` politikası ve agent tarafında süreç/DNS/L7 atıf motoru artık
  **varsayılan çalışır**. Yükseltmeden sonra süreç trafiği / DNS / L7
  panelleri **kendiliğinden dolmaya başlar** (agent'lar root/SYSTEM çalışır)
  ve süreç-atıflı tablolara yazım artar — kapasite planlaması için
  `docs/CAPACITY.md`. İstemiyorsanız: hub `-agent-pcap=false` (veya `hub.yaml`
  / Helm `agent_pcap: false`), ya da agent başına `agent.yml` →
  `collect.method: off`. Eski `collect.pcap: false` **artık yok sayılır**;
  kapatmak için `method: off` gerekir.
- **v1.3.0 — Windows agent'ta L7 için Npcap.** MSI yükseltmesi Npcap yoksa
  onu sessizce indirip kurar (SHA-256 + Authenticode doğrulamalı). İnternet
  erişimi olmayan Windows ana makinelerinde önce Npcap'i elle kurun; MSI onu
  tespit edip atlar, aksi halde agent ETW'ye düşer (L7 hariç her şey akar).

## 2) Agent Otomatik Güncelleme (varsayılan — genelde hiçbir şey yapmanız gerekmez)

**Varsayılan davranış:** hub, `-update-github-repo` (varsayılan
`gokayybaz/bazntms`) deposunun **en son GitHub release'ini** her 30 dk'da bir
yoklar; yeni sürümde `bazntms-agent-*` binary'lerini `-updates-dir`'e
(`updates/`) indirir, SHA-256 hesaplar, `manifest.json` yazar. Agent'larda
otomatik güncelleme de **varsayılan açıktır**: her 6 saatte bir hub'ın
kanalını sorgular → sürüm yükselmişse indirir → **SHA-256** doğrular →
binary'yi atomik değiştirir → çıkar; supervisor yeniden başlatır (systemd
`Restart=always` / launchd `KeepAlive` / docker `--restart` / Windows SCM
failure-action — MSI kurar).

Yani **GitHub'da yeni bir release yayınlamak yeterli** — fleet ~30 dk + agent
yoklama aralığı içinde kendini günceller. `/api/v1/agents` sürüm dağılımından
izleyin.

Kapatmak:

| Kapsam | Yöntem |
|---|---|
| Tek agent | `-update-disabled` bayrağı **veya** `agent.yml` → `update: {disabled: true}` |
| Hub (GitHub senkronu) | `-update-github-repo=''` — kanal yalnızca elle hazırlanmış `-updates-dir` içeriğini sunar |
| Yoklama sıklığı | hub `-update-github-interval=1h`, agent `update.interval_hours` / `-update-interval` |

### 2a) İmzalı / hava-boşluklu (air-gapped) kanal — `bazntmsctl update sign`

Ek tedarik-zinciri imzası isteyen veya hub'ın GitHub'a erişemediği kurulumlar:

```bash
# 1. İmzalama anahtarı (imzalama makinesinde, bir kez)
bazntmsctl update keygen -out updates/keys
# public key'i agent'lara dağıtın: agent.yml → update.public_key / -update-key

# 2. Sürüm imzala (release binary'leri ile)
bazntmsctl update sign -key updates/keys/seed.key \
  -out updates/stable -version v0.3.1 \
  bazntms-agent-linux-amd64 bazntms-agent-linux-arm64 \
  bazntms-agent-windows-amd64.exe bazntms-agent-darwin-arm64

# 3. Hub: GitHub senkronunu kapat, statik dizini sun
./bazntms-hub -config hub.yml -update-github-repo=''   # config: updates.dir: updates/
```

`update.public_key` dolu ise agent indirdiği her binary'de ed25519 imzasını da
doğrular. Beta kanalı: `-out updates/beta` + agent'ta `channel: beta`. Geri
alma: kanalın `manifest.json`'unu önceki sürümle yeniden yazın/imzalayın —
agent `CompareVersions <= 0` görür ve düşmez (yalnızca yeni sürüme çıkar);
gerçek downgrade için agent'ı elle eski binary ile değiştirin.

## 3) Agent Paket Yükseltmesi (manuel)

```bash
# deb/rpm
sudo dpkg -i bazntms-agent-amd64.deb     # veya: rpm -U bazntms-agent-amd64.rpm
# MSI (Windows): çift tık / msiexec /i bazntms-agent-amd64.msi
# macOS: installer -pkg bazntms-agent-arm64.pkg -target /
```

## 4) K8s / Container

```bash
# Chart appVersion imaj etiketini de taşır — yeni chart sürümü = yeni imajlar
helm upgrade bazntms oci://ghcr.io/gokayybaz/charts/bazntms --version 0.3.3 \
  --reuse-values
# DaemonSet agent'ları da günceller (agent.enabled=true ise pod'lar yeniden kurulur)
```

## 5) Sürüm Atlama Kontrol Listesi

1. `docs/API.md` içindeki sürüm notlarındaki kırıcı değişiklikleri okuyun
2. Veri tabanı yedeği (pg) veya SQLite dosya kopyası alın
3. `vault.key` yedeği yerinde mi doğrulayın
4. Pilot bir agent/CI ortamında yeni sürümü deneyin
5. Hub'ı güncelleyin → `/readyz` "ready" + `/healthz` `.version` yeni sürüm
6. Fleet'i güncelleyin (kanal veya paket) → `/api/v1/agents` sürüm dağılımı
7. `/api/v1/audit/verify` ile denetim zinciri bütünlüğünü onaylayın
