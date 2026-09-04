---
name: bazNTMS Dashboard
description: Ağ trafiği izleme paneli — koyu, veri-yoğun, ağ operasyon merkezi estetiği
colors:
  bg-base: "#020617"
  panel: "rgba(15, 23, 42, 0.7)"
  border: "#1e293b"
  ink-primary: "#e2e8f0"
  ink-secondary: "#94a3b8"
  dim: "#64748b"
  dim-aa: "#7d8aa0"
  rx-cyan: "#22d3ee"
  tx-violet: "#a78bfa"
  healthy-emerald: "#34d399"
  warn-amber: "#fbbf24"
  critical-rose: "#fb7185"
  proc-sky: "#38bdf8"
  vendor-orange: "#fb923c"
typography:
  data:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.2
  label:
    fontFamily: "inherit"
    fontSize: "0.6875rem"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "0.1em"
  caption:
    fontFamily: "inherit"
    fontSize: "0.65625rem"
    fontWeight: 400
    lineHeight: 1.4
  micro:
    fontFamily: "inherit"
    fontSize: "0.5625rem"
    fontWeight: 400
    lineHeight: 1.3
rounded:
  sm: "2px"
  md: "6px"
  lg: "8px"
  full: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
components:
  card:
    backgroundColor: "{colors.panel}"
    rounded: "{rounded.md}"
    padding: "16px"
  card-header-title:
    textColor: "{colors.ink-secondary}"
    typography: "{typography.label}"
  badge-cyan:
    backgroundColor: "{colors.rx-cyan}"
    textColor: "{colors.rx-cyan}"
    rounded: "{rounded.full}"
    padding: "2px 8px"
---

# Design System: bazNTMS Dashboard

## Overview

**Creative North Star: "Ağ Operasyon Merkezi" (The Network Operations Deck)**

Bir NOC/güvenlik operasyon odasının duvarındaki panel gibi düşünülmüş: koyu
zemin (parlamayı önler, uzun vardiyada göz yormaz), yoğun teknik veri
(mono font ile sayılar), ve **anlamı taşıyan** renkler — dekoratif değil,
her rengin sabit bir okunuşu var (kırmızı görünce operatör düşünmeden
"kritik" bilir). Kimlik sistemli bir tema dosyasında değil, kod genelinde
tekrarlanan bir *sözleşmede* yaşıyor — bu döküman o sözleşmeyi ilk kez
tek yerde yazıya döküyor.

Sistem düz (flat) — gölge yalnızca sayfadan kopup yüzen 2 istisnai yüzeyde
(giriş kartı, coğrafi harita kartı) var; geri kalan her şey ince bir
kenarlıkla ayrışır, katman derinliği ima etmez. Yoğunluk yüksek (bir ekranda
10+ metrik olağan) ama her metrik aynı 3'lü kalıptan geçtiği için tarama
hızlı kalır.

**Key Characteristics:**
- Koyu zemin zorunlu, açık tema yok
- 7 renkli sabit anlam sözleşmesi (dekoratif renk kullanımı yok)
- Sayısal/teknik veri her zaman mono font, UI metni her zaman sans-serif
- Düz (flat) tasarım — gölge istisna, kural değil
- Sıfır harici ikon/animasyon/chart kütüphanesi — her görsel öğe elle SVG veya Tailwind

## Colors

Palet karakteri: koyu slate zemin üzerine yerleşmiş, her biri tek bir
operasyonel anlamı sabitleyen 7 doygun vurgu rengi.

### Primary
- **Rx Cyan** (`#22d3ee`, cyan-400): indirilen/rx trafik, birincil marka
  vurgusu (linkler, aktif nav, odak durumları). En sık görülen vurgu rengi.

### Secondary
- **Tx Violet** (`#a78bfa`, violet-400): gönderilen/tx trafik. Her zaman
  Rx Cyan ile eşleştirilmiş görünür (indirilen/gönderilen çifti,
  ör. ThroughputChart lejantı) — asla tek başına birincil vurgu değildir.

### Tertiary
- **Healthy Emerald** (`#34d399`, emerald-400): online/sağlıklı durum,
  onay, pozitif değişim.
- **Warn Amber** (`#fbbf24`, amber-400): uyarı eşiği, dikkat gerektiren
  durum, bant genişliği/paket hızı zirvesi.
- **Critical Rose** (`#fb7185`, rose-400): kritik alarm, hata, kesinti.
- **Proc Sky** (`#38bdf8`, sky-400): süreç-tipi (yeni süreç) uyarı sınıfı;
  internet/ağ ucu vurguları.
- **Vendor Orange** (`#fb923c`, orange-400): üçüncü taraf entegrasyon
  rozeti (FortiGate, REST API kaynağı) — "bu veri harici bir sistemden
  geliyor" sinyali.

### Neutral
- **Base** (`#020617`, slate-950): sayfa zemini, `html { color-scheme: dark }`
  ile zorunlu.
- **Panel** (`rgba(15,23,42,.7)`, slate-900/70): kart/konteyner zemini.
- **Border** (`#1e293b`, slate-800): kart kenarlığı, ayraç çizgileri.
- **Ink Primary** (`#e2e8f0`, slate-200): birincil okunabilir metin.
- **Ink Secondary** (`#94a3b8`, slate-400): etiket/ikincil metin.
- **Dim** (`#64748b`, slate-500/600): açıklama, altyazı, en düşük öncelikli metin.
  **Bilinen sorun**: `slate-600`/`slate-700` bu zeminlerde WCAG AA'yı (4.5:1)
  geçmiyor — ölçülen: slate-600 ~2.4:1, slate-700 ~1.8:1 (impeccable
  critique P0). Legacy — yeni/dokunulan koddan çıkarılıyor.
- **Dim AA** (`#7d8aa0`, `--color-dim-aa` / `text-dim-aa`): slate-500/600/700'ün
  AA-uyumlu yerine geçeni — panel zemininde 5.2:1, base zemininde 5.8:1.
  Docs-site landing page'in `--dim` token'ıyla aynı değer (kasıtlı, iki
  yüzey arasında akrabalık). `Overview.tsx`'te tüm eski `slate-600`/`700`
  kullanımı buna taşındı; diğer dosyalar henüz taşınmadı — dosya bir
  refinement turuna girdiğinde bu migrasyon o turun bir parçası olmalı.

### Named Rules
**The Fixed Meaning Rule.** Her vurgu rengi tam olarak bir operasyonel
anlama sabitlenmiştir (cyan=rx, violet=tx, emerald=sağlıklı, amber=uyarı,
rose=kritik, sky=proc-alarm, orange=vendor). Bir rengi başka bir bağlamda
kullanmak (ör. bir butonu sırf "güzel dursun" diye rose yapmak) sözleşmeyi
kırar — operatör artık renkten anlam çıkaramaz. Yeni bir görsel kategori
gerektiğinde önce bu 7 rengin yeniden kullanılıp kullanılamayacağı
düşünülür, yeni renk icadı son çare.

## Typography

**Data Font:** JetBrains Mono (`--font-mono` CSS custom property; fallback
`ui-monospace, SFMono-Regular, Menlo, monospace`)
**UI Font:** sistem sans-serif (Tailwind varsayılanı, ayrı bir font-family
tanımlanmamış)

**Character:** Mono font teknik/ölçülebilir olanı (sayılar, IP'ler, süreç
adları, protokol etiketleri) işaretler; sans-serif okunan/anlaşılan olanı
(başlıklar, açıklamalar, UI metni) taşır. İkisi karışmaz — bir bileşende
hem mono hem sans görülüyorsa bu her zaman veri/açıklama ayrımını yansıtır.

### Hierarchy
- **Data** (700, 1.5rem/24px, mono): stat kartı birincil değerleri
  (ör. "124 / 140", "3.2K pps").
- **Label** (600, 11px, sans, `tracking-widest uppercase`): kart başlıkları,
  stat etiketleri — her zaman büyük harf ve geniş harf aralığı.
- **Caption** (400, ~10.5px, sans, dim renk): stat açıklaması, ikincil bilgi
  ("filo toplamı · gelen · canlı" gibi).
- **Micro** (400, 9px [8.5-10px aralığı], sans/mono, dim-aa): yalnızca
  taranabilir, kendi başına birincil anlam taşımayan meta-veri — zaman
  damgası, pid, birim eki, kısa tür rozeti ("flow"/"agent"/"syslog"). Kod
  genelinde zaten yoğun kullanılıyordu (impeccable dedektörü tek başına
  Overview.tsx'te 830+ alt-eşik metin örneği buldu) ama DESIGN.md'de hiç
  tanımlı değildi — bu satır o boşluğu dolduruyor, kodu bu boyuta indirmiyor.
- **Body** (400, 13-14px, sans, ink-secondary): kart içi açıklama metni.

### Named Rules
**The Triad Rule.** Her metrik/stat kartı üç parçalı aynı kalıptan geçer:
uppercase tracked **Label** → bold mono **Data** → dim **Caption**. Bu üçlü
kod genelinde onlarca yerde elle kopyalanmış (merkezi bir bileşen yok) ama
asla kırılmamış — yeni bir stat eklerken kalıp bozulmaz.

**The Micro Scope Rule.** 11px altı yalnızca *taranabilir* meta-veri için —
zaman damgası, pid, birim eki, kısa rozet. Bir kullanıcının **okuması**
gereken herhangi bir metin (boş-durum mesajı, hata/uyarı metni, birincil
değer) asla Micro'ya düşmez, en az Caption (10.5px) kalır. Micro metin her
zaman `dim-aa` (veya daha açık) renkte olur — asla `slate-600`/`700`
(bkz. Colors → Dim AA), çünkü küçük punto + düşük kontrast birlikte AA'yı
katmerli şekilde ihlal eder.

## Layout

Kart-bazlı grid — sayfa içeriği `Card` konteynerlerine bölünür, her biri
kendi `p-4` iç boşluğuyla. Yoğunluk yüksek: bir ekranda genellikle 5+ kart
+ 8+ stat tek arada görünür (NOC panelinin doğası). Responsive davranış
sayfa bazlı değişir; SVG diyagramları (TrafficFlowDiagram, TopologyCard)
dar ekranlarda küçülüp okunamaz olmak yerine `overflow-x-auto` +
`min-width` ile yatay kaydırmaya geçer.

## Elevation & Depth

Sistem büyük ölçüde **düz** — derinlik gölgeyle değil, zemin
katmanlarıyla (base → panel → border) ifade edilir. `box-shadow`
codebase genelinde yalnızca 2 dosyada kullanılıyor: `LoginScreen.tsx`
(giriş kartı, sayfadan kopup yüzen tek istisnai yüzey) ve `GeoMapCard.tsx`
(coğrafi harita kartı). Geri kalan tüm kartlar `border + bg` ile ayrışır,
gölgesiz.

### Shadow Vocabulary
- **Floating surface** (`shadow-2xl shadow-black/40`): sayfanın geri
  kalanından tamamen kopmuş, kendi başına duran yüzeyler (giriş ekranı).
- **Lifted card** (`shadow-lg shadow-black/40`): istisnai olarak öne
  çıkarılmak istenen tek bir kart (coğrafi harita).

### Named Rules
**The Flat-By-Default Rule.** Kartlar varsayılan olarak gölgesizdir —
`border-slate-800` + `bg-slate-900/70` derinliği taşır. Gölge yalnızca bir
yüzey gerçekten sayfa akışından kopup "yüzüyor" hissi vermesi gerektiğinde
eklenir (istisna, kural değil).

## Shapes

Köşe yarıçapı bağlama göre kademelenir, tek bir sabit değer değil:
`rounded-md` (6px) genel kart/konteyner varsayılanı; `rounded-lg` (8px)
daha büyük/vurgulu yüzeylerde; `rounded-full` rozet, nokta-gösterge ve
pill etiketlerde; `rounded-sm` (2px) yalnızca çok küçük dekoratif
elemanlarda (ör. lejant kare işaretleri). Kenarlıklar her zaman ince
(1-1.5px), `slate-800` veya ilgili vurgu renginin düşük-opaklıklı hali.

## Components

### Cards / Containers
- **Corner Style:** `rounded-md` (6px)
- **Background:** `bg-slate-900/70` (Panel)
- **Border:** `border border-slate-800`, başlık varsa altında ayrı
  `border-b border-slate-800`
- **Shadow Strategy:** yok (bkz. Elevation & Depth — flat-by-default)
- **Internal Padding:** `p-4`; başlık şeridi `px-4 py-2.5`
- Kaynak: `frontend/src/components/Card.tsx` — projedeki tek paylaşılan
  kart bileşeni, tüm kartlar bunun üzerine kurulu.

### Badges / Status Pills
- **Style:** üç parçalı opaklık formülü — `bg-{renk}-500/10 border-{renk}-500/30
  text-{renk}-400` (renk = 7 anlam-renginden biri, ör. `rose` kritik alarm için).
  `rounded-full` veya `rounded` (küçük etiketler), mono font, genelde
  `text-[9-10px] uppercase`.
- **State:** merkezi bir `Badge` bileşeni yok — her kart kendi opaklık
  üçlüsünü satır içi tekrarlıyor (DRY ihlali var, görsel tutarlılık yok
  değil — renk formülü her yerde aynı).

### Data / Metric Tile
- **Style:** bkz. Typography → Named Rules → The Triad Rule. Genelde sol
  kenarlıkta ilgili anlam-rengiyle ince bir `border-l-2` vurgusu var
  (ör. `border-l-cyan-500`).

### Charts (signature component)
- Harici kütüphane yok — tamamen elle yazılmış SVG. `ThroughputChart.tsx`
  referans: çizim elemanlarında (line/path/area) ham hex renk
  (`stroke="#22d3ee"`), lejant/metin elemanlarında Tailwind class
  (`text-cyan-300`) karışık kullanılıyor. Izgara çizgileri elle
  hesaplanmış `x()`/`y()` fonksiyonlarıyla, tick etiketleri `fontSize="10"`
  ham SVG `<text>`. Yeni bir grafik ihtiyacı her zaman bu kalıpla
  (harici kütüphane değil) çözülür.

### Live Diagrams (signature component)
- `TrafficFlowDiagram.tsx` (721 satır) ve `TopologyCard.tsx` (343 satır) —
  agent/hub/cihaz arası canlı paket akışını animasyonlu elle-SVG olarak
  gösteren, projenin en karmaşık görsel bileşenleri. `requestAnimationFrame`
  ile tek döngü, `prefers-reduced-motion` saygılı. İkisi de test kapsamında.

## Do's and Don'ts

### Do:
- **Do** her yeni vurgu ihtiyacında önce 7 anlam-renginden (cyan/violet/
  emerald/amber/rose/sky/orange) birinin yeniden kullanılıp
  kullanılamayacağını düşün.
- **Do** sayısal/teknik veriyi her zaman `font-mono` ile göster, UI
  metnini sans-serif'te bırak.
- **Do** yeni bir grafik/görsel ihtiyacında elle SVG yaz (`ThroughputChart.tsx`
  kalıbı) — çizim elemanlarında ham hex, metin/legend'de Tailwind class.
- **Do** yeni bir stat/metrik eklerken Triad kalıbını (Label→Data→Caption)
  koru.
- **Do** dar ekranlarda büyük SVG diyagramlarını `overflow-x-auto` +
  `min-width` ile yatay kaydırmaya bırak, metni küçültüp okunamaz hale
  getirme (bkz. TrafficFlowDiagram/TopologyCard, landing page LivePreview).
- **Do** grid/flex içindeki her kart/hücreye `min-w-0` ekle — yoksa
  `truncate` etkisiz kalır ve içerik komşu hücreye taşar (bkz. Elevation
  öncesi bulunan text-overflow bulgusu).
- **Do** dim/tertiary metinde `text-dim-aa` kullan (`slate-600`/`700`
  değil) — AA kontrastını (4.5:1+) garanti eden tek dim tonu bu.

### Don't:
- **Don't** yeni bir ikon/animasyon/chart kütüphanesi ekleme (lucide-react,
  framer-motion, recharts vb.) — proje genelinde kasıtlı olarak sıfır.
- **Don't** bir kart için gölge ekleme, sadece `LoginScreen`/`GeoMapCard`
  gibi gerçekten sayfadan kopması gereken istisnai yüzeylerde kullan.
- **Don't** açık tema (light mode) dalı açma — `color-scheme: dark`
  kasıtlı ve tek yönlü.
- **Don't** bir rengi anlamı dışında kullanma (ör. rose'u dekoratif bir
  vurgu için, kritik-olmayan bir yerde) — sözleşme kırılır.
- **Don't** ayrı bir `.css`/CSS-module dosyası açma; Tailwind class'ları
  JSX'te satır içi kalır.
