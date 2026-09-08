# 0009 — Hub-içi zamanlanmış işler (Faz 22 S22.18)

**Tarih:** 2026-09-08 · **Durum:** kabul edildi

## Sorun

Faz 22'nin kurumsal rapor hattı (S22.19) periyodik teslim ister: her sabah
SLA raporu, her Pazartesi kapasite özeti, aylık banding. Ayrıca S22.21 günlük
SLA hedef değerlendirmesi. Hub'da genel bir zamanlayıcı yoktu — `alert.Manager`
1 sn'lik ticker + `tickN%N` gate'leriyle kendi periyodik işlerini yürütüyordu,
ama bu yalnız sabit aralıklara uygun (günün belirli saati / haftanın günü değil)
ve yeni bir "her X'te şunu yap" işi eklemek Manager'ı şişiriyor.

## Kararlar

### Harici bağımlılık yok

`robfig/cron` gibi bir kütüphane eklenmedi. `internal/scheduler` ~150 satır:
30 sn'lik ticker + `NextRun(spec, after)` yineleme hesabı. Tek binary / tek
Go modülü ilkesi (bkz. proje CLAUDE.md) korunur; cron ifadesi yerine okunur
belirteçler (`daily:08:00`, `weekly:mon:07:00`, `monthly:1:06:00`,
`interval:120`).

### Lider-kapılı (C1 deseni)

`alert.Manager` ve `devpoll` ile aynı `store.Leader` advisory-lock deseni
(`LeaderKeyScheduler`). Çoklu controller replikasında yalnız lider işleri
çalıştırır — bir rapor iki kez e-postalanmaz. `-alerts` bayrağıyla aynı
gate'te (controller replikası); ingest replikasında çalışmaz.

### Kaçırılan koşu politikası: bir kez telafi + ileri kaydır

Lider bir süre düşükse (crash, deploy, devir) `next_run_ts` geçmişte kalır.
Devir alan lider işi **bir kez** çalıştırır, sonra `next_run_ts`'i
`NextRun(spec, now)` ile **ilk gelecek** koşuma kaydırır — arada kaçan tüm
koşumlar atlanır. Gerekçe: bir raporun geç gelmesi, hiç gelmemesinden iyidir;
ama 6 saatlik kesintiden sonra 6 kopya rapor göndermek de istenmez.

### `scheduled_jobs` şeması

`0012_scheduled_jobs` (sqlite + postgres). `kind` → handler kaydı
(`scheduler.Register`), `spec` → yineleme, `payload_json` → handler
parametreleri, `next_run_ts` → sıradaki koşum, `last_status` → son sonuç
(UI'da görünür). Sadece ileri migrasyon (0003 kuralı).

## Sonuç

Yeni bir periyodik iş = `sched.Register(kind, handler)` + bir `scheduled_jobs`
satırı. İlk tüketiciler: `report` (S22.19), `sla_eval` (S22.21). Zamanlayıcı
API/protokol kapsam-dışı v1 yüzeyi değil; v1.1'de eklenen geriye uyumlu bir
özellik (ADR 0008).
