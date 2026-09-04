---
target: frontend/src/components/DevicesCard.tsx
total_score: 20
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 4
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/DevicesCard.tsx"
target_fingerprint: "sha256:ea6771995bf959d2023748cca5d356f8c89aac768ba18f908cf3d94e197098af"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/DevicesCard.tsx
timestamp: 2026-09-04T19-34-35Z
slug: frontend-src-components-devicescard-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 2 | İlk yüklemede yükleme göstergesi yok; `Kaydet` submit sırasında devre dışı kalmıyor (çift-tık riski); silme başarı bildirimi vermiyor, satır sessizce kayboluyor |
| 2 | Gerçek Dünya Eşleşmesi | 3 | SNMPv3 auth/priv protokol çiftleri, VDOM alanı, `ifInDiscards/ifOutDiscards` — gerçek NetOps kelime dağarcığı |
| 3 | Kullanıcı Kontrolü ve Özgürlük | 2 | "Vazgeç" formu düzgün kapatıyor; ama "arayüzler" paneli açıldıktan sonra kapatma kontrolü yok, silmenin geri alması yok |
| 4 | Tutarlılık ve Standartlar | 3 | Vendor rozeti projenin 3-parçalı opaklık formülüne tam uyuyor; SNMP versiyon seçici alanların altında render olması iç-tutarlılığı bozuyor |
| 5 | Hata Önleme | 0 | Silme tek tık/Enter, onay yok; zorunlu alanlar yalnızca placeholder'da `*` ile işaretli — hiçbir yerde HTML `required` yok |
| 6 | Tanıma > Hatırlama | 2 | Tüm select seçenekleri görünür (iyi); ama her metin alanının tek kimliği placeholder — yazınca kayboluyor |
| 7 | Esneklik ve Verimlilik | 2 | Toplu işlem/kısayol yok (küçük CRUD yüzeyi için makul); ama submit-sırası disabled durumu eksikliği hızlı kullanıcıyı çift-POST'a davet ediyor |
| 8 | Estetik ve Minimalist Tasarım | 2 | Yoğunluk DESIGN.md'nin NOC estetiğine kasıtlı uyuyor, ama cihaz satırı (8 rekabet eden eleman) ve arayüz tablosu (6 kolon) canlı ortamın verdiği ~320px konteynerde bu kasıtlı yoğunluğu bile aşıyor |
| 9 | Hata Tanıma/Kurtarma | 2 | Yüzeye çıkan doğrulama mesajları iyi yazılmış Türkçe ("fortigate için api_token zorunlu"); ama bazı 500-yolları (`devices.go:52,143,162,183,197,207`) ham Go hata metnini aynı rose banner'a sızdırıyor |
| 10 | Yardım ve Dokümantasyon | 2 | Şifreleme için tek statik satır var; SHA/SHA256/SHA512/MD5 veya AES/AES256/DES seçimini açıklayan bağlamsal yardım yok |
| **Toplam** | | **20/40** | **Acceptable — bir P0 tarafından çapalanmış, belirgin iyileştirme gerekiyor** |

## Tasarım Özgünlüğü Hükmü

**Karma — özgün alan modellemesi, jenerik güvenlik etkileşimi.** Veri modeli gerçekten bazNTMS'e ait: tam SNMPv3 auth/priv protokol çiftleri, FortiGate çok-kiracılı VDOM alanı, yalnızca gerçek QoS drop'ları debug etmiş birinin ekleyeceği `ifInDiscards/ifOutDiscards` kolonu, kimlik bilgisi yapıştırılmadan hemen önce görünen "AES-GCM kasada şifreli" güven metni. Bu iskelet bir admin-CRUD tablosu değil. Ama *silme* etkileşimi — düz metin link, onay yok, geri alma yok — dosyadaki en jenerik, kutudan-çıkma paterni, ve tam olarak dosyanın en yüksek riskli eylemi (izlenen bir ağ varlığını geri döndürülemez şekilde kaldırma) üzerinde oturuyor. Ürünün kimliği form alanlarında yaşıyor; risk yönetiminde yaşamıyor.

**Deterministik tarama** (`detect.mjs --json`): statik CLI **temiz** (`[]`, exit 0) — ama bu dosya türü (`.tsx`) regex-tabanlı kalıp eşleştirmede çalışıyor, hesaplanmış stil/kontrast gerektiren kurallar (contrast, nesting derinliği) bu modda tetiklenemiyor; metodoloji boşluğu, gerçek "temiz" değil. Tarayıcı-içi enjeksiyon sayfa genelinde 260 bulgu döndürdü; DOM sınır-kutusu ile yalnızca DevicesCard.tsx'in kendi alt-ağacına ait olan **11'i** ayrıştırıldı:

| # | Kural | Detay (ölçülmüş) | Satır |
|---|---|---|---|
| 1 | `ai-color-palette` | Cyan buton | 87-92, "+ Cihaz Ekle" |
| 2 | `tiny-text` | 11px altyazı | 93-95 |
| 3 | `low-contrast` | **3.6:1** (4.5:1 gerekli) — beyaz `#0092b8` üzerinde | 266, "Kaydet" submit butonu |
| 4 | `undersized-ui-text` | 10px "router" rozeti | 108 |
| 5 | `ai-color-palette` | Cyan cihaz-adı linki | 109-111 |
| 6 | `undersized-ui-text` | 9px "snmp v2c" | 78-80, `vendorBadge()` |
| 7 | `undersized-ui-text` | 10px "son poll" | 115-117 |
| 8 | `low-contrast` | **2.4:1** — `text-slate-600` | 132-134, `sys_descr` |
| 9 | `tiny-text` | 11px | aynı paragraf |
| 10 | `nested-cards` | form içinde form-kartı | 216, `DeviceForm` |
| 11 | `nested-cards` | kart içinde kart | 106, cihaz-satırı div |

**Bulgu #8, DESIGN.md'nin kendi belgelediği bilinen sorunla birebir eşleşiyor**: "slate-600 ~2.4:1... Legacy — yeni/dokunulan koddan çıkarılıyor... diğer dosyalar henüz taşınmadı" — DevicesCard.tsx o taşınmamış dosyalardan biri. Kendi taramamda dosyada **6 ayrı** `text-slate-600` kullanımı buldum (satır 102, 115, 133, 136, 195 `placeholder:text-slate-600`, 269) — B'nin bulduğu tekil örnek, aslında sistemik bir örüntünün ucu.

**Form etiket ölçümü (DOM, `.labels` property)**: `DeviceForm`'daki **7 alanın tamamında** `labels.length: 0`, `id: null`, `aria-label: null` — yalnızca `placeholder` var. Bu, Assessment A'nın bağımsız gözlemiyle (accessibility tree'de alan adı = placeholder metni) birebir doğrulandı — iki izole değerlendirme aynı sonuca ayrı yöntemlerle ulaştı.

**Muhtemel yanlış pozitifler**: `ai-color-palette` bulguları (#1, #5) — DESIGN.md Rx Cyan'ı "birincil marka vurgusu (linkler, aktif nav)" olarak tanımlıyor; `Overview.tsx` için zaten aynı gerekçeyle bir ignore-rule var. `undersized-ui-text` #4/#6 (rozet/badge, 9-10px) DESIGN.md'nin "Micro Scope Rule"una makul uyuyor (kısa tür rozetleri istisna) — ama #7 (son-poll, 10px) aynı zamanda yasak `text-slate-600` ile eşleşiyor, yani birleşik bir ihlal (boyut belki kabul edilebilir, renk değil).

## Öne Çıkanlar
1. Vendor rozeti + şifreleme mesajı gerçekten ürüne özgü (satır 72-81) — turuncu REST-API rozeti DESIGN.md'nin sabit-anlam sözleşmesine tam uyuyor, dekorasyon değil güvenlik açıklamasıyla eşleşmiş.
2. Vendor/versiyon bazlı kademeli alan açığa çıkarma yapısal olarak doğru (satır 230-256) — CSS gizleme değil, gerçek koşullu render.
3. Cihaz satırının `flex flex-wrap`'i (satır 107) canlı ortamda gerçekten satır kırıyor — rozet/butonlar ~320px genişlikte ikinci satıra düzgün taşıyor; formun grid'i (aşağıda) aynı esnekliği göstermiyor.

## Öncelikli Sorunlar

**[P0] Tek tıkla, onaysız, kalıcı cihaz silme**
- **Neden önemli**: `remove()` (satır 60-63) `sil` butonunun `onClick`'inden (satır 124-130) doğrudan `DELETE` isteği ateşliyor — onay yok, "adı yazarak onayla" yok, geri-al bildirimi yok. Canlı klavye testiyle doğrulandı: cihaz satırında Tab ile ilerleyince odak normal bir focus ring ile doğrudan bu butona geliyor, hiçbir ara adım yok. Bir yanlışlıkla Enter'a basma (bir olay sırasında hızlı Tab'lanırken makul bir senaryo) gerçek bir izlenen ağ cihazını kalıcı olarak kaldırıyor; backend'de de soft-delete yok (`internal/server/devices.go:161` doğrudan `store.DeleteDevice` çağırıyor). Ayrıca `sil`/`arayüzler` butonlarının erişilebilir ismi tüm satırlarda aynı (statik `title="Cihazı sil"`) — ekran okuyucu kullanıcı hangi cihazı sildiğini adından değil yalnızca çevresel bağlamdan anlıyor.
- **Düzeltme**: DELETE isteği ateşlenmeden önce bir onay adımı ekle (native `confirm()` taban çizgisi, ya da satır-içi "silmek istediğinize emin misiniz?" iki-adımlı buton); `sil`/`arayüzler` butonlarının erişilebilir ismine cihaz adını enjekte et.
- **Önerilen komut**: `/impeccable harden`

**[P1] Form alanlarının hiçbirinde gerçek `<label>` yok — yalnızca placeholder**
- **Neden önemli**: DOM ölçümüyle doğrulandı (`.labels` property) — 7 alanın tamamında `labels.length: 0`, `id: null`. İki izole değerlendirme bağımsız yöntemlerle aynı sonuca ulaştı. Placeholder yazılınca kayboluyor (WCAG 3.3.2 ihlali) ve `placeholder:text-slate-600` (satır 195) zaten ~2.4:1 kontrastta — yapısal + renk, aynı alanlarda çifte ihlal.
- **Düzeltme**: Her alana görünür `<label>` ekle (Triad kalıbındaki gibi kompakt/uppercase kalabilir), `placeholder:text-slate-600` → `placeholder:text-dim-aa`.
- **Önerilen komut**: `/impeccable harden`

**[P1] Form grid'i viewport genişliğini varsayıyor, kendi konteyner genişliğini değil**
- **Neden önemli**: `grid-cols-2 md:grid-cols-4` (satır 217) ve SNMPv3 satırı `md:grid-cols-5` (satır 245) *viewport* kırılma noktası (`md`=768px), *konteyner* değil. Canlı `/cihazlar` sayfasında DevicesCard ~320-340px genişliğinde bir yarı-sütun kartında oturuyor — ama 1280px viewport yine de `md:` kuralını tetikliyor. Canlı ekran görüntüsü placeholder metninin okunaksız parçalara kırpıldığını doğruluyor: `"ad * (core-sv"`, `"site (RBAC s"`. Tam da tek kimlik kaynağı olan placeholder'ın (bkz. üstteki P1) en dar olduğu anda okunamaz hale gelmesi — iki sorun tek anda çakışıyor.
- **Düzeltme**: Viewport-bazlı grid yerine `@container` sorgusu veya bu bileşenin gerçekçi genişliğinde koşulsuz `grid-cols-2`'ye düş.
- **Önerilen komut**: `/impeccable layout`

**[P1] Birincil CTA (Kaydet) kontrastı AA'yı geçmiyor**
- **Neden önemli**: Ölçülen 3.6:1 (4.5:1 gerekli) — beyaz metin `bg-cyan-600` (`#0092b8`) üzerinde (satır 266). Bu, docs-site landing page'in Faz 0'da düzelttiği tam aynı sınıf sorun (birincil CTA kontrastı) — orada çözülmüştü, burada henüz yok.
- **Düzeltme**: `bg-cyan-600` → daha koyu bir cyan tonu (ör. `bg-cyan-700`) veya metin rengini koyulaştır; 4.5:1'i geçene kadar test et.
- **Önerilen komut**: `/impeccable harden`

**[P2] Arayüz-detay tablosu son kolonu kırpıyor, yatay kaydırma yok**
- **Neden önemli**: Sarmalayıcı (satır 148) yalnızca `overflow-y-auto` içeriyor — `overflow-x-auto` yok. Aynı dar kartta 6. kolon ("Atılan in/out") canlı ekran görüntüsünde kart kenarında kırpılıyor, veriye ulaşacak scrollbar yok (veri DOM'da var, sayfa-metni çıkarımıyla doğrulandı). Bu tam olarak DESIGN.md'nin kendi "Do" kuralının öngördüğü senaryo ("dar ekranlarda... `overflow-x-auto` + `min-width`") — ama bu tablo projenin kendi belgelediği kalıbı izlemiyor.
- **Düzeltme**: Sarmalayıcıya `overflow-x-auto`, tabloya `min-w-[...]` ekle.
- **Önerilen komut**: `/impeccable layout`

**[P2] SNMP versiyon seçici, yönettiği alanların altında render oluyor**
- **Neden önemli**: `snmp_version` seçici (satır 257-264, fiziksel olarak en altta) hangi bloğun (242-256) render olacağını belirliyor — ama o koşullu blok JSX'te seçicinin ÜSTÜNDE. Canlı DOM sırası doğrulandı: `community` şifre alanı (ya da 5 alanlı v3 bloğu) versiyonu değiştiren dropdown'dan önce görünüyor — normal yukarıdan-aşağıya form tarama yönüne ters.
- **Düzeltme**: Versiyon seçiciyi vendor seçicinin hemen ardına, versiyon-bağımlı bloktan önceye taşı.
- **Önerilen komut**: `/impeccable clarify`

**[P2] Submit sırasında bekleme durumu yok, silme başarı bildirimi vermiyor**
- **Neden önemli**: `Kaydet` butonu submit sırasında devre dışı kalmıyor — hızlı bir kullanıcı çift-POST'a davet ediliyor. Silme başarılı olduğunda satır sessizce kayboluyor, hiçbir onay/bildirim yok (Nielsen #1).
- **Düzeltme**: Submit sırasında butonu devre dışı bırak + "kaydediliyor…" durumu; silme sonrası kısa bir "cihaz silindi" bildirimi (mevcut `error` state'inin başarı karşılığı).
- **Önerilen komut**: `/impeccable harden`

**[P3, tartışmalı] `nested-cards` — cihaz-satırı ve form konteyneri**
- **Not**: Dedektör hem cihaz-satırı div'ini (106) hem `DeviceForm`'un kendi çerçevesini (216) "kart içinde kart" olarak işaretliyor. Cihaz-satırı, taranabilir bir liste öğesi kalıbı (satır ayracı gibi) — GeoMapCard'daki segmented-control'e benzer şekilde tartışmalı. Form çerçevesi ise satır-içi bir oluşturma panelini görsel olarak ayırmak için makul bir kalıp. İkisi de düşük öncelik.

**[P3, tartışmalı] `ai-color-palette` — cyan buton/link**
- **Not**: "+ Cihaz Ekle" butonu ve cihaz-adı linki cyan kullanıyor — DESIGN.md Rx Cyan'ı "birincil marka vurgusu (linkler, aktif nav)" olarak tanımlıyor ve `Overview.tsx` için zaten aynı gerekçeyle bir ignore-rule var. Muhtemelen aynı mantık burada da geçerli.

## Persona Kırmızı Bayrakları

**Riley (Stres testçisi)** — bu dosyanın en çok başarısız olduğu persona:
- Cihaz satırında Tab ile `sil`e doğrudan, sıradan bir focus ring'le geliyor — `arayüzler`den farklı olduğuna dair hiçbir sinyal yok.
- `arayüzler` panelini kapatma kontrolü yok (`showIfaces`, satır 65-69, yalnızca `detail`'i set ediyor, hiç toggle etmiyor) — birden fazla açıp kapatmak isteyen Riley sayfayı yenilemekten başka yol bulamıyor.
- Hiçbir alan `required` olmadığından, boş form gönderimi önce gereksiz bir ağ round-trip'i yapıyor, önlenebilir bir durum önceden engellenmiyor.

**Sam (Erişilebilirlik-bağımlı)**:
- 8 metin alanının tamamı yalnızca placeholder-kimlikli (canlı accessibility tree'de doğrulandı: alan adı = placeholder metni).
- `sil`/`arayüzler` her satırda aynı erişilebilir ismi taşıyor — ekran okuyucu kullanıcı çoklu-cihaz listesinde "buton, sil" aynı şekilde tekrar duyuyor, cihaz farkı yalnızca çevresel bağlamdan.
- Placeholder metni `slate-600`'de render oluyor — DESIGN.md'nin kendi belgelediği AA-kontrast hatası, aynı alanlarda çifte isabet.

## Küçük Gözlemler
- Boş-durum mesajı (satır 102) da `text-slate-600` kullanıyor — ilk-çalıştırma yönlendirmesi projenin bilinen-hatalı kontrast tonunda.
- Bazı 500-hata yolları (`devices.go:52,143,162,183,197,207`) ham Go hata metnini aynı UI banner'a sızdırıyor — temiz doğrulama mesajlarıyla aynı görsel kapta, farklı kalite.
- `site` alanı, site-kapsamlı kimlikler için sunucu tarafında sessizce geçersiz kılınabiliyor (`internal/server/devices.go:130-132`) — istemci tarafında bu konuda hiçbir gösterge yok.
- FortiPanel.tsx (bu incelemenin kapsamı dışı ama `detailLabel`'dan bağlı) gerçekten derin FortiGate-özgü modelleme gösteriyor — ürünün özgünlüğünün gerçek olduğunun, yalnızca bu dosyanın güvenlik paternlerine eşit dağılmadığının kanıtı.

## Kışkırtıcı Sorular
- Bir operatörün bir olay sırasında yanlışlıkla bastığı Enter tuşunun bir cihazı kaldırmasının işletmeye maliyeti, tek bir onay diyaloğunun maliyetiyle karşılaştırıldığında nedir?
- Form zaten vendor/SNMP versiyonuna göre alanları kademeli açığa çıkarıyorsa, o açığa çıkarmayı yöneten kontrol neden yönettiği alanların altında saklanıyor?
- ~320px yarı-genişlik kart, `md:` (768px+) varsayan bir form için gerçekten doğru yer mi — yoksa "Cihaz Ekle" kendi tam-genişlik veya modal yüzeyini mi hak ediyor?
