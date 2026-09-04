---
target: frontend/src/components/ComplianceCard.tsx
total_score: 20
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/ComplianceCard.tsx"
target_fingerprint: "sha256:427e2cc2bdec832fd1b8c844c1ee0de6531b520ed256f56e02cbfdd4932313fe"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/ComplianceCard.tsx
timestamp: 2026-09-04T20-25-11Z
slug: frontend-src-components-compliancecard-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 2 | Motor rozetleri iyi; `addReview()` gönderiminde başlangıç/başarı/hata sinyali yok |
| 2 | Gerçek Dünya Eşleşmesi | 3 | ISO/5651 terminolojisi hedef kitleye tam uyuyor |
| 3 | Kullanıcı Kontrolü ve Özgürlük | 2 | İlk prompt'ta tam çıkış var, ikincide iptal = sessiz devam; tutanak düzenleme/silme yok |
| 4 | Tutarlılık ve Standartlar | 2 | Kart/rozet/tipografi dili tutarlı; native prompt() ve violet rozet kullanımı sistemden sapıyor |
| 5 | Hata Önleme | 2 | `type="date"` biçim kısıtı iyi; tutanak oluşturma akışında doğrulama/onay yok |
| 6 | Tanıma > Hatırlama | 3 | Her şey görünür buton/etiket; ISO kod açıklaması hafızaya güveniyor |
| 7 | Esneklik ve Verimlilik | 1 | Klavye kısayolu yok, hazır tarih aralığı yok, toplu işlem yok |
| 8 | Estetik ve Minimalist Tasarım | 3 | Yoğun ama düzenli, Card sınırları içinde gürültüsüz |
| 9 | Hata Tanıma/Kurtarma | 1 | `load()` hatası gösteriliyor; `addReview()` hatası tamamen yutuluyor |
| 10 | Yardım ve Dokümantasyon | 1 | Bağlamsal yardım yok; nadir kullanım (aylık/yıllık) tam da rehberlik gerektirir |
| **Toplam** | | **20/40** | **Acceptable (sınırda) — belirgin iyileştirme gerekiyor** |

## Tasarım Özgünlüğü Hükmü

**Ürüne dayalı, ama bölünmüş.** ISO 27001 ek-madde referansları (A.8.15, A.8.2, A.5.28), 5651-özel terminoloji (TSA, WORM, hash-imzalı log) ve `bazntmsctl verify -bundle <dosya>` CLI ipucu jenerik bir panele kopyalanamayacak kadar özgü. Rozet opaklık formülü, mono/sans ayrımı ve stat kutuları DESIGN.md'ye harfiyen uyuyor. Ama kartın **tek "yazma" etkileşimi** — bir uyumluluk tutanağı oluşturmak, ürünün var oluş nedeni olan denetim izini üretmek — art arda iki `window.prompt()` çağrısıyla yapılıyor: platformun en jenerik, stillendirilemez, ürün kimliğinden tamamen kopuk ilkeli. Son derece özel domain içeriğinin ortasında tek bir jenerik etkileşim kalıbı.

**Deterministik tarama** (`detect.mjs --json`): 1 bulgu, `gray-on-color` (satır 42, exit 2). **Yanlış pozitif**: satır 42 `badge()` yardımcısının ternary'si — `text-slate-500` ve `bg-emerald-500` asla aynı render edilen elemanda birleşmiyor (birbirini dışlayan iki dal), dedektörün regex motoru ternary/dal farkındalığı olmadan aynı JSX attribute string'inde ikisini de görüp eşleştiriyor.

Tarayıcı-içi enjeksiyon sayfa genelinde 48 bulgu döndürdü; ComplianceCard.tsx'e ait olanlar:
- `undersized-ui-text`: 9px rozet metinleri (111-115) + 10px çeşitli etiketler (116-118,123,135,177-179,185,190,196)
- `low-contrast` **3.9:1** (yeni bulgu — DESIGN.md yalnızca 600/700'ü ölçmüştü): `text-slate-500` — badge off-durumu (42), "PII maskele" etiketi (167), tutanak kullanıcı adı (213)
- `low-contrast` **2.5:1**: `text-slate-600` — saklama/doğrulama/tutanak-yok/zaman damgası (116-118,177-179,200,217-219)
- `nested-cards`: 3 stat kutusu + delil paketi çubuğu + tutanak satırları — **çekince**: bu iç div'ler bu dosyaya ait, ama dosyanın kendi kökü düz bir `<div>`, dışarıdaki kartlı çerçeve bir üst (sayfa-seviyesi) bileşenden geliyor; DevicesCard'daki cihaz-satırı ile aynı tartışmalı liste-öğesi deseni

**Tarih alanı DOM ölçümü**: her iki `from`/`to` alanının da (`labelsLength: 0`, `ariaLabel: null`) **sıfır erişilebilir ismi yok** — iki bağımsız değerlendirme aynı sonuca ayrı yöntemlerle ulaştı. "PII maskele" checkbox'ı doğru şekilde sarılı (karşılaştırma referansı).

**"indir ↓" linki**: erişilebilir ismi tam olarak "indir ↓" — bağlamsal açıklama yok; link listesiyle gezinen bir ekran-okuyucu kullanıcısı neyin indirileceğini bilemiyor.

**Rozet renk-yalnızca-anlam kontrolü**: `badge()` ve tutanak-türü rozeti **temiz** — metin her zaman renkle birlikte değişiyor (`ok ? on : off`, `{r.kind}`), saf renk-anlamı ihlali yok.

## Öne Çıkanlar
1. Domain sadakati — ISO/5651 kod referansları + TSA/WORM/PII terminolojisi + verify-CLI ipucu, jenerik bir panelin üretemeyeceği türden.
2. Sistem uyumu — rozet formülü, mono/sans ayrımı, kart çerçevesi DESIGN.md'ye harfiyen uyuyor.
3. Mobil davranış — 375px'te delil paketi satırı canlı test edildi, kırılmıyor.

## Öncelikli Sorunlar

**[P1] `addReview()` — tutarsız iptal semantiği + sessiz hata yutma**
- **Neden önemli**: İlk `prompt()` iptal edilirse temiz çıkılıyor (satır 87), ama ikinci `prompt()` iptal edilirse `?? ''` (satır 88) boş bulguyla **sessizce POST'a devam ediyor** — kullanıcı "vazgeçtim" sanırken gerçek bir tutanak kaydı oluşuyor. `.catch(() => {})` (satır 98) POST başarısız olsa bile hiçbir hata göstermiyor; başarıda da hiçbir onay yok.
- **Düzeltme**: İkinci prompt'ta da `null` dönüşünü tam iptal say; POST hatasını görünür hale getir; başarıda kısa bir onay sinyali ekle.
- **Önerilen komut**: `/impeccable harden`

**[P1] Tarih alanlarının erişilebilir adı yok**
- **Neden önemli**: DOM ölçümüyle doğrulandı (iki bağımsız yöntem) — her iki `<input type="date">` da isimsiz, ekran okuyucu "başlangıç"/"bitiş" ayrımını hiç duyuramıyor.
- **Düzeltme**: Her ikisine `aria-label` ekle.
- **Önerilen komut**: `/impeccable harden`

**[P2] Oluşturulan tutanaklar için düzenleme/silme yok, değiştirilemezlik açıklanmıyor**
- **Neden önemli**: Yukarıdaki P1 ile birleşince, yanlışlıkla oluşan bir tutanağı düzeltmenin hiçbir yolu yok. Bu kasıtlı bir WORM/değiştirilemezlik ilkesi olabilir ama arayüzde hiç açıklanmıyor — kullanıcı bug mu tasarım mı bilemiyor.
- **Düzeltme**: Kısa bir mikro-metinle açıkça belirt ("tutanaklar oluşturulduktan sonra değiştirilemez").
- **Önerilen komut**: `/impeccable clarify`

**[P2] `text-slate-500`/`600` kontrast hataları, 7 yerde**
- **Neden önemli**: DESIGN.md 600/700'ü ~2.4:1 ölçüp bilinen sorun işaretliyor; bu turda **slate-500 de** ölçüldü (3.9:1, AA altı) — badge off-metni(42), "PII maskele"(167), kullanıcı adı(213) + zaten bilinen 600 örnekleri saklama/doğrulama/tutanak-yok/zaman damgası(116-118,177-179,200,217-219).
- **Düzeltme**: 7 satırın tamamını `text-dim-aa`'ya taşı.
- **Önerilen komut**: `/impeccable harden`

**[P2] Fixed Meaning Rule ihlali — `violet` tek başına "access" rozeti için**
- **Neden önemli**: DESIGN.md violet'i her zaman cyan ile eşleşmesi gereken tx-trafik rengi olarak sabitliyor. Burada tx ile ilgisi olmayan bir kategori etiketi için tek başına kullanılıyor (satır 207). ("log" rozetinin cyan kullanımı sorun değil — DESIGN.md cyan'ı ayrıca "birincil marka vurgusu (linkler)" olarak da tanımlıyor, genel amaçlı kullanım payı var; violet'in böyle bir istisnası yok.)
- **Düzeltme**: "access" rozetini nötr slate tonuna taşı (badge()'in "off" estetiğiyle tutarlı) — iki kategori arasında zaten metin ayrım sağlıyor, yeni bir anlam-rengi icat etmeye gerek yok.
- **Önerilen komut**: `/impeccable colorize`

**[P2] "indir ↓" linki bağlamsal erişilebilir isim taşımıyor**
- **Neden önemli**: Erişilebilir isim tam olarak "indir ↓" — hangi tarih aralığının/maskeleme durumunun uygulandığına dair hiçbir ipucu yok.
- **Düzeltme**: `aria-label="Kanıt paketini indir (${from||'başlangıç'} – ${to||'bitiş'}, PII ${mask?'maskeli':'maskesiz'})"` gibi bağlamsal bir etiket ekle.
- **Önerilen komut**: `/impeccable harden`

**[P3, tartışmalı] `gray-on-color` (satır 42, CLI yanlış pozitif) ve `nested-cards` (stat kutuları/delil paketi/tutanak satırları)**
- **Not**: İlki kanıtlı yanlış pozitif (birbirini dışlayan ternary dalları). İkincisi DevicesCard'da zaten ele alınan aynı tartışmalı liste-öğesi deseni — kart değil, taranabilir satır ayracı.

## Persona Kırmızı Bayrakları

**Riley**: İkinci prompt'u iptal ettiğinde sistem "başarısız" göstermek yerine sessizce boş-bulgulu bir tutanak **oluşturuyor** — tam olarak aradığı "görünürde çalışıyor ama yanlış sonuç üretiyor" kırmızı bayrağı. Boş tarih aralığıyla `evidenceUrl()` `from`/`to`'yu tamamen atlıyor — bunun "tüm zamanlar" mı hata mı olduğuna dair arayüzde hiçbir ipucu yok.

**Sam**: Tarih alanları isimsiz (yukarıda detaylı). Olumlu: rozetler saf renk-anlamı ihlali taşımıyor — metin her zaman renkle birlikte değişiyor.

## Küçük Gözlemler
- Motor tamamen kapalıyken (canlı demo durumu) beş sessiz gri rozetten fazlası yok — "etkinleştirmek için →" yönlendirmesi eksik.
- `+ log inceleme`/`+ erişim incelemesi` butonları "indir ↓" ile aynı görsel ağırlıkta — PRODUCT.md'nin asıl başarı eylemi (tek-tık kanıt paketi) vurgulanmıyor.
- Motor durumu satırında 5 rozet + saklama metni tek satırda (chunking sınırını aşıyor).

## Kışkırtıcı Sorular
- İki native `prompt()` zinciri, kartın geri kalanının tasarım diline hiç uymuyor — bilinçli bir kapsam kararı mıydı, yoksa ertelenmiş bir iş mi?
- Tutanakların düzenlenemez/silinemez olması bilinçli bir WORM ilkesi mi — öyleyse neden kullanıcıya söylenmiyor?
- Motor tamamen kapalıyken beş sessiz gri rozet, bir denetim görevlisi için yeterli bir alarm mı?
