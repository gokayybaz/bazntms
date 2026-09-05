---
target: frontend/src/pages/DeviceDetailPage.tsx
total_score: 18
max_score: 32
na_heuristics: 5,7
p0_count: 0
p1_count: 3
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/pages/DeviceDetailPage.tsx"
target_fingerprint: "sha256:ea834286c098de9d2e5b4eb4d05f4604fec819215bc2a6c2e8a9c01e36f3473a"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/pages/DeviceDetailPage.tsx
timestamp: 2026-09-05T03-29-20Z
slug: frontend-src-pages-devicedetailpage-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent — B'nin ilk 3 denemesi ortam/altyapı kaynaklı kesintilerle başarısız oldu; CLI taraması + kod envanteri bu yüzden ana oturumda doğrudan çalıştırıldı, B'nin son [4.] denemesi başarıyla tamamlanıp yalnızca tarayıcı-kanıtı katkısını üstlendi — findings aşağıda tek rapor olarak sentezlenmiştir, ayrı ayrı degrade edilmemiştir)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 2 | Sessiz 401 yutma (satır 95,121,145) donuk veriyi hiç işaretlemeden bırakıyor; NetFlow/syslog sayıları gerçekte gösterilenle uyuşmuyor |
| 2 | Gerçek Dünya Eşleşmesi | 3 | Türkçe boyunca, jargon (discards) satır-içi ipucuyla açıklanıyor |
| 3 | Kullanıcı Kontrolü ve Özgürlük | 2 | Tek çıkış "← Cihazlar" linki; 37 arayüzü veya 200 syslog satırını kapsamlama/filtreleme yolu yok |
| 4 | Tutarlılık ve Standartlar | 1 | Syslog rozeti (satır 395) 8 seviyenin tamamı için tek düz stil kullanıyor — canlı doğrulandı: 200 satırda 3 farklı gerçek seviye (info/error/notice) birebir aynı `text-slate-400 ring-slate-700` ile basılıyor; kardeş `SyslogCard.tsx`'in doğru `SEV_STYLES` haritası var |
| 5 | Hata Önleme | n/a | Salt-okunur görüntüleme, girdi/yıkıcı-eylem yüzeyi yok |
| 6 | Tanıma > Hatırlama | 3 | Hiçbir şey gizli değil, kolon başlıkları + satır-içi ipucu + etiketli alanlar |
| 7 | Esneklik ve Verimlilik | n/a | Tasarım gereği salt-okunur izleme sayfası |
| 8 | Estetik ve Minimalist Tasarım | 3 | Triad/Card kalıbı temiz izleniyor |
| 9 | Hata Tanıma/Kurtarma | 2 | `last_error` net bir rose banner'da (gerçek güçlü yön) ama tekrar-dene eylemi yok |
| 10 | Yardım ve Dokümantasyon | 2 | Tek yerinde bağlamsal ipucu (discards), başka yok |
| **Toplam** | | **18/32** (8 sezgi, 2 n/a) | **%56 — Acceptable** |

## Tasarım Özgünlüğü Hükmü

**Ürüne dayalı.** Sağlık/vendor rozeti kombinasyonu, `ifInDiscards/ifOutDiscards` ipucu (satır 309, gerçek bir SNMP sayaç ayrımını açıklıyor), NetFlow-exporter-IP-yeniden-yazma yorumu (162-164) ve FortiGate-koşullu özet hücreleri — hepsi bu ürünün gerçek veri modeline özgü, jenerik bir dashboard'a değiştirilmeden taşınamaz.

**Deterministik tarama**: CLI temiz (`[]`, exit 0). **Kaynak-kod envanteri** (ana oturum tarafından doğrulandı, B tarafından canlı DOM'da teyit edildi): 23 adet `text-slate-500`/`600` kullanımı — canlı DOM'da (nav/sidebar hariç) bu class'ları taşıyan **631 element** sayıldı (37 arayüz satırı × çoklu dim-hücre çarpımından). `<th>` scope denetimi: 13 `<th>` (8 Arayüzler + 5 NetFlow), **hiçbirinde `scope="col"` yok** — canlı DOM'da teyit edildi.

**Syslog rozeti** (satır 395): kod-seviyesinde tüm 8 seviye için aynı stil kullandığı doğrulandı; **canlı veride de teyit edildi** — 200 render edilen satırda 3 farklı gerçek severity (info/error/notice) birebir aynı nötr griyle basılıyor. Kardeş `SyslogCard.tsx` (satır 13-22) doğru bir `SEV_STYLES` haritası zaten taşıyor (0-1=güçlü rose, 2=rose, 3-4=amber, 5=sky, 6-7=slate) — bu dosya o haritayı hiç kullanmıyor, kendi (yanlış) düz stilini elle kopyalamış.

**NetFlow sayaç tutarsızlığı** (satır 349 vs 365): canlı ortamda `deviceFlows.length` = 20, `.slice(0,100)` sınırı şu an tetiklenmiyor — **kod-seviyesinde gerçek, canlı olarak henüz tetiklenmemiş bir risk**. Syslog sayacı ise canlı olarak tam tavanda (200/200, `limit=200` API çağrısına dayanıyor) — cihaza-özgü filtreleme istemci tarafında çalışıyor ama paylaşılan 200-satırlık pencere, yoğun trafikli farklı bir cihaz bu cihazın eski olaylarını sessizce dışarı itebilir.

**Mobil (375px, B'nin canlı ölçümü)**: özet şerit doğru 2 sütuna düşüyor; her iki tablo (`Arayüzler` 850px, `NetFlow` 457px) kendi `overflow-x-auto` sarmalayıcısı içinde doğru taşıyor (sayfa gövdesinde yatay kaydırma yok) — ama hiçbir kaydırma ipucu/gölge olmadığından mobilde yalnızca "Arayüz" sütunu görünür kalıyor, diğer 7 sütun keşfedilebilir değil. Kırık değil, düşük keşfedilebilirlik.

## Öne Çıkanlar
1. Satır-içi jargon kurtarması (309) — `ifInDiscards/ifOutDiscards` ipucu tek hoverla açıklıyor.
2. Koşullu açığa çıkarma doğru yapılmış — FortiGate hücreleri/paneli ve hata banner'ı yalnızca ilgili koşul doğruyken görünüyor.
3. Üç tablonun/listenin hepsinde ayrı, dostane Türkçe boş-durum metni var.

## Öncelikli Sorunlar

**[P1] Sağlık rozeti hiç poll edilmemiş cihazları "sorunlu" olarak yanlış etiketliyor**
- **Neden önemli**: Satır 205 — `device.enabled && device.last_poll > 0 && !device.last_error`. Yeni eklenmiş, etkin, hatasız bir cihaz (`last_poll===0`) devre-dışı bırakılmış veya gerçekten arızalı bir cihazla aynı gri "sorunlu" rozetini alıyor. Üç ayrı durumu (devre-dışı / ilk-poll'u-bekliyor / gerçekten arızalı) tek bir etikete sıkıştırıyor. Aynı formül `Overview.tsx:265`'te de birebir tekrarlanıyor — tek seferlik bir yazım hatası değil, tekrar eden bir kavramsal yanlış anlama.
- **Düzeltme**: Üçüncü bir "bekliyor" durumu ekle (`last_poll===0` için), "sorunlu"yu yalnızca `!enabled || last_error` için ayır.
- **Önerilen komut**: `/impeccable clarify`

**[P1] Syslog önem-seviyesi görsel olarak düz — canlı veride teyit edildi**
- **Neden önemli**: Kardeş `SyslogCard.tsx` doğru renk-kodlamayı zaten yapıyor (DESIGN.md'nin "kırmızı görünce operatör düşünmeden 'kritik' bilir" ilkesiyle tam uyumlu); bu sayfa aynı bilgiyi tek düz gri ile basıyor. Canlı veride "error" seviyesi bile "info" ile ayırt edilemiyor.
- **Düzeltme**: `SEV_NAMES`/`SEV_STYLES`'ı `SyslogCard.tsx`'ten paylaşılan bir `lib/` dosyasına çıkar (aynı `alertKinds.ts` deseni — iki dosyada elle kopyalanan bir sözleşme zaten bir kez sessizce sürüklenmişti), ikisi de oradan import etsin.
- **Önerilen komut**: `/impeccable harden`

**[P1] Gösterilen sayılar gerçekte render edilenle uyuşmuyor (biri gizli, biri sınırlı)**
- **Neden önemli**: NetFlow başlığı `deviceFlows.length`'i gösteriyor (349) ama tablo yalnızca ilk 100'ü render ediyor (365, `slice(0,100)`) — bugün 20 akışla tetiklenmiyor ama 15dk'da 100'ü aşan bir cihazda sessizce sayı-satır uyuşmazlığı oluşacak. Syslog ise paylaşılan, cihaza-özgü olmayan bir 200-satır API tavanına dayanıyor — yoğun bir cihaz bu cihazın eski olaylarını sessizce dışarı itebilir, arayüzde hiçbir işaret yok.
- **Düzeltme**: NetFlow'da kesme olduğunda "ilk 100 / N" ifadesi göster.
- **Önerilen komut**: `/impeccable clarify`

**[P2] Sessiz 401 — hiçbir donukluk sinyali yok**
- **Neden önemli**: 4 fetch effect'inin hepsi (95,121,145) `if (res.status === 401) return` yapıyor, sayfa-seviyesi "son güncelleme" göstergesi yok. Bir olay sırasında kullanılan bu sayfada, ölü bir oturum operatörü sonsuza kadar donmuş bir "sağlıklı" rozetine güvendiriyor.
- **Düzeltme**: 401'de görünür bir "oturum sona ermiş olabilir" banner'ı göster.
- **Önerilen komut**: `/impeccable harden`

**[P2] `text-slate-500`/`600` kontrast borcu, dosyada tamamen taşınmamış**
- **Neden önemli**: 23 kaynak-kod kullanımı, canlı DOM'da 631 element — sıfırı `text-dim-aa` kullanmıyor. Aynı sayfada gömülü `FortiPanel` (satır 289) bu turda zaten `text-dim-aa`'ya taşındı — bir FortiGate cihazında bu sayfa tek bir kart sınırı içinde iki farklı "dim" tonu gösterecek, biri AA-uyumlu biri değil.
- **Düzeltme**: Dosya genelinde `text-dim-aa`'ya taşı.
- **Önerilen komut**: `/impeccable polish`

**[P2] `<th>` elemanlarının hiçbirinde `scope="col"` yok**
- **Neden önemli**: Canlı DOM'da teyit edildi — 13 `<th>` (2 tablo), hiçbiri `scope`'suz. Ekran okuyucu kullanıcısı sütun-satır ilişkisini tablo yapısından çıkaramıyor.
- **Düzeltme**: Her `<th>`'ye `scope="col"` ekle.
- **Önerilen komut**: `/impeccable harden`

## Persona Kırmızı Bayrakları

**Riley**: Syslog sayısının paylaşılan 200 tavanında sabit kaldığını (gerçek hacimden bağımsız) fark edip bunu bug/yalan olarak işaretler. Hiç poll edilmemiş bir cihazda yanlış "sorunlu" okuması alır. 401'i zorladıktan sonra sayfanın son "sağlıklı" durumu sonsuza kadar göstermeye devam ettiğini bulur.

**Sam**: Sağlık durumu metin+nokta (renk-yalnızca değil, olumlu). 3 tablonun hiçbirinde `<th scope>` yok. Syslog rozetleri her seviye için aynı nötr gri — düşük görüşte "emergency" ile "debug" gerçekten ayırt edilemiyor.

## Küçük Gözlemler
- Özet şerit ızgarası tek satırlık sarkan bir kart bırakıyor (5 hücre SNMP, 7 FortiGate — `grid-cols-2 lg:grid-cols-4` ile tek sayı).
- 3 bağımsız `useEffect` fetch'i "henüz çekilmedi" ile "gerçekten boş" ayrımı yapmıyor — ilk poll turunda kısa bir "veri yok" titremesi olabilir.
- Yerel `SEV_NAMES` dizisi (satır 69) `SyslogCard.tsx`'inkinin birebir kopyası — sürüklenmenin yapısal kök nedeni.
- Mobilde (375px) tablolar taşmayı doğru kapsıyor ama kaydırma ipucu/gölge yok — kırık değil, düşük keşfedilebilirlik.

## Kışkırtıcı Sorular
- `SyslogCard.tsx` önem-renk eşlemesini zaten doğru çözmüşken, bu sayfanın kendi syslog listesi neden onu import etmiyor?
- "Sağlıklı/sorunlu" gerçekten tek bit mi olmalı, yoksa bir olayı araştıran operatör devre-dışı / hiç-poll-edilmedi / poll-başarısız arasındaki farkı bir bakışta görmeyi hak etmiyor mu?
- Gerçek bir cihazda NetFlow hacmi 15dk'da 100 satırı aşsaydı, bu kritik onu işaretlemeden önce başlık sayısıyla tablonun sessizce ayrıştığını kimse fark eder miydi?
