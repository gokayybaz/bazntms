---
target: frontend/src/components/AlertsCard.tsx
total_score: 19
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/AlertsCard.tsx"
target_fingerprint: "sha256:0e4dffccdc4a00cc2b70734c70ffab9ce97a538648c273d6a1f4cb11fb605667"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/AlertsCard.tsx
timestamp: 2026-09-04T19-54-30Z
slug: frontend-src-components-alertscard-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 3 | "Kaydediliyor…" + "kaydedildi · saat" durumları var; alan-bazlı doğrulama geri bildirimi yok |
| 2 | Gerçek Dünya Eşleşmesi | 3 | SIEM/FortiGate terminolojisi doğru ve hedef kitleye uygun |
| 3 | Kullanıcı Kontrolü ve Özgürlük | 2 | Sayfadan ayrılmak değişiklikleri iptal ediyor (dolaylı undo) ama açık bir reset/iptal yok, canlı eşikleri değiştiren Kaydet'ten önce onay yok |
| 4 | Tutarlılık ve Standartlar | 2 | Aynı dosyada label-sarma kalıbı tutarsız uygulanmış; paylaşılan `alertKinds.ts` renk sözleşmesini kırıyor |
| 5 | Hata Önleme | 2 | Sayısal alanlarda `min`/`max` iyi; ama etkin biçimde geri alınamaz bir prod Save'den önce onay yok, port/süreç listeleri sessizce geçersiz girdi düşürüyor |
| 6 | Tanıma > Hatırlama | 2 | Değerler `/api/alerts`'ten önceden dolduruluyor; etiketsiz alanlar (bkz. #4) tahmine zorluyor |
| 7 | Esneklik ve Verimlilik | 1 | Kısayol yok, daraltma yok, bölüme atlama yok; Save'e ulaşmak için 32 ardışık Tab durağı |
| 8 | Estetik ve Minimalist Tasarım | 2 | Tek tek kutular temiz; 8'i eşit görsel ağırlıkla üst üste yığılınca ağır okunuyor |
| 9 | Hata Tanıma/Kurtarma | 0 | Hem `/api/alerts` fetch'i hem save PUT'u her hatayı sessizce yutuyor — sıfır kullanıcı-görünür başarısızlık durumu |
| 10 | Yardım ve Dokümantasyon | 2 | Satır-içi altyazılar bağlamsal yardım gibi işliyor; "z-skoru"/"HEC-token" gibi jargon için tek satır dışında açıklama yok |
| **Toplam** | | **19/40** | **Poor (Acceptable sınırına yakın — neredeyse tamamen #7 ve #9 tarafından çekiliyor)** |

## Tasarım Özgünlüğü Hükmü

**Ürüne dayalı, jenerik değil.** Form yoğun ama bu yoğunluğu gerçek alan bilgisiyle hak ediyor: CEF/LEEF önem-sıralama notu (satır 239), FortiGate SD-WAN eşik dörtlüsü + "0 = kapalı" notu (satır 180-194), ve en çarpıcısı — iki büyük-harf bölüm-ayracı altyazısı ("Yerel yakalama gerektirir" / "Filo/cihaz tabanlı", satır 105-107 ve 170-172) bazNTMS'in gerçek çift-kaynaklı mimarisini (hub paket yakalama vs. filo/cihaz poll) doğrudan ayarlar arayüzünde öğretiyor. Jenerik bir "alert settings" şablonu bunu bilemez.

**Deterministik tarama** (`detect.mjs --json`, her iki dosya ayrı): **her ikisi de temiz** (`[]`, exit 0) — ama bu mimari bir sınırlama, gerçek bir "temiz" değil: `nested-cards`/`low-contrast`/`tiny-text`/`undersized-ui-text` yalnızca tarayıcı-DOM motorunda var, regex motorunda hiç yok (hesaplanmış layout/kontrast gerektiriyorlar). Tarayıcı-içi enjeksiyon sayfa genelinde 105 bulgu döndürdü; AlertsCard.tsx'in kendi render kökünde **60'ı** ayrıştırıldı:

| Kural | Sayı | Kaynak |
|---|---|---|
| `undersized-ui-text` | 41 | 20× olay-akışı zaman damgası (satır 75), 20× tür-rozeti (satır 71, renk `alertKinds.ts`'ten), 1× "Filo/cihaz tabanlı" (satır 170) |
| `ai-color-palette` | 16 | Tamamı `alertKinds.ts:8`'in `proc`→sky rengi (satır 71'de wire edilmiş) |
| `low-contrast` | 10 | İki `h3` başlığı (62,88), 2 bölüm etiketi (198,210), 3× `text-slate-600` (106,171,194,239), **Kaydet butonu (satır 249, 3.6:1)** |
| `nested-cards` | 8 | 8 `rounded-lg border border-slate-800 p-3` bölüm kutusunun her biri (109,124,134,144,151,174,197,209) |
| `tiny-text` | 4 | 4× `text-[10.5px]` paragraf (106,171,194,239) |
| `all-caps-body` | 2 | Bkz. aşağıdaki yanlış-pozitif notu |

**Yanlış pozitif**: `all-caps-body` bulguları ~2.8× şişirilmiş — dedektör iç içe `normal-case` span'ı (satır 106,171) hesaba katmadan ebeveyn `<p>`'nin tüm `textContent.length`'ini sayıyor; gerçek uppercase-render edilen kısım 74/68 değil 26/19 karakter. Alttaki desen (kısa bölüm-ayracı metninde uppercase) gerçek ama DESIGN.md'nin kendi Label rolü (600 ağırlık, uppercase, tracked) tam olarak bunu tanımlıyor — bu iki altyazı işlevsel olarak inline Label gibi davranıyor, uzun paragraf-gövdesi değil. `ai-color-palette` (16, sky/proc) de muhtemelen benzer şekilde yanlış pozitif: DESIGN.md, Proc Sky'ı tam olarak "süreç-tipi (yeni süreç) uyarı sınıfı" olarak tanımlıyor — `KIND_LABELS.proc` zaten "yeni süreç" diyor, birebir eşleşme.

**Erişilebilir-isim DOM ölçümü** (33 form kontrolünün tamamı): **12'sinde (%36) gerçek `<label>` bağlantısı yok**:
- **Hiçbir isim yok (3)**: soğuma alanı (satır 94-100, düz `<div>` içinde bare text+input), SIEM `format`/`transport` `<select>`'leri (218-230, `<select>`'in placeholder yedeği de yok).
- **Yalnızca placeholder (9)**: portlar (126-131), yoksayılacak süreçler (136-141), 4 webhook alanı (200-205), SIEM target/token (232,235).
- **Doğru sarılı (21)**: bant genişliği/anomali/FortiGate alanlarının tamamı + 9 Toggle'ın hepsi (`Toggle` bileşeni koşulsuz `<label>` içine sarıyor).

## Öne Çıkanlar
1. İki mimari-bilinçli bölüm-ayracı (satır 105-107, 170-172) — dekorasyon değil gerçek bilgi mimarisi, ürünün hub-yakalama vs. filo-poll ayrımını öğretiyor.
2. SIEM'in koşullu token/insecure açığa çıkarması (233-238) — canlı doğrulandı, doğru çalışıyor, formun karmaşıklığı iyi yönettiği tek yer.
3. Alan-doğruluğu (CEF/LEEF önem sırası, syslog facility, FortiGate SD-WAN eşikleri) — gerçek bir NOC işleten biri tarafından yazılmış gibi okunuyor.

## Öncelikli Sorunlar

**[P0] Sessiz kaydetme/yükleme hatası — hiçbir hata geri bildirim yolu yok**
- **Neden önemli**: `save()` (34-49) ve ilk `fetch('/api/alerts')` (27-32) her hatayı boş `catch` ile yutuyor. PUT başarısız olursa (ağ sorunu, auth süresi doldu, 500) buton sessizce "Ayarları Kaydet"e geri dönüyor — operatör yeni FortiGate/SIEM/webhook yapılandırmasının canlıya çıktığına inanmaya her türlü nedene sahip, ama çıkmamış. Ürünün asıl hedef kullanıcısı (olay sırasında bu ekrana güvenen NetOps/güvenlik operatörü) için bu kozmetik değil, güven kıran bir güvenlik boşluğu.
- **Düzeltme**: Her iki fetch yoluna da ayrı bir hata durumu ekle (satır-içi banner/toast), "hâlâ kaydediliyor" ile "başarısız" aynı sessiz duruma çökmesin.
- **Önerilen komut**: `/impeccable harden`

**[P1] Paylaşılan renk sözleşmesi `ioc`/`target` için kırık — canlı olay akışında ve sayfa özet rozetlerinde**
- **Neden önemli**: `lib/alertKinds.ts:9,11` `target`'a tek başına `violet-*`, `ioc`'a `red-*` veriyor. DESIGN.md'nin Sabit Anlam Kuralı tam 7 renk adlandırıyor ve Tx Violet'in "asla tek başına birincil vurgu değildir" dediğini söylüyor — `target` bunu doğrudan çiğniyor — `red` ise 7 rengin hiçbirinde yok, ad-hoc bir 8. ton. Bu tek dosya iki görünür yüzeyi besliyor: `AlertsCard.tsx:71-73`'ün olay rozetleri ve `AlertsPage.tsx:27`'nin tür-sayım özet rozetleri — ikisi de operatörün bir olay sırasında izlediği sayfada. `Overview.tsx:68-76` zaten düzeltilmiş eşlemeyi taşıyor (`target`→amber, `ioc`→rose) — paylaşılan dosya aynı düzeltmeyi hiç almadı. `ioc` bir önceki commit'te (`410a3b9`) eklendi — bu eski borç değil, canlı kod. Bugünkü canlı örneklemde (20 olay) bu iki tür rastlantısal olarak yoktu — görsel kırılma şu an gizli, ama kodda doğrulanmış ve bir sonraki IOC eşleşmesi/yeni-hedef uyarısında yüzeye çıkacak.
- **Düzeltme**: `Overview.tsx`'in eşlemesini `lib/alertKinds.ts`'e yansıt (`ioc`→rose, `target`→amber) — böylece her iki tüketici de düzeltmeyi miras alır.
- **Önerilen komut**: `/impeccable colorize`

**[P1] 33 form kontrolünün 12'sinde erişilebilir isim yok — canlı DOM'da doğrulandı**
- **Neden önemli**: Soğuma alanı (en kötüsü, hiç isim yok), 2 SIEM `<select>`'i (hiç isim yok, placeholder yedeği de imkansız — bir ekran okuyucu birbirinden ayırt edilemeyen iki "combobox" duyuruyor), + 9 yalnızca-placeholder alan (portlar, süreç, 4 webhook, SIEM target/token). Aynı dosyanın geri kalanı (~21 alan) doğru `<label>` sarma kalıbını zaten kullanıyor — düzeltme kalıbı dosyada zaten var, yalnızca tutarsız uygulanmış.
- **Düzeltme**: Geri kalan tüm bare input/select'leri dosyada zaten kullanılan `<label className="space-y-1">metin<input/></label>` kalıbına sar.
- **Önerilen komut**: `/impeccable clarify`

**[P2] İlerleyici açığa çıkarma yok — 32 alanlık kesintisiz tek sütun duvarı**
- **Neden önemli**: 7 nav linki + çıkış sonrası, sıradaki 32 odaklanabilir eleman (indeks 8-38) tamamen bu formun alanları, Save butonundan (indeks 39) önce geliyor. 8 bölümün tamamı her zaman tam açık — FortiGate dahil (FortiGate olmayan kurulumlarda muhtemelen alakasız). SIEM'in HTTP-açığa-çıkarması (233-238) kod tabanının bunu iyi yapabildiğini kanıtlıyor, 8 bölümden yalnızca birinde uygulanmış. Aynı 8 kutu ayrıca dedektörün `nested-cards` (8) bulgusunun kaynağı.
- **Düzeltme**: Her bölümü native `<details>`/`<summary>` ile daraltılabilir yap (harici kütüphane gerektirmez, klavye-erişilebilir Enter/Space ile varsayılan tarayıcı davranışı) + form kontrollerini `<fieldset>`/`<legend>` ile grupla — bu aynı zamanda `nested-cards` bulgusunu da çözer (kart değil, semantik form grubu olur) ve gruplama erişilebilirliğini iyileştirir.
- **Önerilen komut**: `/impeccable layout`

**[P2] Mobilde iç grid'ler daralmıyor — 375px'te doğrulandı**
- **Neden önemli**: `grid-cols-3` satırları (111, 157) ~75px/kolon, `grid-cols-2` satırları (180, 203, 217) ~117px/kolon render ediyor — 5 iç grid'in hiçbiri dış konteynerin (`lg:grid-cols-2`, satır 59) sahip olduğu responsive prefix'e sahip değil. "sd-wan gecikme eşiği (ms)" gibi bir etiket, aynı genişlikteki sayı alanının üstünde 2-3 satıra bölünüyor.
- **Düzeltme**: 5 iç grid'in her birine `sm:grid-cols-1` (veya benzeri) fallback ekle.
- **Önerilen komut**: `/impeccable layout`

**[P2] Kaydet butonu kontrastı AA'yı geçmiyor**
- **Neden önemli**: Ölçülen 3.6:1 — `bg-cyan-600` + beyaz metin (satır 249). DevicesCard'da zaten çözülen tam aynı sınıf sorun.
- **Düzeltme**: `bg-cyan-700` (5.36:1) + hover'da `bg-cyan-400`/`text-slate-950` (aynı doğrulanmış desen).
- **Önerilen komut**: `/impeccable harden`

**[P2] Port/süreç listelerinde geçersiz girdi sessizce düşüyor**
- **Neden önemli**: "23, abc, 4444" yazmak "abc"yi hiçbir görünür işaret olmadan `.filter()` ile atıyor (satır 126-131, 136-141).
- **Düzeltme**: Geçersiz jeton sayısı > 0 olduğunda küçük bir satır-içi ipucu göster.
- **Önerilen komut**: `/impeccable harden`

**[P3, tartışmalı] `ai-color-palette` (proc/sky) ve `all-caps-body` (bölüm-ayracı altyazıları) — muhtemelen yanlış pozitif**
- **Not**: Proc Sky, DESIGN.md'de tam olarak "yeni süreç" uyarı sınıfı için tanımlı — `KIND_LABELS.proc` ile birebir eşleşiyor. Uppercase altyazılar DESIGN.md'nin Label tipografi rolüyle (uppercase, tracked, 600 ağırlık) tutarlı, uzun paragraf-gövdesi değil kısa bölüm etiketi işlevi görüyorlar.

## Persona Kırmızı Bayrakları

**Alex (Güç kullanıcı)**: Formda hiçbir klavye kısayolu yok. Save'e ulaşmak 32 ardışık Tab durağı (DOM sırasıyla doğrulandı). Alex'in şu an ilgilenmediği 7 bölümü daraltıp doğrudan ayarladığı bölüme gitme imkanı yok — SIEM açığa-çıkarması kod tabanının bunu yapabildiğini gösteriyor, başka hiçbir yerde uygulanmamış. P0 sessiz-kaydetme-hatası tetiklenirse Alex ayarının kaydolmadığını bilme yolu yok.

**Sam (Erişilebilirlik)**: Soğuma alanı çıplak bir `<div>soğuma<input/>dk</div>` — ekran okuyucu yalnızca "spinbutton" duyuyor. İki SIEM `<select>`'i sıfır erişilebilir isim taşıyor (placeholder yedeği bile imkansız) — birbirinden ayırt edilemeyen iki "combobox". 5 bildirim alanının tamamı yalnızca-placeholder — doldurulmuş bir Discord/Slack/Telegram alanını tekrar ziyaret etmek hangisi olduğunu söylemiyor. Focus göstergesi metin/sayı alanlarında yalnızca 1.5px kenarlık rengi değişimi — düşük görüşte düşük belirginlik.

## Küçük Gözlemler
- Birkaç altyazı ve zaman damgası `text-slate-600` kullanıyor (52,64,75,105-107,170-172,194,239) — DESIGN.md bunu ~2.4:1 ölçüyor, `text-dim-aa`'ya taşınması gerekiyor (Overview.tsx'te zaten yapıldı).
- FortiGate yardımcı altyazısı ("Eşik 0 ise…", satır 194) 4 alanın altında duruyor, hangisine ait olduğu belirsiz.
- Bildirim bölümü 4 tam-genişlik URL/token alanını hangisinin dolu/boş olduğuna dair hiçbir görsel işaret olmadan yığıyor.
- `line-length` bulgusu (SIEM önem-notu, ~87 karakter) — teknik bir referans altyazısı için düşük eyleme geçirilebilirlik, aksiyon gerektirmiyor.

## Kışkırtıcı Sorular
- Bir operatör gerçekten 8 eşik kategorisinin tamamını aynı anda açık mı istiyor, yoksa varsayılan-daraltılmış bir akordeon (yalnızca ayarlanan bölüm açık) bu formun gerçek kullanım şeklini daha iyi mi yansıtır?
- Bir operatörün SIEM push'u veya webhook'u arka plandaki sessiz bir Save hatası yüzünden durursa, bir olay bu boşluğu ortaya çıkarmadan önce kimse nasıl fark eder?
- 7-renk sözleşmesi dosyalar arası elle-kopyalanan bir kuralla uygulanıyor (`Overview.tsx` ve `lib/alertKinds.ts` zaten anlaşmıyor) — `ioc` gibi 8. bir renk daha eklenmişken, paralel kopyalar yerine tek paylaşılan bir kaynağın zamanı gelmedi mi?
