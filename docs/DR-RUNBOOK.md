# DR Runbook — Felaket Kurtarma ve Yüksek Erişilebilirlik

## Genel Bakış

| Bileşen | Durum | Kurtarma yöntemi |
|---|---|---|
| Hub (ingest/API) | Stateless, N replika | Yeni pod/binary başlat; veri kaybı yok |
| PostgreSQL/TimescaleDB | Stateful | `pg-backup.sh` yedeğinden `pg-restore.sh` |
| NATS JetStream | Kuyruk (MaxAge 24s) | Yeniden başlatma yeterli; kuyruktaki kayıplar agent offline kuyruğundan telafi olur |
| Agent'lar | Uçta stateful (state.json + disk kuyruğu) | Otomatik: çoklu-hub failover + offline kuyruk |
| vault.key | Cihaz SNMP secret'larının şifre anahtarı | Yedekten geri koy — kaybedilirse secret'lar yeniden girilir |

## 1) Agent çoklu-hub failover

Agent, hub adreslerini sırayla dener ve sağlıklıya yapışır:

```yaml
# bazntms-agent.yml
hub:
  urls:
    - https://hub-a.kurum.local
    - https://hub-b.kurum.local
```

veya `-hub-url "https://hub-a,https://hub-b"`. Bir hub düşerse agent diğerine
devam eder; bağlantı yokken topladığı batch'ler offline disk kuyruğunda
(`state.json.queue.jsonl`, 100 batch'a kadar) bekler.

## 2) PostgreSQL yedekleme

```bash
# günlük yedek (cron): custom format + schema dump, 14 gün saklama
deploy/scripts/pg-backup.sh "postgres://bazntms:***@db:5432/bazntms?sslmode=disable" /var/backups/bazntms

# geri yükleme (ezici işlem — önce durdurun):
systemctl stop bazntms-hub   # veya kubectl scale deploy bazntms --replicas=0
deploy/scripts/pg-restore.sh /var/backups/bazntms/bazntms-YYYYMMDD-HHMMSS.dump "postgres://..."

# hub yeniden başlat → /readyz "ready" dönmeli
```

TimescaleDB notları:
- Açılışta hypertable/cagg/retention politikaları `if_not_exists` ile yeniden
  kurulur; restore sonrası özel işlem gerekmez.
- Continuous aggregate'lar `WITH NO DATA` ile restore olabilir; ilk policy
  penceresi (5 dk) dolduğunda otomatik dolar.

## 3) vault.key (kimlik kasası)

`vault.key`, cihaz SNMP community/v3 secret'larını AES-GCM ile şifreler.
**Bu dosyanın yedeği olmadan geri yükleme, secret'ların yeniden girilmesini
gerektirir.** Yedekleme:

```bash
cp vault.key /secure/offline/location/   # ve şifreli ikinci kopya (ör. passmanager)
chmod 600 vault.key
```

## 4) Hub failover senaryosu

1. Bir ingest replikası ölür → LB sağlıksız pod'u çıkarır (k8s: readiness
   probe `/readyz` otomatik yönetir).
2. Aktif uyarı/poller rolü tek replikada tutulur (`-alerts/-poller`
   bayrakları). Yönetim replikası ölürse: rodları boş bir replikaya taşı.
3. Tüm replikalar ölürse: agent'lar offline kuyruğa yazar; hub geri geldiğinde
   kuyruklar boşalır (≤100 batch × batch aralığı kayıp toleransı).

## 5) Denetim zinciri bütünlüğü (DR sonrası kontrol)

```bash
curl -H "Authorization: Bearer $TOKEN" https://hub/api/v1/audit/verify
# {"ok":true,"broken_at":0,"checked":N}  → zincir sağlam
```

`ok:false` + `broken_at:X` → X ID'li kayıttan itibaren veri tabanı
değiştirilmiş demektir; en son sağlam yedeği araştırın.

## 6) Sıralı DR kontrol listesi

1. Enfrastrüktürü değil veriyi önce kurtar: PostgreSQL restore
2. `vault.key` geri koy (cihaz secret'ları için)
3. Hub'ı yedek DSN ile başlat → `/healthz`, `/readyz`
4. NATS'i başlat (stream otomatik oluşur)
5. Bir pilot agent'ın telemetrisini doğrula (`/api/v1/agents` online)
6. `/api/v1/audit/verify` ile denetim zinciri bütünlüğünü onayla
7. Kalan replikaları ölçekle
