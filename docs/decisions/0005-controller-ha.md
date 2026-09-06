# 0005 — Controller HA: rol ayrıştırma & liderlik (C1 / Faz 15 S15.4–S15.7)

**Tarih:** 2026-09-06 · **Durum:** kabul edildi

## Sorun

`hub-controller` tek replika: uyarı motoru + SNMP poller + NetFlow/Syslog UDP
alıcıları + panel oturumları hepsi orada. Yeniden başlarken alarm durur, poll
durur, UDP datagram'ları düşer, kullanıcılar düşer. "Cloud-hosted HA" için zayıf.

Panel oturumları [[0004-shared-sessions]] ile çözüldü. Kalan: stateful roller.

## Karar — S15.4: ayrı binary mi, rol bayrağı mı?

**Aynı binary, mevcut rol bayrakları.** bazNTMS bilinçli olarak tek-binary
(`go:embed` frontend, tek `bazntms-hub`); ayrı bir "receiver binary" bu mimariye
aykırı. Roller zaten bayrakla açılıp kapanıyor: `-capture`, `-alerts`, `-poller`,
`-flow-port`, `-syslog-port`, `-prune`. Dağıtım (compose/helm) rolleri **aynı
image'ın farklı komut satırlarıyla** kompoze eder.

## Karar — uyarı motoru & poller: DB tabanlı liderlik (S15.5–S15.6)

Bu iki rol **tam olarak bir replikada** çalışmalı (aksi halde alarm çift
ateşlenir, cihazlar iki kez pollanır). Kubernetes `Lease` / harici koordinatör
(etcd, Consul) yerine **PostgreSQL session-scoped advisory lock** —
bağımlılık sıfır, zaten Postgres var:

- `store.Leader(key, name)` — arka planda `pg_try_advisory_lock(key)` dener;
  alınırsa kilidi TUTAN özel bir `*sql.Conn` saklar. `IsLeader()` rol
  döngüsünden her turda kontrol edilir.
- Lider süreç ölürse bağlantı düşer → PG kilidi **otomatik** bırakır → başka
  replika ~10 sn içinde devralır (`Run` ticker'ı).
- SQLite (dev / tek replika): `Leader` hemen ve kalıcı lider — no-op.
- `alert.Manager.SetLeaderCheck(fn)` / `devpoll.Poller.SetLeaderCheck(fn)` —
  `nil` → daima lider (geriye uyum). Lider değilken motor "sıcak" kalır ama
  hiçbir kural değerlendirmez; bant genişliği sayaçları sıfırlanır (devralınca
  geçmiş kalıntısıyla tetiklenmesin).
- Anahtarlar: `LeaderKeyAlerts = 8823101`, `LeaderKeyPoller = 8823102`.
- **Failover'da küçük dup riski:** tek-atışlık alarmların "görüldü" durumu DB'de
  (`alert_seen`) → paylaşımlı. Ama in-memory `lastFire` cooldown'ı replika-başına
  → devralan yeni lider yakın zamanda ateşlenmiş bir alarmı bir kez daha
  ateşleyebilir. Kabul edilebilir.

## Karar — UDP alıcıları: her replikada, kuyruğa yaz (S15.7)

NetFlow/Syslog alıcıları (`-flow-port`, `-syslog-port`) **her replikada**
çalışabilir çünkü:

1. Bir UDP exporter tek adrese gönderir; k8s Service / LB datagram'ı **tek**
   pod'a yönlendirir → her datagram bir kez işlenir.
2. Alıcılar yalnızca JetStream'e publish eder; `store-writer` durable consumer
   mesajı hangi replika publish etti fark etmeksizin **bir kez** işler.

İdempotentlik: aynı datagram iki replikaya ulaşırsa (patolojik) → `flows` çift
satır (çift sayım), `syslog_events` + `compliance_logs` çift kayıt. Zincir
bozulmaz, yalnızca gürültü. Service LB ile bu durum oluşmaz; ek dedup kapsam
dışı (YAGNI).

Kuyruk kapalıysa (`-nats` boş) alıcılar doğrudan store'a yazar → o modda tek
replika gerekir (zaten tek-node kurulum).

Compose'da "Service LB" rolünü nginx `lb` üstlenir: `deploy/nginx/lb.conf`
`stream{}` bloğunda `listen 2055 udp` / `listen 5514 udp` sunucuları her
datagram'ı `hub-controller` havuzundan **tek** replikaya proxy'ler (bkz.
`deploy/docker-compose.scale.yml` `lb.ports` loopback eşlemeleri; Docker Desktop
for Mac için `deploy/scripts/mac-udp-relay.sh`).

## Sonuç

`-alerts` / `-poller` açık her replika lider yarışına girer; kazanan çalışır,
diğerleri bekler. `-flow-port` / `-syslog-port` açık her replika datagram alır
ve kuyruğa yazar. `-session-store=db` ile panel her replikada. → controller
artık N replika, biri ölünce kesinti yok.

Kalan (S15.9): ölçek smoke testini 2× controller ile güncelle.
