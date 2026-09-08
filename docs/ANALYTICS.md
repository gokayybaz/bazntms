# Anomali Motoru

İstatistiksel, AI'sız erken uyarı (Faz 6.2 · Faz 22 v2). Kaynak:
`internal/alert/anomaly.go` + `internal/store/{fleet_baseline,anomaly}.go`.

## Model

Her **metrik × boyut** için bir **mevsimsel baseline** tutulur ve o anki
pencere değeriyle z-skoru olarak karşılaştırılır:

```
z = (şu_anki_pencere − baseline.mean) / baseline.std
```

`|z| ≥ sensitivity` (varsayılan 3.0) ve `|şu_anki − mean| ≥ min_abs_delta`
(sessiz-saat gürültü tabanı) ise **`anomaly`** uyarısı üretilir. `|z| ≥ crit_z`
(varsayılan 5) ise önem **crit**, aksi **warn**.

### Boyutlar (`dim`)

| dim | anlam | kaynak |
|---|---|---|
| `fleet` | tüm agent arayüzleri toplamı | `agent_iface_samples` |
| `local` | hub'ın kendi paket yakalaması | `samples` (yalnız `-capture`) |
| `site` | `agents.site` bazında | JOIN |
| `agent` | agent bazında | `agent_id` |

`per_site` / `per_agent` config ile kapatılabilir. Değerlendirmede aday sapmalar
`|z|`'ye göre sıralanır, en çok `max_surfaced` (varsayılan 8) tanesi ateşlenir —
5.000 agent'ta uyarı seli olmaz. Panel (`/anomali`) hepsini gösterir.

### Metrikler (`metric`)

| metric | birim | kaynak | sinyal |
|---|---|---|---|
| `bps` | bit/sn | arayüz sayaç LAG delta ×8 | hacim sıçraması |
| `dns_qps` | sorgu/sn | `agent_dns.queries` | DNS tünelleme / DGA |
| `proc_bps` | bit/sn | `process_traffic` bytes ×8 | süreç bazlı sızdırma |

### Mevsimsel kova (`seasonality`)

| değer | kova sayısı | ayrım |
|---|---|---|
| `hourly` | 24 | saat-of-day |
| `weekday` (vars.) | 48 | hafta içi/sonu × saat |
| `dow` | 168 | haftanın günü × saat |

Gerçek trafik hafta içi ve sonu belirgin farklıdır; tek bir saat kovası
Pazartesi sabahını Cumartesi gecesiyle aynı sepete koyardı.

### EWMA drift

Baseline `baseline_days` (varsayılan 21) günlük pencereden kurulur ama her gün
**gün-yaşına göre** ağırlıklanır:

```
w(yaş) = 2^(−yaş / ewma_half_life_days)     # varsayılan yarı-ömür 10 gün
```

Yeni günler eskilerden ağır basar → baseline yavaş drift'e (kalıcı trafik
artışı) uyum sağlar, ani sıçramaya değil.

## Materyalize baseline

`anomaly_baseline` tablosu `(dim, metric, key, bucket)` → Welford momentleri
`(n, mean, m2)`; `std = sqrt(m2/n)` (popülasyon varyansı). **Lider-kapılı**
(`LeaderKeyAlerts`) bir **saatlik rebuild** bu tabloyu doldurur; `checkAnomaly`
değerlendirme başına canlı `LAG` taraması yapmaz (5.000 agent × 7 gün ölçekte
kritik). Yeniden kurulum tek transaction (okuyucular commit'e kadar eski
baseline'ı görür).

`min_samples` (varsayılan 120) altındaki kovalar "ısınıyor" sayılır ve
değerlendirilmez.

## Yaşam döngüsü

Ateşlenen anomali `alert_events` yaşam döngüsüne girer (bkz.
[`ALERTING.md`](ALERTING.md)): dedup, otomatik-çözülme (koşul-tabanlı — mevcut
kova artık aday değilse çözülür), korelasyon, susturma.

## API

- `GET /api/v1/anomaly/baseline?dim=&metric=&key=` — beklenen eğri (mean ± std
  per kova). Site-kapsamlı kimlik yalnız kendi sahasını/filonun görür.
- `GET /api/v1/anomaly/active` — o an z eşiğini aşan tüm sapmalar (sıralı,
  `max_surfaced` sınırsız).

## Yapılandırma

`alert_config` JSON'unda `anomaly`:

```
seasonality        weekday        # hourly | weekday | dow
baseline_days       21            # 1..90
ewma_half_life_days 10            # 0 = eşit ağırlık
sensitivity         3.0           # z eşiği
crit_z              5             # bunun üstü → crit
min_samples         120
window_min          5             # karşılaştırma penceresi
per_site / per_agent  true
max_surfaced        8
metrics             [bps, dns_qps, proc_bps]
min_abs_delta_bps   500000        # bps/proc_bps gürültü tabanı
min_abs_delta_qps   5             # dns_qps gürültü tabanı
```

## Kapsam dışı

ML tabanlı tespit (değişim-noktası, tahmin, otomatik eşik öğrenme) v1.1
kapsamında değil — model bilinçli olarak istatistiksel (z-skoru / EWMA /
mevsimsel).
