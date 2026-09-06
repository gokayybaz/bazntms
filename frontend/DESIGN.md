---
name: bazNTMS Dashboard
description: Ağ trafiği izleme paneli — htop/ncurses estetiğinde çok-panelli terminal TUI
colors:
  bg-base: "#0a0d13"
  panel: "#10141d"
  panel-2: "#151b27"
  border: "#232b3a"
  border-hi: "#35485f"
  ink-primary: "#ced7e3"
  ink-hi: "#f1f5fa"
  dim: "#8794a8"
  tui-dim: "#8794a8"
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
    fontSize: "0.8125rem"
    fontWeight: 700
    lineHeight: 1.35
  value:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.8125rem"
    fontWeight: 700
    lineHeight: 1.35
  label:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.6875rem"
    fontWeight: 500
    lineHeight: 1.3
    letterSpacing: "0.04em"
    textTransform: "uppercase"
  caption:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.6875rem"
    fontWeight: 400
    lineHeight: 1.35
  micro:
    fontFamily: "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.625rem"
    fontWeight: 400
    lineHeight: 1.3
  prose:
    fontFamily: "system-ui, -apple-system, Segoe UI, Roboto, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 400
    lineHeight: 1.6
rounded:
  all: "0"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
components:
  panel:
    backgroundColor: "{colors.panel}"
    border: "1px solid {colors.border}"
    rounded: "0"
    padding: "12px 16px"
  panel-title:
    textColor: "{colors.ink-hi}"
    typography: "{typography.label}"
    accent: "┤ … ├ (box-drawing) veya reverse-video şerit"
  meter:
    track: "[ … ] köşeli parantez"
    fill: "█ / | ; boş: ·"
    thresholds: "<60% emerald · <85% amber · ≥85% rose"
  tui-table-header:
    backgroundColor: "{colors.rx-cyan}"
    textColor: "{colors.bg-base}"
    typography: "{typography.label}"
  row-selected:
    backgroundColor: "{colors.border-hi} @ 50%"
    textColor: "{colors.ink-hi}"
    note: "nötr grileştirme — cyan yalnızca başlıkta"
  fn-key-bar:
    backgroundColor: "{colors.rx-cyan}"
    textColor: "{colors.bg-base}"
    typography: "{typography.micro}"
---

# Design System: bazNTMS Dashboard

> **Durum (2026-09-06):** TUI dönüşümü **tamamlandı** (Faz 17 izleme yüzeyi +
> Faz 18 Uyumluluk/ISMS + Yönetim). Tüm sayfalar bu dilde; `Card.tsx` silindi
> (tek konteyner `Panel`), tüm native `prompt/confirm` `useDialog()`'a taşındı
> (çok-alanlı `form()` dahil), tüm `slate-*`/`dim-aa` tokenlere göç etti.

## Overview

**Creative North Star: "htop çok-panelli terminal"**

Bir terminal çok-panelli izleyicisi (htop, btop, glances, k9s) gibi düşünülmüş:
düz siyah zemin, tek monospace aile, karakter-hücresi hissi veren yoğun ızgara,
köşeli parantezli ölçer çubukları (`[|||||||    ]`), reverse-video başlıklar,
alt sabit `F-tuşu` şeridi ve **gerçek klavye-öncelikli navigasyon**. Fare
çalışır ama birincil değildir — her eylemin bir tuşu vardır.

Önceki "Ağ Operasyon Merkezi" dili zaten yolun yarısındaydı (koyu zemin,
mono=veri, 7-renk sabit-anlam sözleşmesi, sıfır harici kütüphane, elle SVG).
TUI dönüşümü bunu bir adım öteye taşır: mono artık **her yerde** (etiketler
dahil), köşeler **tamamen kare**, gölge **yok**, yüzeyler **opak**.

**Key Characteristics:**
- Düz siyah zemin (`#0a0d13`), açık tema yok, gölge yok, blur yok, nokta-ızgara yok
- Tek font: JetBrains Mono her yerde (uzun açıklama paragrafı → bilinçli `font-sans`)
- Köşeler kare (`* { border-radius: 0 }` global)
- 7 renkli sabit anlam sözleşmesi **korunur** (htop yakınsaması: yeşil=düşük/sağlıklı,
  sarı=uyarı, kırmızı=kritik, cyan=birincil/seçim)
- İmza bileşenler: `Meter` · `TuiTable` · `Panel` · `Sparkline` · `TabBar` · `FnKeyBar`
- Klavye-öncelikli: `1-9` sekme, `↑↓/jk` satır, `/` filtre, `F1-F10` bağlam eylemi
- İnce CRT: hareketsiz tarama çizgisi + canlı değerlerde blok imleç
- Sıfır harici ikon/animasyon/chart kütüphanesi (değişmedi)

## Colors

Palet karakteri: düz siyah zemin üzerine, her biri tek bir operasyonel anlamı
sabitleyen 7 doygun ANSI-benzeri vurgu rengi. Renkler `index.css` `@theme`
bloğunda CSS custom property (`--color-*`); kod tabanı Tailwind `slate-*`
kullanımından bu tokenlere faz boyunca göç ediyor.

### Semantic accents — the Fixed Meaning Rule

| Rol | htop karşılığı | Token / hex | Not |
|-----|----------------|-------------|-----|
| rx / birincil / seçim | cyan (htop başlık) | `--color-rx` `#22d3ee` | En sık vurgu. **Tablo/log başlık şeridi, aktif sekme, F-tuşu bar zemini** (reverse-video). Link, aktif nav, odak. Seçili satır cyan DEĞİL — nötr gri. |
| tx | magenta | `--color-tx` `#a78bfa` | Her zaman rx ile çift (rx/tx lejantı). Asla tek başına birincil vurgu değil. |
| sağlıklı / düşük yük | green (user CPU) | emerald-400 `#34d399` | online, onay, `Meter` ilk eşiği (`<60%`). |
| uyarı / orta yük | yellow | amber-400 `#fbbf24` | eşik aşımı, `Meter` ikinci eşik (`<85%`), pps/bw zirvesi. |
| kritik / yüksek yük | red (system CPU) | rose-400 `#fb7185` | kritik alarm, hata, kesinti, `Meter` üçüncü eşik (`≥85%`). |
| süreç-alarm / ağ ucu | blue/cyan | sky-400 `#38bdf8` | `proc` uyarı sınıfı, internet/ağ ucu vurgusu. |
| harici entegrasyon | — | orange-400 `#fb923c` | FortiGate / REST API kaynağı — "bu veri harici bir sistemden". |

**The Fixed Meaning Rule.** Her vurgu rengi tam olarak bir operasyonel anlama
sabitlenmiştir. Bir rengi anlamı dışında kullanmak (bir butonu "güzel dursun"
diye rose yapmak) sözleşmeyi kırar. Yeni bir görsel kategori gerektiğinde önce
bu 7 rengin yeniden kullanılıp kullanılamayacağı düşünülür; yeni renk icadı son
çare. `Meter` eşikleri bu kuralın en katı uygulamasıdır: yük düştükçe
emerald → amber → rose, tıpkı htop CPU çubuğu gibi.

### Neutrals

| Token | hex | Kullanım |
|-------|-----|----------|
| `--color-ground` | `#0a0d13` | sayfa zemini; `html { color-scheme: dark }` zorunlu. slate-950'den daha derin, hafif mavi-bias fosfor siyahı. |
| `--color-panel` | `#10141d` | kart/konteyner zemini. **OPAK** — önceki `slate-900/70` alfa + blur kaldırıldı. |
| `--color-panel-2` | `#151b27` | iç içe yüzey, pasif sekme, zebra satır. |
| `--color-rule` | `#232b3a` | kenarlık, ayraç çizgisi. |
| `--color-rule-hi` | `#35485f` | vurgulu kenarlık + box-drawing aksanı (`┤ ├ ─ │`). |
| `--color-ink` | `#ced7e3` | birincil okunabilir metin. |
| `--color-ink-hi` | `#f1f5fa` | vurgulu metin / başlık. |
| `--color-tui-dim` | `#8794a8` | açıklama, altyazı, en düşük öncelikli metin. Ground üzerinde ~6.4:1 (WCAG AA ✔). |

**Dim borç.** Eski `text-slate-500/600/700` kullanımları koyu zeminde AA'yı
geçmiyordu (slate-600 ~2.4:1, slate-700 ~1.8:1). `--color-tui-dim` (=`text-dim-aa`,
=`text-tui-dim`, aynı değer) AA-uyumlu yerine geçendir. Faz 17 dosyaları
dokunuldukça göç eder; S17.31 faz-geneli süpürme ile kalanlar temizlenir.

## Typography

**Tek aile:** JetBrains Mono (`--font-mono`; fallback `ui-monospace,
SFMono-Regular, Menlo, "DejaVu Sans Mono", monospace`). Gövde fontu `body`
üzerinde mono'ya sabitlenmiştir — **her metin mono'dur**. Tek istisna: uzun
açıklama paragrafları (rapor açıklamaları, ISMS politika metinleri) bilinçli
`font-sans` ile sistem sans-serif'e döner.

JetBrains Mono kurulu değilse fallback yığınına düşülür; box-drawing (`─│┤├`) ve
blok rampası (`▁▂▃▄▅▆▇█`) ve braille karakterleri fallback fontlarda da hizalanır — **hiçbir layout
box-drawing karakter genişliğine bağlı değildir** (yalnızca görsel aksan).

### Hierarchy

- **Data / Value** (700, 13px, mono): stat/`Meter` değerleri, tablo sayıları.
  htop'ta dev sayı yoktur — hepsi aynı hücre boyu. Eski `text-2xl` stat
  değerleri bu boyuta indi.
- **Label** (500, 11px, mono, `uppercase tracking-[0.04em]`): panel başlıkları,
  stat etiketleri, tablo başlık hücreleri. Panel başlığı ayrıca box-title
  aksanı (`┤ AGENTS ├`) veya reverse-video şerit alır.
- **Caption** (400, 11px, mono, `tui-dim`): ikincil bilgi ("filo toplamı ·
  gelen · canlı").
- **Micro** (400, 10px, mono, `tui-dim`): yalnızca taranabilir meta-veri —
  zaman damgası, pid, birim eki, kısa tür rozeti. Asla okunması gereken metin.
- **Prose** (400, 13px, **sans**, 1.6): yalnızca uzun açıklama blokları.

### Named Rules

**The Triad Rule.** Her metrik üç parçalı aynı kalıptan geçer:
uppercase **Label** → bold mono **Data** → dim **Caption**. `Meter` bu kalıbın
görsel yoğunlaştırılmış halidir (label + çubuk + değer tek satırda).

**The Micro Scope Rule.** 11px altı yalnızca *taranabilir* meta-veri için.
Okunması gereken hiçbir metin (boş-durum, hata, birincil değer) Micro'ya
düşmez, en az Caption (11px) kalır. Micro her zaman `tui-dim` veya daha açık.

## Layout

**Kabuk:** `grid-rows-[auto_auto_1fr_auto]` — üst `TuiHeader` (filo `Meter`
bandı + WS durumu + kimlik + saat) / `TabBar` (numaralı yatay nav) / `<main>`
(kayan sayfa gövdesi) / `FnKeyBar` (alt sabit F-tuşu şeridi). Sol sidebar
kaldırıldı.

**Sayfa içi:** `Panel` konteynerlerine bölünmüş yoğun ızgara — bir ekranda
5+ panel + 8+ metrik olağan (terminal izleyicinin doğası). SVG diyagramları
(TrafficFlowDiagram, TopologyCard) dar ekranda küçülüp okunamaz olmak yerine
`overflow-x-auto` + `min-width` ile yatay kaydırmaya geçer. `TabBar` de dar
ekranda yatay kaydırır.

## Shapes & Borders

Köşeler **tamamen kare** — `* { border-radius: 0 !important }` global kuralı
tüm `rounded-*` utility'lerini nötrleştirir. Kenarlıklar her zaman ince
(`1px solid`), `--color-rule` veya vurgu bağlamında `--color-rule-hi`.
Box-drawing karakterleri (`┤ ├ ─ │ ┌ ┐ └ ┘`) yalnızca panel başlığı aksanı ve
diyagram bağ çizgisi olarak — hizalama-kritik hiçbir yerde değil.

**Elevation yok.** Gölge kaldırıldı (önceki `LoginScreen` + `GeoMapCard`
istisnaları dahil). Derinlik yalnızca zemin katmanlarıyla ifade edilir:
ground → panel → panel-2 → rule.

## Motion

**İnce CRT.** `body::after` hareketsiz tarama çizgisi (1px/3px,
`rgba(0,0,0,.13)`, `pointer-events:none`) — saf siyahta görünmez, panel/metinde
hafif fosfor koyulaşması. Canlı/güncellenen değerlerin yanında `.tui-cursor`
blok imleci (sert on/off yanıp sönme, `steps(1)`).

**`prefers-reduced-motion`:** blok imleç durur, log-tail autoscroll anlık
atlar, `TrafficFlowDiagram` paket animasyonu durur (mevcut davranış). Tarama
çizgisi hareketsiz olduğu için kalır.

## Components

### Panel (tek paylaşılan konteyner)
- Kaynak: `frontend/src/components/Panel.tsx`. Eski `Card.tsx` bunun ince
  alias'ıdır (S17.31'de silinir).
- **Zemin:** `--color-panel` (opak). **Kenarlık:** `1px solid --color-rule`.
  **Köşe:** kare. **Gölge:** yok. **İç boşluk:** `p-4`; başlık şeridi `px-4 py-2.5`.
- **Başlık:** box-title aksanı `┤ BAŞLIK ├` (mono, uppercase, `ink-hi`) +
  isteğe bağlı sağ slot.

### Meter (imza bileşen)
- Kaynak: `frontend/src/components/Meter.tsx`.
- **Yapı:** `LABEL [███████········] değer birim` — köşeli parantezli çubuk,
  dolgu `█`/`|`, boş `·`, sağ-hizalı değer.
- **Eşik renkleri:** `<60%` emerald · `<85%` amber · `≥85%` rose (props ile
  override edilebilir). htop CPU/Mem/Swap çubuğunun birebir dili.
- **Statik** — animasyon yok; değer değişince anlık günceller.

### TuiTable (imza bileşen)
- Kaynak: `frontend/src/components/TuiTable.tsx`.
- **Başlık:** reverse-video (`bg-rx text-ground`), mono uppercase; aktif sort
  kolonu `▼`/`▲`. **Cyan yalnızca başlıkta** — satırlarda kullanılmaz.
- **Satır:** `↑↓`/`j`/`k` seçim (wrap yok), `g`/`G` baş/son, `Enter` →
  `onActivate(row)`. Seçili satır **nötr gri highlight** (`bg-rule-hi/50
  text-ink-hi`) — cyan değil; hücre renkleri (rx/tx/emerald…) okunur kalır.
- **Filtre:** `/` → label'lı filtre alanına odaklan, `Esc` temizler + çıkar.
  Başlık **filtrelenmiş** satır sayısını gösterir (filtre aktifken `/ toplam`).
- **Kesme:** `maxRows` üstünde sınırlı yükseklik + iç kaydırma + kesme notu.
- **A11y:** gerçek `<table>` semantiği, `<th scope="col">`, seçili satır
  `aria-selected`, filtre `<label>`.

### TabBar / FnKeyBar
- `TabBar`: numaralı yatay nav (`1:PANO 2:AGENT …`), aktif sekme reverse-video,
  `1-9` global hotkey ile route. Dar ekran `overflow-x:auto`.
- `FnKeyBar`: alt sabit şerit, reverse-video. `KeymapContext`'ten aktif ekranın
  F-tuşu eylemlerini çeker (`F1` yardım, `F5` yenile, `F10` çıkış varsayılan +
  ekran-özel `F2-F6`).

### Sparkline
- Kaynak: `frontend/src/components/Sparkline.tsx`. Sayı dizisi → 8 seviyeli blok rampası
  (`▁▂▃▄▅▆▇█`) sabit genişlik string; `toSparkChars` saf fonksiyonu ayrı ihraç. Liste satırlarında satır-içi trend
  (agent en-yoğun-arayüz, cihaz). Harici kütüphane yok.

### Badges / Status Pills
- `StatusPill` (`frontend/src/components/StatusPill.tsx`): köşeli, `●`/`○` +
  etiket, mono `text-[10px] uppercase`. 7 anlam-renginden biri; opaklık üçlüsü
  `bg-{renk}-500/10 border-{renk}-500/30 text-{renk}-400`.

### Charts (signature component)
- Harici kütüphane yok — elle SVG. `ThroughputChart.tsx` referans: basamaklı
  çizgi (`stroke`), mono tick etiketleri, karakter-ızgara zemin, **gradyan/
  area-fill yok**, endpoint nokta vurgusu. Çizim elemanlarında ham hex,
  metin/legend'de token/Tailwind class.

### Live Diagrams (signature component)
- `TrafficFlowDiagram.tsx`, `TopologyCard.tsx` — agent/hub/cihaz arası canlı
  paket akışını animasyonlu elle-SVG gösteren en karmaşık bileşenler. Köşeli
  düğüm kutuları, mono etiket, `─│` bağ estetiği. `requestAnimationFrame` tek
  döngü, `prefers-reduced-motion` saygılı. İkisi de test kapsamında.

### Dialog
- `Modal.tsx` → ortalanmış bordürlü kutu (ncurses dialog): `1px --color-rule-hi`,
  box-title, backdrop `rgba(0,0,0,.6)` (blur yok), `Esc` kapat, focus trap.
  `useConfirm()` / `usePrompt()` promise API — native `confirm/prompt` yerine.

## Keyboard Model

Kaynak: `frontend/src/lib/useHotkeys.ts` + `KeymapContext.tsx`. Tek global
`keydown`; `<input>/<textarea>/[contenteditable]` odaktayken tek-harf
kısayolları bastırılır (`Esc` alanı bırakır).

| Tuş | Kapsam | Eylem |
|-----|--------|-------|
| `1`–`9` | global | Sekme değiştir (`TabBar` sırası) |
| `↑ ↓` / `j k` | liste & log | Satır seçimi taşı |
| `g` / `G` | liste & log | Başa / sona |
| `Enter` | liste | Seçili satırı aç |
| `/` | liste | Filtre alanına odaklan (`Esc` temizler) |
| `s` / `F6` | tablo | Sırala (kolon döngüsü) |
| `F1` / `?` | global | Klavye yardım overlay'i |
| `F2` | Uyarılar | Eşik/bildirim "Setup" ekranı |
| `F3` / `F4` | bağlam | Ara / Filtre |
| `F5` | global | Aktif sayfayı yenile |
| `F10` | global | Oturumu kapat (onay) |

## Do's and Don'ts

### Do:
- **Do** her yeni vurgu ihtiyacında önce 7 anlam-renginden birinin yeniden
  kullanılıp kullanılamayacağını düşün.
- **Do** her metni mono bırak; yalnızca uzun açıklama paragrafında `font-sans`.
- **Do** yeni tablo/liste ihtiyacında `TuiTable`, yeni ölçüm çubuğunda `Meter`,
  yeni konteynerde `Panel` kullan.
- **Do** yeni grafik ihtiyacında elle SVG yaz (`ThroughputChart.tsx` kalıbı) —
  basamaklı çizgi, mono tick, gradyan yok.
- **Do** yeni bir etkileşimli ekranda `useRegisterKeys` ile F-tuşu eylemlerini
  kaydet; her eylemin tıklanır bir karşılığı da olsun.
- **Do** grid/flex içindeki her hücreye `min-w-0` ekle (yoksa `truncate` etkisiz).
- **Do** dim/tertiary metinde `text-tui-dim` (veya `text-dim-aa`) kullan.

### Don't:
- **Don't** yeni ikon/animasyon/chart kütüphanesi ekleme (lucide, framer-motion,
  recharts…) — proje genelinde kasıtlı sıfır.
- **Don't** gölge ekleme — hiçbir yüzeyde. Derinlik zemin katmanıyla.
- **Don't** köşe yuvarlama — global kural kare zorlar, karşı `!important` yazma.
- **Don't** açık tema (light mode) dalı açma — `color-scheme: dark` tek yönlü.
- **Don't** bir rengi anlamı dışında kullanma.
- **Don't** box-drawing karakterini hizalama-kritik bir layout'a koyma —
  yalnızca görsel aksan.
- **Don't** ayrı `.css`/CSS-module dosyası açma; Tailwind class JSX'te satır içi
  (global tokenler ve `.tui-*` util'ler `index.css`'te).
- **Don't** klavye kısayolunu fare-only bir eylemin tek yolu yapma.
