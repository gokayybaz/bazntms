---
target: frontend/src/components/GeoMapCard.tsx
total_score: 20
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 3
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/GeoMapCard.tsx"
target_fingerprint: "sha256:d6b33f488b51e8f6ea28a267dd07dfe9f43a542adea13e4aba19df1892aff48a"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/GeoMapCard.tsx
timestamp: 2026-09-04T18-34-13Z
slug: frontend-src-components-geomapcard-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 2 | Zaman aralığı değişince (`setMinutes`) `loaded` sıfırlanmıyor — tıklama ile veri değişimi arasında hiçbir görsel geri bildirim yok. |
| 2 | Match System / Real World | 3 | ISO2 kodları (TR/BG/US) operatör konvansiyonuna uygun; tooltip tam isme genişliyor ("Türkiye (TR)"). |
| 3 | User Control and Freedom | 2 | Tek kontrol zaman-aralığı; belirli bir ülkeyi filtreleme/sabitleme yok, tooltip'i mouse'suz dondurma yok. |
| 4 | Consistency and Standards | 2 | `text-slate-600` (satır 107, 109) vs. aynı dosyada 6 satır aşağıdaki kardeş boş-durum (`Overview.tsx:662`) `text-dim-aa` kullanıyor — aynı dosyada iki kontrast standardı. |
| 5 | Error Prevention | 3 | Boş-veri durumu spesifik ve yönlendirici; yanlış kullanılabilecek bir girdi yüzeyi yok. |
| 6 | Recognition Rather Than Recall | 2 | Görsel kullanıcı için iyi (her zaman görünür özet şeridi + hover); AT kullanıcıları için sıfır — içerik `role="img"` içinde. |
| 7 | Flexibility and Efficiency | 1 | Tek sabit yol (3 buton), klavye kısayolu yok, toplu/karşılaştırma yok, mouse'suz veri inceleme yok. |
| 8 | Aesthetic and Minimalist Design | 2 | Yinelenen "GeoIP · uzak uç noktalar" başlığı (`Overview.tsx:655` + `GeoMapCard.tsx:101-103`); canlı ortamda TR/BG/RO balon/etiket çakışması doğrulandı. |
| 9 | Help Recognize/Diagnose/Recover from Errors | 1 | `catch { /* yoksay */ }` (satır 71) — başarısız fetch hiçbir kullanıcı sinyali üretmiyor. |
| 10 | Help and Documentation | 2 | Boş-durum metni alışılmadık derecede spesifik ("MaxMind MMDB veya `-ip-api-lookup` gerekir"); ama balon-boyutu=trafik-hacmi eşlemesini açıklayan bir lejant hiçbir yerde yok. |
| **Toplam** | | **20/40** | **Acceptable (alt sınır, Poor'a yakın)** |

## Tasarım Özgünlüğü Hükmü

**Çizim katmanında bu ürüne özgü; state-yönetim katmanında jenerik.**

`CONTINENTS` (satır 19-34) kasıtlı olarak coğrafi doğruluktan vazgeçip "kaba kıta blobları" kullanıyor — kod yorumu bunun bilinçli bir tercih olduğunu, anlamı balonların taşıdığını açıkça belirtiyor; bu, projenin sıfır-harici-bağımlılık SVG kuralı içinde kalmak için topojson/d3-geo/react-simple-maps'e gitmemek adına düşünülmüş bir karar. Çizim elemanları ham hex (`#0a1120`, `#13233a`, `#22d3ee`) kullanırken metin/lejant Tailwind class kullanıyor — tam olarak DESIGN.md'nin "Charts" bölümünde belgelenen `ThroughputChart.tsx` kalıbı. Trafik hacmi yeni bir renk skalası icat etmek yerine mevcut rx-cyan anlamını balon *yarıçapı* üzerinden yeniden kullanıyor — sözleşmeye tam uyumlu, kasıtlı bir kısıtlama. `GeoMapCard.tsx`, DESIGN.md'nin `box-shadow` için tanımladığı 2 istisnadan biri ("Lifted card") — düz-tasarım kuralından tek sapması bile adlandırılmış, kasıtlı bir istisna, savrukluk değil.

Çatlaklar çevredeki iskelette: `Overview.tsx:655`'teki kart başlığıyla neredeyse birebir aynı ikinci bir altyazı, "yoksay" yorumlu boş bir `catch` bloğu, zaman-aralığı değişiminde hiçbir yükleme geri bildirimi yok. Özenli SVG işçiliği ile özensiz veri katmanı aynı 204 satırda yan yana.

**Deterministik tarama** (`detect.mjs --json frontend/src/components/GeoMapCard.tsx`): statik CLI taraması **temiz** çıktı (`[]`, exit 0) — dosyada hiçbir kural tetiklenmedi. Ancak tarayıcı-içi (runtime) enjeksiyon, statik taramanın kaçırdığı **2 gerçek bulgu** buldu (yalnızca GeoMapCard.tsx'in kendi alt-ağacına ait olduğu DOM'da doğrulandı, sayfa genelindeki 199 bulgudan ayrıştırılarak):

| Kural | Eleman | Satır |
|---|---|---|
| `nested-cards` | Zaman-aralığı toggle konteyneri (`border-slate-700/80` div) | `GeoMapCard.tsx:88` |
| `tiny-text` | Üst araç çubuğu altyazısı (`text-[11px] text-slate-500`) | `GeoMapCard.tsx:101-103` |

Bu, statik regex-tabanlı tarayıcının çözemediği gerçek bir CLI-vs-runtime farkı: `nested-cards` hesaplanmış border/ata bağlamı gerektiriyor, `tiny-text` ise tam 11px eşiğinde oturuyor. SVG ülke etiketleri (`fontSize={9}`, satır 159) kontrol edildi ve **temiz** çıktı — ölçülen gerçek render yüksekliği ~13px (kartın 982px genişliğe ölçeklenen `viewBox="0 0 800 400"`'ü nedeniyle ~1.23× ölçek faktörü) — ama bu genişliğe bağımlı bir risk, dar bir kolonda tekrar eşiğin altına düşebilir.

## Öne Çıkanlar

1. **Hover-durum senkronizasyonu** — balon ile özet şerit arasında (`GeoMapCard.tsx:136, 150-151, 160` vs. `177`) — ucuz, doğru bağlanmış, harita↔liste'yi tek sistem gibi hissettiriyor.
2. **Kasıtlı kaba kıta poligonları, kod-içi gerekçeyle** (satır 19-34) — haritalama kütüphanesi almak yerine sıfır-bağımlılık SVG kuralını koruyan, düşünülmüş bir kapsam kararı.
3. **Spesifik, eyleme dönük boş-durum metni** (satır 109-112) — jenerik "veri yok" yerine tam olarak neyin eksik olduğunu söylüyor ("MaxMind MMDB veya `-ip-api-lookup` gerekir").

## Öncelikli Sorunlar

**[P0] Ülke bazlı veri klavye/ekran-okuyucu kullanıcıları için tamamen erişilemez**
- **Neden önemli**: DOM incelemesiyle doğrulandı — 5 ülke `<g>` elemanının tamamı `tabIndex: -1`, `role` yok, `aria-label` yok, klavye handler'ı yok; üst `<svg role="img" aria-label="...">` tüm alt-ağacı tek statik etikete indirgiyor (`read_page` sıfır çocuk döndürüyor). Tek yedek olan 6-öğelik özet şerit etiketsiz düz metin, tooltip'in gösterdiği oturum-sayısı verisini atlıyor, 6'nın ötesini sessizce kesiyor. Klavye-yalnızca veya ekran-okuyucu kullanıcı için bu widget'ın birincil içeriğinin **tamamının** kaybı.
- **Düzeltme**: Her `<g>`'ye `tabIndex={0}`, `role="button"`, ülke-başına `aria-label` (ör. "Türkiye, 1.6 GB, 4 oturum") ekle; mevcut `enter`/`leave` handler'larını `onFocus`/`onBlur`'a da bağla; ve/veya özet şeridi SVG'nin gerçek erişilebilir metin karşılığı yap (`aria-hidden` SVG + `aria-describedby`).
- **Önerilen komut**: `/impeccable harden`

**[P1] Coğrafi olarak kümelenen ülkelerde balon/etiket çakışması — canlı doğrulandı**
- **Neden önemli**: Çalışan panelde DOM ölçümü: TR balonu (r≈32px, merkez 844,576) ve BG balonu (merkez 817,566, yalnızca 29px uzakta) o kadar örtüşüyor ki BG'nin merkezi TR'nin dairesinin içinde; metin etiketleri birbirinin 4×8px içine düşüyor ve karşılıklı okunamaz hale geliyor. RO ikisinin de içine yerleşiyor. Şu an gösterilen 5 ülkenin 3'ü — uç durum değil, operatörün haritadan tam olarak çözmesi gereken senaryo (bölgesel trafik kümelenmesi, gerçek bir olay anı).
- **Düzeltme**: Etiket yerleşimine çakışma-önleme ekle (merkezler birleşik yarıçap içindeyse radyal ofset) veya kalabalık bölgeler için sıralı listeye düş.
- **Önerilen komut**: `/impeccable layout`

**[P1] Tooltip, imleç viewport köşesindeyken taşıyor — canlı ölçümle doğrulanmış, kod-içi kenar-önleme mantığı çalışmıyor**
- **Neden önemli**: Kod zaten kenar-taşmasını önlemeye çalışıyor (satır 189-191, `window.innerWidth/Height` kontrolleri) ama imleç sağ-alt köşeye zorlandığında (`getBoundingClientRect` ile ölçüldü) tooltip viewport altından taşıyor (`bottom: 834.5` vs `innerHeight: 720`) ve anormal dar/uzun render oluyor (68×131px, 3 satıra bölünmüş metin). Kök neden: `position: fixed; left: <kenar-yakını-x>` + `transform: translateX(-100%)` birlikte kullanıldığında, tarayıcının auto-width shrink-to-fit hesaplaması transform'dan *önce* `(viewport genişliği − left)` ile sınırlanıyor, metin sarılıp kutu boyu patlıyor. Statik taramanın veya enjekte edilen dedektörün hiçbiri hover+köşe-imleç durumunu simüle etmediği için bu ikisinden de gizli — yalnızca doğrudan etkileşimle bulunabildi.
- **Düzeltme**: Sabit genişlikli tooltip (`width` veya `max-width` ata, `translateX` yerine `left`/`right` hesaplamasını doğrudan kutunun gerçek genişliğine göre yap) veya `transform` yerine koşullu `right: ...px` konumlandırmasına geç.
- **Önerilen komut**: `/impeccable harden`

**[P1] Yükleme/boş-durum metni, tasarım sisteminin kendi P0 olarak işaretlediği kontrast hatasını tekrarlıyor**
- **Neden önemli**: `GeoMapCard.tsx:107` ve `:109` `text-slate-600` kullanıyor — DESIGN.md bu class'ı bu zeminlerde ~2.4:1 ölçüyor (AA'nın altında, 4.5:1 gerekli) ve "impeccable critique P0" etiketiyle bilinen sorun olarak işaretliyor. Bu migrasyon *aynı dosyadaki kardeş* Cihazlar kartının boş-durumunda zaten yapılmış (`Overview.tsx:662`, `text-dim-aa`). GeoMapCard'ın yükleme metni ve en iyi boş-durum kopyası (operatöre tam olarak neyin eksik olduğunu söyleyen metin) tasarım sisteminin kendi denetiminin mahkum ettiği bir kontrast oranında.
- **Düzeltme**: Her iki satırda `text-slate-600` → `text-dim-aa`.
- **Önerilen komut**: `/impeccable harden`

**[P2] Kart başlığı ile bileşen gövdesi arasında yinelenen altyazı + rol-dışı 11px/slate-500 metin**
- **Neden önemli**: `Overview.tsx:655` zaten Card'ın başlık-sağ slotunda "GeoIP · uzak uç noktalar" render ediyor; `GeoMapCard.tsx:101-103` birkaç piksel altında neredeyse aynı ikinci bir altyazı ("uzak uç noktalar (NetFlow + agent) · GeoIP ile ülke merkezine") render ediyor. Aynı iki kavram bir kartta iki kez söyleniyor. Ayrıca bu satır, DESIGN.md'nin 4 tipografi rolünden (Label/Caption/Micro/Body) hiçbirine uymayan tek-seferlik bir boyut/renk kombinasyonu (`text-[11px] text-slate-500`) kullanıyor — dedektörün `tiny-text` olarak işaretlediği tam da bu satır.
- **Düzeltme**: Kart-seviyesi `right` slot metnini bu kart için kaldır veya tek bir altyazıda birleştir — bu aynı zamanda dedektörün `tiny-text` bulgusunu da kendiliğinden çözer.
- **Önerilen komut**: `/impeccable distill`

**[P2] Sessiz fetch-hata yutma**
- **Neden önemli**: `catch { /* yoksay */ }` (satır 71) — başarısız bir `/api/v1/geo` isteği (backend down, ağ kesintisi, 401-dışı auth sorunu) hiçbir kullanıcı-görünür sinyal üretmiyor. UI ya eski veride donuyor ya da hiç "Yükleniyor…"dan çıkmıyor. Widget'ın tüm amacının canlı bir olay sırasında uzak-uç kaynağını göstermek olduğu düşünülürse bu en kötü olası hata modu.
- **Düzeltme**: Hata durumu takip et, ayrı bir satır-içi bildirim göster ("veri alınamadı, yeniden deneniyor") — meşru "yapılandırılmamış" boş-durumundan ayrı tut.
- **Önerilen komut**: `/impeccable harden`

**[P3, tartışmalı] `nested-cards` dedektör bulgusu — muhtemelen yanlış pozitif**
- **Not**: Dedektör, zaman-aralığı toggle konteynerini (`GeoMapCard.tsx:88`, `rounded-lg border border-slate-700/80 p-0.5`) Card'ın içinde "kart içinde kart" olarak işaretliyor. Ama bu semantik olarak bir kart değil, bir buton-grubu çerçevesi (segmented control) — bu desen arayüz literatüründe yaygın ve kendi kenarlığı olmadan 3 butonu görsel olarak gruplamanın standart yolu. Düzeltmeye değer mi yoksa `hook-admin.mjs ignore-value` ile kapsamlı biçimde susturulmalı mı, kullanıcı kararına bırakılıyor.

## Persona Kırmızı Bayrakları

**Sam (Erişilebilirlik-bağımlı kullanıcı)** — bu bulgu kümesi için en alakalı:
- Tek bir ülke balonuna Tab ile ulaşamıyor (5 grubun tamamında `tabIndex: -1` doğrulandı, klavye yolu yok).
- `role="img"` SVG'nin her alt-elemanını erişilebilirlik ağacından siliyor — Sam ne kadar trafik verisi olursa olsun tek bir statik cümle alıyor ("Uzak trafiğin ülke bazlı dünya haritası") ve başka hiçbir şey.
- Tek makul yedek (özet şerit) eksik (oturum sayısı yok, 6'da sert kesim, etiketsiz span'lar) ve hiçbir zaman SVG'nin gerçek metin karşılığı olarak bağlanmamış (`aria-describedby` yok) — erişilebilirliği tesadüfi, tasarlanmış değil.
- **Hüküm: bu persona için görevi tamamen engelliyor.**

**Alex (Sabırsız güç kullanıcı)**:
- Zaman-aralığı toggle'ının kendisi Alex için sorunsuz (native `<button>`, tek tık, modal yok) — ama aralık değişimi sessiz (network isteği ve `bg-slate-700` aktif durumu tetikleniyor ama aradaki hiçbir şey "yükleniyor" demiyor), yavaş bir bağlantıda Alex tıklamanın kaydolup kaydolmadığını anlayamıyor ve sabırsızlıkla çift tıklayabiliyor.
- Hiçbir balonun klavye-güdümlü incelemesi yok — dashboard'ın geri kalanındaki standart focus-edilebilir buton/linkleri mouse'suz kullanmaya alışkın Alex, bu tek widget için mouse'a geri dönmek zorunda kalıyor.
- Alex'in "bir bakışta anla" beklentisi tam olarak trafik bölgesel kümelendiğinde kırılıyor (TR/BG/RO çakışması) — Alex'in gerçek triaj sırasında istatistiksel olarak en çok karşılaşacağı senaryo.

## Küçük Gözlemler

- 1/6/24 saat segmented control'de `aria-pressed`/`role="group"` yok (DOM'da doğrulandı: üçünde de `null`) — aktif durum yalnızca görsel (`bg-slate-700` + beyaz metin), AT'ye duyurulmuyor.
- Balon yarıçapının byte hacmini kodladığını açıklayan bir lejant hiçbir yerde yok — ilk kez gören kullanıcı bunu yalnızca hover ederek keşfedebiliyor.
- Riley-tipi stres notu: canlı veri setinde yalnızca 5 ülke vardı; ülke sayısı arttıkça balonları sıkıştıran/dağıtan hiçbir mekanizma yok — n=5'te bulunan çakışma sorunu daha geniş dağıtımda katlanarak büyür, senkron bir 30-ülkelik test verisiyle önceden doğrulanmalı.
- Tooltip kenar-kırpma matematiği (`window.innerWidth - 220`, `window.innerHeight - 90`, satır 189-191) bu oturumun veri setindeki kısa kod/isimler için çalıştı, ama uzun bir ülke ismine (ör. "Birleşik Arap Emirlikleri") karşı test edilmedi — sağ kenara yakınken yine kırpabilir.
- `radius()` 4px'te taban sınırlıyor (satır 83) — iyi, neredeyse-sıfır-byte'lı ülkelerin tamamen kaybolmasını önlüyor.
- Gerçek mobil genişlikte (375px) kartın `overflow-x-auto` konteyneri, herhangi bir kaydırma ipucu görünmeden önce daraltılmayan yan menü tarafından ~85px görünür genişliğe sıkıştırılıyor — bu sayfa-seviyesi bir düzen sorunu (yan menü mobilde daralmıyor), tek başına GeoMapCard'ın suçu değil, ama GeoMapCard'ı mobilde ilk boyamada neredeyse görünmez yapan mekanizma bu.

## Kışkırtıcı Sorular

- "Aynı olayı izleyen iki operatör — biri geniş monitörde, biri RSI sonrası klavye-yalnızca — bugünkü GeoMapCard'da aynı 5-saniyelik anlama hızına sahip mi? Şu an cevap hayır. Özet şeridi gerçek arayüz, harita ise zenginleştirme olsaydı ne gerekirdi?"
- "Kart zaten başlığında bir kez 'GeoIP · uzak uç noktalar' diyor. Üç inç aşağıda neden aynı şeyin bir versiyonunu tekrar söylüyor? Bu kartın gerçekten ihtiyacı olan tek altyazı hangisi?"
- "Bu widget'ın var olma nedeni olan senaryo — komşu ülkelerden yakınsayan trafik — şu an haritanın okunamaz bir bulanıklık olarak render ettiği tek senaryo. Harita kendi başlık senaryosunu bile kaldıramıyorsa, bu kitle için düz sıralı bir tablo haritadan daha güvenli mi?"
