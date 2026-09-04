---
target: frontend/src/components/FortiPanel.tsx
total_score: 13
max_score: 28
na_heuristics: 3,5,7
p0_count: 1
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/FortiPanel.tsx"
target_fingerprint: "sha256:d71f487ce15dbc010ccb71b6a88c3bc7dc650efdbfa699c7aebe7ae77057b8aa"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/FortiPanel.tsx
timestamp: 2026-09-04T20-51-33Z
slug: frontend-src-components-fortipanel-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

**Kanıt kısıtı**: Demo ortamında hiç FortiGate cihazı yok (kullanıcı kararıyla sentetik veri eklenmedi) — bu kritik tamamen kod-okumasına ve statik/kısıtlı tarayıcı kanıtına dayanıyor. Gauge/tablo/sparkline'ın gerçek render halini gösteren canlı ekran görüntüsü yok; bu aşağıdaki her bulguda açıkça not düşülüyor.

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 1 | Hata durumu kalıcı (bkz. P0) + ilk yüklemede iki çelişkili boş-durum mesajı aynı anda |
| 2 | Gerçek Dünya Eşleşmesi | 4 | VDOM, SD-WAN health-check/member, Hit Δ/Bayt Δ — gerçek FortiGate REST API kavramları |
| 3 | Kullanıcı Kontrolü ve Özgürlük | n/a | Salt-okunur görüntüleme, kaçılacak/geri alınacak bir şey yok |
| 4 | Tutarlılık ve Standartlar | 2 | Fixed Meaning Rule iki yerde kırık (Gauge + Sparkline); kardeş tablodaki min-w eksik |
| 5 | Hata Önleme | n/a | Bu bileşende hiç kullanıcı girdisi yok |
| 6 | Tanıma > Hatırlama | 3 | Her tablo kendi etiketini taşıyor, sparkline lejantı hafızaya gerek bırakmıyor |
| 7 | Esneklik ve Verimlilik | n/a | Pasif görüntüleme paneli, hızlandırılacak tekrar eden eylem yok |
| 8 | Estetik ve Minimalist Tasarım | 2 | 4 gauge + sparkline + 3 tam tablo her zaman birlikte, tekini izole etme yolu yok |
| 9 | Hata Tanıma/Kurtarma | 1 | Tek hata mesajı düz dilde ama tekrar-dene yok, hiç kaybolmuyor |
| 10 | Yardım ve Dokümantasyon | 0 | FortiGate-özgü jargon (SD-WAN health-check, VDOM) için bağlamsal yardım yok |
| **Toplam** | | **13/28** (7 sezgi puanlandı, 3 n/a) | **%46 — Poor** |

## Tasarım Özgünlüğü Hükmü

**Güçlü ürün-özgüllüğü.** Her tablo satırındaki VDOM kapsamı, SD-WAN "health-check/member" eşleşmesi, VPN peer/uptime formatlaması (`fmtUptime`, "3g 4sa" Türkçe stili), "Hit Δ / Bayt Δ" kolon çerçevelemesi (yaşam-boyu toplam değil *değişim hızı*) — gerçek FortiGate REST API kavramları, başka bir vendor'a (Cisco/Palo Alto) değiştirilmeden taşınamaz. Vendor-orange kullanımı (satır 167-169, "fortigate rest api · canlı veri") DESIGN.md'nin kendi örneğiyle birebir örtüşüyor — tam da bu doğru kullanımın hemen yanında iki yerde (Gauge, Sparkline) aynı kural kırılıyor.

**Deterministik tarama**: CLI temiz (`[]`, exit 0) — ama bu dosyanın gerçek "temiz" olduğu anlamına gelmiyor: dar regex kuralları (`gray-on-color` tek-satır kapsamlı, `ai-color-palette` yalnızca başlık-boyutlu satırlar) yapısal olarak bu dosyanın kalıplarını yakalayamıyor; kontrast/tiny-text/nested-cards gibi hesaplanmış-DOM gerektiren kategoriler zaten regex motorunda hiç yok. **Dolaylı kanıt**: aynı sayfada (DevicesCard'ın SNMP arayüz tablosu, FortiPanel'le birebir aynı stil dili — sub-11px mono hücreler, cyan-neon vurgular) tarayıcı-içi enjeksiyon 336 bulgu döndürdü (202 tiny-text, 54 undersized-text, 76 ai-color-palette). FortiPanel'in kendisi render edilemediği için bu **doğrudan kanıt değil, güçlü bir emsal** — aynı kalıpları taşıyan bu dosyanın da render edilseydi benzer bulgular üretmesi muhtemel, ama ölçülmedi.

**Tablo min-width denetimi** (kaynak-kod, iki değerlendirme bağımsız doğruladı): VPN(9 kolon, satır 200-201), SD-WAN(7 kolon, 235-236), Policy(6 kolon, 266-267) — üçü de `overflow-x-auto` sarmalayıcıya sahip ama **hiçbirinde `min-w-[...]` yok**. `DevicesCard.tsx`'in aynı genişlikte render olan kardeş SNMP tablosu bu turun başında tam bu nedenle `min-w-[520px]` aldı — FortiPanel'e henüz uygulanmadı.

**SVG erişilebilirlik denetimi**: sparkline'ın `<svg>`'i (satır 100) `role`/`aria-label` taşımıyor — kod tabanındaki **3 kardeş** elle-SVG bileşeninin (`ThroughputChart.tsx`, `GeoMapCard.tsx`, `TrafficFlowDiagram.tsx`) hepsi bu özniteliği taşıyor. Bu bir tutarlılık boşluğu, izole bir eksiklik değil.

**Renk-anlam analizi** (7 sabit renk / Fixed Meaning Rule): `statusColor()` (54-59) ve vendor-orange kullanımı **temiz** — DESIGN.md'nin tanımlarıyla birebir. İki ihlal bulundu:
- `Gauge` (satır 111): "normal" durum (≤60%) cyan kullanıyor — DESIGN.md sağlıklı/normal için emerald tanımlıyor, cyan rx-trafik/marka rengi. Aynı dosyadaki `statusColor()` zaten "up" için emerald kullanıyor — **dosya kendi içinde tutarsız**.
- `ResourceSparkline` (satır 85-87): cpu=cyan, bellek=amber (DESIGN.md'de amber "uyarı eşiği" sabit — sağlıklı %20 bellek okuması bile "dikkat" renginde çiziliyor), disk=violet TEK BAŞINA (DESIGN.md'nin violet'i her zaman cyan ile eşleşmesi gereken tx-trafik rengi olarak tanımladığı kuralı kırıyor — bu oturumda `AlertsCard`/`ComplianceCard`'da zaten düzeltilen aynı ihlal kalıbı).

Ayrıca (küçük, ikincil): `amber-500` (#f59e0b) DESIGN.md'nin belgelediği `amber-400` (#fbbf24) yerine kullanılmış — ton kayması; policy "deny" aksiyonu için `rose-400/80` kullanımı (satır 282) tartışmalı — rose "kritik alarm" anlamına sabit ama bir politika reddi rutin/beklenen bir sonuç, alarm değil (yumuşak bir esneme, kesin ihlal değil).

## Öne Çıkanlar
1. Domain-özgü terminoloji ve formatlamalar (VDOM, SD-WAN, Hit Δ/Bayt Δ, Türkçe göreli-zaman) — gerçek FortiGate modellemesi.
2. Durum hiçbir yerde yalnızca renkle taşınmıyor — `statusColor()` metni renkle birlikte her zaman yazıyor ("up"/"down"/"connecting" kelimeleri hep görünür).
3. Vendor-orange kullanımı DESIGN.md'nin kendi örneğiyle birebir doğru.

## Öncelikli Sorunlar

**[P0] Hata durumu kalıcı — sonraki tüm başarılı verileri gizliyor**
- **Neden önemli**: `load()` (132-148) `catch` bloğunda `error`'ı set ediyor ama başarılı bir sonraki pollda hiç temizlemiyor. Render `if (error) return <p>{error}</p>` (satır 156) olduğundan, 4 paralel fetch'ten (resources/vpn/sdwan/policies) **herhangi birinin** herhangi bir 60-saniyelik pollda **bir kez** başarısız olması, gauge'ları/sparkline'ı/3 tabloyu kalıcı olarak tek bir hata satırıyla değiştiriyor — sonraki her poll başarılı olsa bile. `DevicesCard.tsx`'in `showIfaces` mantığı aynı cihaz için paneli hiç yeniden mount etmediğinden (satır 103 civarı), sayfayı yenilemek veya başka bir cihaz satırını genişletmek dışında geri dönüş yolu yok. Bu panel tam olarak PRODUCT.md'nin "olay anında derinlemesine inceleme" dediği kullanım senaryosu için var — geçici bir ağ sıçraması yüzünden tam da bir olay sırasında kalıcı olarak boşalması ciddi bir güven kaybı.
- **Düzeltme**: Başarılı `load()` çalışmasında `error`'ı temizle; "hiç yüklenmedi" / "son poll başarısız" / "eski ama geçerli veri var" durumlarını ayrı tut, son iyi render'ı hata sırasında da göstermeye devam et (küçük satır-içi "yenileme başarısız, yeniden deneniyor" ipucuyla).
- **Önerilen komut**: `/impeccable harden`

**[P1] İlk yüklemede iki çelişkili boş-durum mesajı aynı anda + ikisi de yasak renkte**
- **Neden önemli**: Mount anında `last` null ve 4 dizi de boş olduğundan hem "kaynak verisi bekleniyor" (185) hem de alttaki genel "FortiGate verisi henüz yok" (294-298) **aynı anda** render oluyor (kod izlenerek doğrulandı). İkisi de `text-slate-600` kullanıyor — DESIGN.md'nin Micro Scope Rule'ü boş-durum mesajlarının asla `slate-600`/`700` olmaması gerektiğini açıkça belirtiyor (~2.4:1, AA altı), tam da bu metnin **okunması** gerektiği için.
- **Düzeltme**: Ayrı bir `loading` durumu ekle, tek bir mesaj göster, ikisini de `text-dim-aa`'ya taşı. Dosyada ek olarak 18 `text-slate-500` örneği var (aynı "Dim" ailesinin henüz taşınmamış parçası) — kapsamlı bir geçiş olarak bunlar da `text-dim-aa`'ya taşınmalı.
- **Önerilen komut**: `/impeccable polish`

**[P1] Fixed Meaning Rule — Gauge ve Sparkline'da kırık**
- **Neden önemli**: Yukarıda detaylandırıldı — Gauge'ın normal-durum cyan'ı (dosyanın kendi `statusColor()`'ıyla tutarsız) ve Sparkline'ın amber/violet kategorik seri renkleri, bu oturumda AlertsCard/ComplianceCard'da zaten düzeltilen aynı sınıf ihlal.
- **Düzeltme**: Gauge normal-durum → emerald (dosyanın kendi `statusColor()` emsaliyle hizala). Sparkline'ın 3 seri rengi için DESIGN.md'de "nötr kategorik seri" diye bir kavram yok — kullanıcı kararı gerekiyor (aşağıda soru olarak sorulacak).
- **Önerilen komut**: `/impeccable colorize`

**[P2] 3 tablonun hiçbirinde `min-w` yok**
- **Neden önemli**: VPN(9 kolon)/SD-WAN(7 kolon)/Policy(6 kolon) — `overflow-x-auto` var ama tabloda `min-w-[...]` yok, `DevicesCard.tsx`'in bu turda düzeltilen kardeş tablosunun aksine. Min-width olmadan `table-auto` düzeni kaydırma yerine kolonları sıkıştırıyor.
- **Düzeltme**: Her tabloya kolon sayısına uygun `min-w-[...]` ekle (VPN en geniş, 9 kolon).
- **Önerilen komut**: `/impeccable layout`

**[P2] Sparkline SVG'sinin erişilebilir ismi yok**
- **Neden önemli**: Kod tabanındaki 3 kardeş elle-SVG bileşeninin (`ThroughputChart.tsx`, `GeoMapCard.tsx`, `TrafficFlowDiagram.tsx`) hepsinde `role`/`aria-label` var, bu SVG'de yok — izole bir eksiklik değil, kurulu bir konvansiyondan sapma.
- **Düzeltme**: `role="img" aria-label="son N dakika CPU/bellek/disk trendi"` ekle.
- **Önerilen komut**: `/impeccable harden`

## Persona Kırmızı Bayrakları

**Alex**: P0 hatasına bir vardiya ortasında çarpıp hiçbir görünür tekrar-dene eylemi bulamıyor; `DevicesCard.tsx`'in toggle mantığı yüzünden başka bir cihaz satırı açmak veya sayfayı yenilemek dışında zorla yeniden-mount yolu yok. 3 üst üste yığılmış tablo 10-11px metinle, sıralama/filtre/daraltma olmadan — "hangi VPN tüneli down" aramak her tablonun her satırını sırayla okumayı gerektiriyor.

**Sam**: Sparkline (satır 100) ekran okuyucuya lejant metni dışında sessiz — gerçek trend şekli aktarılmıyor. İki boş-durum mesajı de (185, 295) — ilk fetch süresince düşük görüşlü bir kullanıcının gördüğü tek içerik — DESIGN.md'nin kendi denetiminin AA-altı ölçtüğü tonda.

## Küçük Gözlemler
- `Gauge` bar genişliğini/renk-eşiğini 0-100'e kırpıyor (110-111,119) ama etikette ham `pct`'i yazıyor (116) — API 105% veya -5% dönerse sayı ile bar görsel olarak çelişebilir.
- İç alt-panel zeminleri `bg-slate-900/60` (90,113,179) — projenin belgelediği Panel token'ı `/70` — muhtemelen kopyala-yapıştır kayması, düşük görsel etki.
- SD-WAN satırlarında VPN satırlarının aksine (`fmtAgo`, 222) satır-bazlı "ne zaman güncellendi" yok — 30dk pencerede bir okumanın 2dk mı 28dk mı eski olduğu belirsiz.
- Policy tablosu sessizce 15 satırda kesiliyor (`limit=15`, 138), "15/N gösteriliyor" göstergesi yok.
- Amber ton kayması: sparkline `#f59e0b` (amber-500) kullanıyor, DESIGN.md `#fbbf24` (amber-400) belgeliyor.

## Kışkırtıcı Sorular
- Tek bir fetch olay-ortasında zaman aşımına uğrarsa, panel gerçekten sonsuza kadar boşalmalı mı, yoksa eski-ama-yakın veri "yenileme başarısız, yeniden deneniyor" ipucuyla görünür kalmalı mı?
- Üç tamamen bağımsız tablo her zaman birlikte render olurken, Uyumluluk'un kendi yoğunluk sorununu ayrı rotalara bölmesine benzer bir sekmeli alt-görünüm bir operatörün doğrudan aradığı tabloya atlamasını sağlar mı?
- Gauge ve Sparkline artık ikisi de uyarı/trafik renklerini çıplak kategorik etiket olarak yeniden kullanıyorken, DESIGN.md'nin sözleşmesi açık bir "nötr kategorik seri" kuralıyla genişletilmeli mi — böylece bir sonraki bileşen alışkanlıkla cyan/amber/violet'e uzanmasın?
