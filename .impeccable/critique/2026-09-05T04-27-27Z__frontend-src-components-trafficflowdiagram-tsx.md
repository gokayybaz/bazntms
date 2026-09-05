---
target: frontend/src/components/TrafficFlowDiagram.tsx
total_score: 13
max_score: 24
na_heuristics: 3,5,9,10
p0_count: 2
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/TrafficFlowDiagram.tsx"
target_fingerprint: "sha256:2485986b0cae54e90f6e1fd452e8c8ea7d50575a201e245dacdc28ca7c3a7661"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/TrafficFlowDiagram.tsx
timestamp: 2026-09-05T04-27-27Z
slug: frontend-src-components-trafficflowdiagram-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 2 | Çoğu kullanıcı için canlı ve etkileyici ama kısmen sahte (sentetik paketler) VE hareket-azaltma kullanıcıları için ilk render'dan sonra tamamen donuk |
| 2 | Gerçek Dünya Eşleşmesi | 4 | Filo/router/internet metaforu gerçek ağ topolojisini birebir yansıtıyor |
| 3 | Kullanıcı Kontrolü ve Özgürlük | n/a | Pasif görüntüleme, çıkılacak bir akış yok |
| 4 | Tutarlılık ve Standartlar | 1 | İcat edilmiş 8. renk (fuchsia) sabit 7-renk sözleşmesiyle çarpışıyor; SVG-içi metin ThroughputChart'ın kendi karışık hex/Tailwind kalıbını terk ediyor; `role="img"` kardeş dosyada zaten gemiye yüklenen düzeltmenin gerisinde kalıyor |
| 5 | Hata Önleme | n/a | Bu bileşende kullanıcı girdisi yok |
| 6 | Tanıma > Hatırlama | 3 | Gösterge satırı renk+yazılı-etiket+canlı-sayı üçlüsünü hep birlikte veriyor; "fuchsia=giden" kullanıcının ThroughputChart'tan öğrendiğiyle ("violet=tx") çelişiyor |
| 7 | Esneklik ve Verimlilik | 0 | Hiçbir kontrol yok — duraklat/yöne-göre-filtrele/sentetik-paketi-kapat/hız yok |
| 8 | Estetik ve Minimalist Tasarım | 3 | Ölçülü, amaca yönelik; firewall'ın dekoratif LED sıraları küçük bir düşüş |
| 9 | Hata Tanıma/Kurtarma | n/a | Salt-okunur görselleştirmede hata durumu yok |
| 10 | Yardım ve Dokümantasyon | n/a | Widget seviyesinde kapsam dışı |
| **Toplam** | | **13/24** (6 sezgi, 4 n/a) | **%54 — Acceptable** |

## Tasarım Özgünlüğü Hükmü

**Son derece ürüne özgü.** Üç-bölgeli sahne (filo ▸ router/firewall kalkanı ▸ dönen küre), her düğümdeki monitör-ikonu, nabız halkalı kalkan, filo büyüdükçe tam kart görünümünden LED-yalnızca "mini" satırlara sıkışan uyarlanabilir yoğunluk — hiçbiri jenerik bir grafik-kütüphanesi kaplaması değil. DESIGN.md'nin adlandırdığı iki "imza" Live Diagram'dan biri olmayı hak ediyor.

**Deterministik tarama**: CLI temiz (`[]`), canlı DOM taraması da bu bileşenin kendi kart sınırı içinde **sıfır** bulgu verdi (sayfadaki 977 bulgunun tamamı başka bileşenlere ait, kapsam dışı). Ama bu "gerçek temiz" değil — aşağıdaki kontrast sorunları dedektörün kör noktasında.

**Renk sözleşmesi karşılaştırması** (DESIGN.md'nin 7 hex kodu ile birebir kıyaslandı): `DIR_COLOR.in`(cyan)/`lan`(emerald)/`log`(amber) tam eşleşiyor. **`DIR_COLOR.out` = `#e879f9` (fuchsia) 7 renkten hiçbiriyle eşleşmiyor** — icat edilmiş 8. renk. `ThroughputChart.tsx` aynı "giden/tx" anlamı için zaten `#a78bfa` (violet) kullanıyor — aynı dashboard'da aynı anlam iki farklı renkle kodlanmış. Ek olarak `DIR_COLOR.lan` (emerald) aynı dosyada `AgentNode`'un "çevrimiçi" durum rengiyle (satır 205) birebir aynı hex — aynı yeşil aynı SVG içinde hem "yerel-ağ yönü" hem "online durumu" anlamına geliyor.

**Kontrast — gerçek arkaplan gradyanına karşı hesaplandı** (`#0f1e33`→`#0a1220`→`#070d18`): `#94a3b8`/`#cbd5e1` geçiyor (6.5-13:1); **`#64748b` (satır 256,354) 3.52-4.09:1 — geçmiyor**; **`#475569` (satır 594,649) 2.21-2.57:1 — ciddi şekilde geçmiyor**, DESIGN.md'nin kendi ölçümüyle (~2.4:1) örtüşüyor. Alt gösterge şeridinde `text-slate-500`(712, tally sayacı) 3.92-4.24:1 ve `text-slate-600`(715, hareket-azaltma metni) 2.46-2.66:1 — ikisi de AA altı. Önerilen `dim-aa` aynı zeminlere karşı 4.80-5.57:1 ile geçiyor.

**Hareket-azaltma — kod izi ile doğrulandı** (canlı emülasyon aracı bu oturumda yoktu): `reduced` true olunca hem olay-tüketen effect (satır 493) hem RAF döngüsü (satır 512-513) tamamen `return` ediyor. `setTally`/`setNetEnd` yalnızca `spawnRef.current` içinde çağrılıyor, o da bir daha hiç tetiklenmiyor — **hareket-azaltma tercih eden kullanıcı ilk render'dan sonra bu widget'tan sonsuza kadar sıfır güncel veri alıyor**, yalnızca animasyon değil.

**SVG erişilebilirlik — canlı doğrulandı**: `role="img"` (satır 567) tüm alt-ağacı tek görüntüye indiriyor — accessibility tree'de yalnızca üst-seviye `aria-label` var, her `AgentNode`'un `<title>`'ı (agent adı+site) hiçbir düğümde görünmüyor. Kardeş `TopologyCard.tsx` bu turda tam bu sorunu zaten çözdü (`role="group"` + düğüm-bazlı `role="button"`+`aria-label`) — bu dosya kendi ailesinin çoktan gemiye yüklediği düzeltmenin gerisinde.

## Öne Çıkanlar
1. Ürüne özgü sahne kurgusu ve uyarlanabilir yoğunluk — jenerik grafik değil.
2. Filo-üyeliği savunması (`resolveIdx`/`DROP`) — çevrimdışı/bilinmeyen agent olaylarını doğru şekilde eliyor, "N çevrimdışı gizli" sayacıyla dürüst kalıyor, zaten test kapsamında.
3. `prefers-reduced-motion` CSS katmanında doğru saygı görüyor (`@media` bloğu keyframe'leri doğru öldürüyor) — sorun bunun VARLIĞI değil, beraberinde neyi de durdurduğu.

## Öncelikli Sorunlar

**[P0] Sentetik "ambient" paketler gerçek trafikten görsel olarak ayırt edilemiyor, gerçek trafik arttıkça DAHA SIK tetikleniyor**
- **Neden önemli**: Tetikleme koşulu (`pps > 0`) Overview.tsx'in "PAKET HIZI" istatistik kutusunu besleyen AYNI canlı sayı — yani sahte paketler yalnızca sıfır-trafik durumunda değil, **neredeyse her zaman** gerçek trafikle iç içe akıyor, ve aralık formülü (satır 523) `pps` yükseldikçe DAHA SIK ateşleniyor. Renk/animasyon/glow birebir aynı, tek ayrım (etiket yokluğu) gerçek paketlerde de zaten yaygın. Bir NOC aracının "gördüğün gerçekten oluyor" vaadini tam da operatörün güvenine en çok ihtiyaç duyduğu anda (yükselen pps) zayıflatıyor.
- **Düzeltme**: Kullanıcı kararı gerekiyor (aşağıda soruluyor).
- **Önerilen komut**: `/impeccable harden`

**[P0] Fuchsia (`#e879f9`) icat edilmiş 8. renk, sözleşme dışı**
- **Neden önemli**: DESIGN.md tam 7 renk tanımlıyor, violet'i "tx/gönderilen" için ayırıyor — `out` yönü tam olarak bu anlamı taşıyor. `ThroughputChart.tsx` aynı anlam için zaten violet kullanıyor. Kardeş `TopologyCard.tsx` bu ailede zaten bir kez "violet tek başına yanlış kullanım" hatasını düzeltmişti — bu dosya daha ciddisini yapıyor: mevcut bir rengi yanlış kullanmak değil, DESIGN.md'de hiç olmayan yepyeni bir renk icat etmek.
- **Düzeltme**: `DIR_COLOR.out` → `#a78bfa` (ThroughputChart'ın "Gönderilen" rengiyle birebir).
- **Önerilen komut**: `/impeccable colorize`

**[P1] `role="img"` her AgentNode'un `<title>`'ını ekran okuyucudan gizliyor**
- **Neden önemli**: Canlı doğrulandı — accessibility tree tüm diyagramı tek `img` düğümüne indiriyor, agent adı/site bilgisi hiçbir yere ulaşmıyor. Kardeş `TopologyCard.tsx` bu turda tam bu deseni zaten kurdu (`role="group"` + düğüm-bazlı `role="button"`+`tabIndex`+`aria-label`).
- **Düzeltme**: `role="img"` → `role="group"`, düğümlere `role="button"`+`tabIndex`+`aria-label` ekle.
- **Önerilen komut**: `/impeccable harden`

**[P1] Hareket-azaltma yalnızca animasyonu değil VERİYİ de donduruyor**
- **Neden önemli**: Kod izi net — `reduced` true olunca olay-işleme effect'i VE RAF döngüsü tamamen duruyor, `tally`/`netEnd` bir daha güncellenmiyor. Bu ayarı açan kullanıcı (genellikle vestibüler/migren nedeniyle, ekran-okuyucudan bağımsız) ilk render'dan sonra widget'tan sıfır canlı bilgi alıyor — ayarın amaçladığından çok daha büyük bir kayıp.
- **Düzeltme**: Veri-tetikleme mantığını (tally/netEnd) hareketten ayır — olay-işleme effect'i `reduced` iken de çalışsın (yalnızca `packetsRef`'e görsel paket eklemeyi atlasın), yalnızca RAF-tabanlı uçuş animasyonu devre dışı kalsın.
- **Önerilen komut**: `/impeccable harden`

**[P2] Ham hex metin dolguları + gösterge şeridi kontrast borcu**
- **Neden önemli**: `#64748b`(256,354)/`#475569`(594,649) SVG içinde, `text-slate-500`(712)/`text-slate-600`(715) HTML şeritte — hepsi AA altı, en kötüsü (715) tam da hareket-azaltma kullanıcısının sonsuza dek okuduğu tek metin.
- **Düzeltme**: SVG metinlerini `fill="#hex"` yerine `className="fill-dim-aa"` yap (hem kontrastı hem ThroughputChart'ın "metin Tailwind class" kuralını düzeltir), şerit `text-slate-500/600`'ü `text-dim-aa`'ya taşı.
- **Önerilen komut**: `/impeccable harden`

**[P3, tartışmalı] `lan`(emerald)/`log`(amber) renklerinin yumuşak yeniden-kullanımı**
- **Not**: `lan` zaten "sağlıklı/online" anlamı taşıyan emerald'ı (hatta aynı dosyada AgentNode'un online-rengiyle birebir aynı hex'i) trafik-yönü için kullanıyor; `log` "uyarı eşiği" anlamı taşıyan amber'ı her syslog olayı için (ciddiyet fark etmeksizin) kullanıyor. Fuchsia kadar kesin bir ihlal değil (A ve B ikisi de "daha savunulabilir" diyor) ama Fixed Meaning Rule'ün ikinci bir bakışı hak ediyor.

## Persona Kırmızı Bayrakları

**Sam**: Ekran okuyucu tüm diyagram için tek düz `aria-label` alıyor, hiçbir düğüm klavyeyle odaklanabilir değil (kardeş `TopologyCard`'ın aksine) — bu widget klavye akışında ölü bir bölge. Hareket-azaltma açıksa (P1) widget mount'tan sonra sonsuza dek sessiz kalıyor, hayatta kalan tek metin de AA altı (P2) — tam da bu personaya üst üste iki başarısızlık.

**Riley**: Sentetik-paket davranışını (P0) "çalışıyormuş gibi görünüyor ama sonuçları sessizce uyduruyor" diye işaretler — tam kanonik kırmızı bayrağı. Firewall'un yanıp sönen LED üçlüsünün de (satır 273-283) gerçek bir sinyale bağlı olmadan sabit bir zamanlayıcıda döndüğünü, sentetik-paketlerle aynı "canlı görünüyor ama değil" örüntüsünün küçük bir ikinci örneği olduğunu fark eder.

## Küçük Gözlemler
- Firewall'un ilk LED sırası (273-283) hiçbir gerçek sinyale (paket varışı, alarm durumu) bağlı değil, sabit zamanlayıcıda dönüyor — sentetik-paket sorunuyla aynı "canlı görünüyor ama değil" örüntüsünün küçük bir tekrarı.
- Yaklaşık bir düzine SVG metni ham hex kullanıyor, ThroughputChart'ın kendi "metin Tailwind class" kuralından sapıyor (görsel etkisi yok, tutarlılık borcu).
- Dar ekranlarda (375px, canlı test edildi) davranış doğru — `overflow-x-auto`+`min-w-[680px]` metni küçültmek yerine kaydırıyor, dokunulmamalı.

## Kışkırtıcı Sorular
- Sentetik paketlerin dürüst gerekçesi "boşta demo ölü görünmesin" ise, bu bir güvenlik-izleme aracında kalıcı bir doğruluk riskine değer mi — gerçek dağıtımlarda kapalı bir `demo`/`seed` modu bayrağı olabilir mi?
- `TopologyCard.tsx` bu ailede `role="img"` ve renk-sözleşmesi sorunlarını zaten bir kez çözmüşken, tüm SVG bileşenlerinin bu iki emsale karşı tek bir turda denetlenmesi mi, yoksa dosya dosya tek tek mi fark edilmesi daha iyi?
