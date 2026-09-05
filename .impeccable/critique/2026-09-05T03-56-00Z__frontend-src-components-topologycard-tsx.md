---
target: frontend/src/components/TopologyCard.tsx
total_score: 12
max_score: 24
na_heuristics: 3,5,7,10
p0_count: 1
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/TopologyCard.tsx"
target_fingerprint: "sha256:9e47bfc7d38615bb35153f2a4949b863b65fb1f4311e9c779e057e75bf90ad39"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/TopologyCard.tsx
timestamp: 2026-09-05T03-56-00Z
slug: frontend-src-components-topologycard-tsx
---
Method: dual-agent (A: design-review sub-agent · B: detector/browser-evidence sub-agent)

## Design Health Score

| # | Sezgi | Puan | Anahtar Sorun |
|---|-------|------|----------------|
| 1 | Sistem Durumu Görünürlüğü | 2 | Canlı backend'de 10 gerçek keşif bağlantısı var (hepsi `kind:"subnet"`) ama diyagram hiç keşif olmamış gibi render oluyor |
| 2 | Gerçek Dünya Eşleşmesi | 3 | LLDP/CDP/ARP/subnet terminolojisi bu hedef kitle için meşru NOC jargonu |
| 3 | Kullanıcı Kontrolü ve Özgürlük | n/a | Pasif, salt-okunur görselleştirme |
| 4 | Tutarlılık ve Standartlar | 1 | "subnet" legend rengi (violet, satır 329) hem Fixed Meaning Rule'ü kırıyor hem ölü kod |
| 5 | Hata Önleme | n/a | Korunacak kullanıcı girdisi yok |
| 6 | Tanıma > Hatırlama | 3 | Satır-içi gösterge, ARP host noktalarında `<title>` tooltip |
| 7 | Esneklik ve Verimlilik | n/a | Pasif diyagrama uygulanabilir hızlandırıcı yok |
| 8 | Estetik ve Minimalist Tasarım | 2 | 8.5px altı üç metin + hiç tetiklenemeyen bir legend girdisi yer kaplıyor |
| 9 | Hata Tanıma/Kurtarma | 1 | Sunucu tarafında keşif verisi var ama renderer'ın onu sessizce düşürdüğüne dair hiçbir işaret yok |
| 10 | Yardım ve Dokümantasyon | n/a | Uygulama geneli temel çizgide tutarlı (hiçbir yerde bağlamsal yardım yok) |
| **Toplam** | | **12/24** (6 sezgi, 4 n/a) | **%50 — Acceptable (bandın altı)** |

## Tasarım Özgünlüğü Hükmü

**Gerçekten ürüne özgü.** Yatay client(agent) ▸ HUB ▸ keşfedilen cihazlar ▸ ROUTER ▸ İNTERNET sahnesi PRODUCT.md'nin "hub+agent+cihaz üçlü mimarisi" iddiasını doğrudan kodluyor. `ROUTER_KINDS` türetme mantığı (gerçek bir router/firewall cihazı sabit ROUTER yuvasını canlı adı/çevrimiçi-durumuyla devralıyor) başka bir ürünün inşa etmeye gerekçesi olmayan özel bir detay. Ama bu özgünlüğün hemen yanında, backend'in ürettiği **gerçek keşif verisinin %100'ü** hiç görselleştirilmiyor.

**Deterministik tarama**: CLI temiz (`[]`, exit 0) — ama bu dar regex kurallarının (glow/gradient/font klişeleri) bu dosyanın asıl sorunlarını (küçük-punto eşiği, ölü-kod, SVG ARIA) yapısal olarak kontrol edemediği anlamına geliyor, gerçek bir "temiz" değil.

**Canlı API + kod izi (iki değerlendirme bağımsız doğruladı)**: `GET /api/v1/topology` **10 gerçek bağlantı** döndürüyor — hepsi `kind:"subnet"`, hepsi `source_type:"agent"`. Satır 126 (`if (l.kind === 'subnet') continue`) VE satır 127 (`if (l.source_type !== 'device') continue`) bu 10 bağlantının tamamını `discoveredEdges`/`hosts`'a ulaşmadan eliyor. Sonuç: diyagram hiç keşif olmamış gibi görünüyor, ama alt başlık açıkça "agent subnetleri" vaat ediyor. `TopologyCard.test.tsx`'in 4 testinin tamamı `links: []` ile çalışıyor — bu regresyon yeşil CI altında görünmez şekilde taşınıyor.

**Subnet legend — canlı doğrulandı**: 48 elemanlı render edilmiş SVG'de `#a78bfa` (violet) rengini taşıyan **tam olarak 1 eleman** var — legend'in kendi swatch'ı. Başka hiçbir çizgi/nokta bu rengi kullanmıyor; kod izi ve canlı DOM ölçümü birebir örtüşüyor. Ayrıca DESIGN.md'nin Fixed Meaning Rule'ü violet'i her zaman cyan ile eşleşmesi gereken tx-trafik rengi olarak sabitliyor — burada (render olabilse bile) tek başına bir keşif-kategorisi etiketi olarak kullanılıyor, bu oturumda AlertsCard/ComplianceCard/FortiPanel'de zaten defalarca düzeltilen aynı ihlal.

**SVG erişilebilirlik — canlı doğrulandı**: 9 `<circle>`, 22 `<text>`, 2 `<rect>`, 8 `<line>` render ediliyor; `<svg>`'in kendisi `role/aria-label/aria-hidden/tabindex` = `null`, `<title>` çocuğu yok; örneklenen 9 circle + 22 text'in **hiçbiri** `tabindex`/`role`/`aria-label` taşımıyor. GeoMapCard'ın (bu oturumda düzeltilen) aksine, veri varken hiçbir yedek metin/liste yok — tamamen sessiz.

**Kontrast — gerçek kompozit zemine karşı ölçüldü** (`rgb(11,18,36)`, panel+base üst üste): `fill-slate-500` **3.92:1** (AA altı), `fill-slate-600` **2.46:1** (3:1 büyük-metin tabanının bile altı), `fill-slate-700` **1.80:1** (zeminden zar zor ayrışıyor). Tam envanter (satır numaralarıyla): 500→226,258,272,326,328,330,332; 600→184,262,291,303(circle),307,334,336; 700→**yalnızca satır 191**. Satır 191 hem dosyanın en kötü kontrastını taşıyor HEM DE gerçek bir durum bilgisi ("+N çevrimdışı gizli" — gizlenen offline agent sayısı) — en önemli bilgilerden biri en kötü kontrastta.

**8.5px altı metin envanteri** — bazıları dekoratif (statik "ROUTER"/"NET"/"internet" etiketleri, satır 255/269/272), bazıları **gerçek durum bilgisi**: satır 191 (gizli-agent sayısı, 8px + 1.8:1), satır 226 (LLDP/CDP port etiketi, 8px), satır 258 (router adı, 7px — dosyanın en küçüğü), satır 262 (router `kind · online/offline` durumu, 7.5px), satır 291 (agent site), satır 307 (device kind·host).

## Öne Çıkanlar
1. Router-yuvası türetme mantığı (satır 48/87/240-265) — gerçek gateway cihazı canlı adı/durumuyla sabit yuvayı devralıyor, tek-en-önemli-cihazı doğru önceliklendiriyor.
2. Responsive strateji doğru yapılmış — 375px'te canlı doğrulandı: `overflow-x-auto` + `min-w-[720px]` metni küçültmek yerine kaydırmaya bırakıyor, DESIGN.md'nin kendi kuralına tam uyuyor.
3. Beş-sütunlu sahne ve kenar-türü renk ayrımı bazNTMS'in mimarisine özgü yazılmış, ödünç bir şablon değil.

## Öncelikli Sorunlar

**[P0] Gerçek keşif verisinin %100'ü görünmez — canlı API'de doğrulandı, testler bunu kaçırıyor**
- **Neden önemli**: `GET /api/v1/topology` 10 gerçek bağlantı döndürüyor (hepsi subnet/agent-kaynaklı) ama ikili filtre (satır 126 kind, satır 127 source_type) hepsini eliyor. Alt başlık "agent subnetleri" vaat ediyor, `TopologyCard.test.tsx`'in 4 testi de `links:[]` kullandığından bu regresyon CI'da hiç yakalanmıyor.
- **Düzeltme**: `posOf`'u hem `device` hem `agent` kaynaklarını çözecek şekilde genişlet, satır 127'deki `source_type !== 'device'` kısıtını kaldır, subnet-türü bağlantıları (çözümlenemeyen komşular gibi) mevcut `hosts` noktası deseniyle render et.
- **Önerilen komut**: `/impeccable harden`

**[P1] Subnet legend girdisi kanıtlı ölü kod + renk-sözleşmesi ihlali**
- **Neden önemli**: Canlı doğrulandı — 48 elemanlı SVG'de yalnızca legend'in kendisi `#a78bfa` kullanıyor. P0 düzeltilirse bu renk gerçekten kullanılır hale gelir ama violet'in tek-başına kullanımı DESIGN.md'nin kuralını hâlâ kırar.
- **Düzeltme**: P0 ile birlikte subnet kategorisine nötr bir ton ata (ARP'ın nötr sloganıyla tutarlı, 7 sabit-anlam renginden birini işgal etmeden).
- **Önerilen komut**: `/impeccable colorize`

**[P1] SVG tamamen erişilebilirlik-sessiz + en kritik metin hem küçük hem AA-altı**
- **Neden önemli**: Canlı ölçüldü — 9 circle + 22 text'in hiçbirinde erişilebilir isim yok, veri varken hiç yedek metin/liste yok (GeoMapCard'dan bile geride). Satır 262 (router online/offline durumu) 7.5px + `fill-slate-600` (2.46:1) — hem boyut hem renk kat kat ihlal.
- **Düzeltme**: GeoMapCard'da kurulan deseni tekrarla — düğümlere `role="button"`+`tabIndex`+`aria-label`, dekoratif elemanlara `aria-hidden`, `<svg>`'e `role="group"`+`aria-label`.
- **Önerilen komut**: `/impeccable harden`

**[P2] `fill-slate-500/600/700` kontrast borcu, dosya genelinde**
- **Neden önemli**: Gerçek kompozit zemine karşı ölçülen 3.92/2.46/1.80:1 — üçü de AA'nın altında, en kötüsü (700) gerçek durum bilgisi taşıyan tek satırda.
- **Düzeltme**: `fill-dim-aa`'ya taşı (Tailwind v4 tema token'ı otomatik olarak bu utility'yi de üretir — `text-dim-aa` ile aynı renk).
- **Önerilen komut**: `/impeccable harden`

**[P2] "KEŞFEDİLEN CİHAZ" başlığı açık bir boş-durum yerine sessizce kayboluyor**
- **Neden önemli**: Canlı doğrulandı (hem `/topoloji` hem Dashboard) — orta sütun boşken (satır 187 koşulu) başlık DOM'dan tamamen siliniyor, tamamen-boş durumun (satır 176-179, açık bir cümle yazan) aksine hiçbir açıklama yok.
- **Düzeltme**: Başlığı koşulsuz render et, boşken soluk bir "(0)" ekle.
- **Önerilen komut**: `/impeccable clarify`

**[P2] Gerçek durum bilgisi taşıyan metinler 8.5px altında**
- **Neden önemli**: Router adı(7px)/durumu(7.5px), gizli-agent sayısı(8px), port etiketi(8px), agent site(8px), device kind·host(8px) — DESIGN.md'nin Micro Scope Rule tabanının (8.5px) altında, dekoratif değil gerçek veri.
- **Düzeltme**: En az 8.5px'e çıkar.
- **Önerilen komut**: `/impeccable typeset`

**[P3] Dar ekranlarda kaydırma ipucu yok**
- **Neden önemli**: 375px'te doğrulandı — 720px genişliğindeki SVG'nin yalnızca ~%35'i (253px) görünüyor, hiçbir kenar-gölgesi/ipucu yok; kartın kendi yuvarlak kenarlığı kırpılmış görünümü "tamamlanmış" gibi gösteriyor.
- **Düzeltme**: Sağ kenara hafif bir fade maskesi veya kaydırma ipucu.
- **Önerilen komut**: `/impeccable polish`

## Persona Kırmızı Bayrakları

**Sam**: Diyagramdan gerçek anlamda sıfır ilişkisel bilgi alıyor — ne ARIA ne yedek metin ne odaklanabilir eleman. Ekran okuyucunun teorik olarak aktarabileceği tek gerçek bilgi (router online/offline) aynı zamanda dosyanın en küçük ve en düşük-kontrastlı metni.

**Riley**: Gerçek canlı veri setiyle (10 gerçek keşif bağlantısı) beslendiğinde diyagram keşif hiç çalışmamış gibi davranıyor — Riley bunu ilk gerçek-dünya kontrolünde "özellik sessizce hiçbir şey yapmıyor" diye işaretler. Subnet legend swatch'ı hiçbir yapılandırmada karşılığı olmayan bir vaat.

## Küçük Gözlemler
- Dashboard kart başlığı "LLDP/CDP/ARP" derken `/topoloji` alt başlığı "LLDP/CDP/ARP keşfi + agent subnetleri" diyor — aynı widget, iki farklı kapsam iddiası, ikisi de subnet boşluğu yüzünden tam doğru değil.
- `trunc(routerDev.name, 10)` (satır 259) 7px'te, ARP host noktalarının aksine kırpılan adı geri getirecek bir tooltip yok.
- Legend sabit piksel ofsetleri kullanıyor (`cx={48}`, `cx={96}`...) — gelecekte yeni bir girdi tüm sonraki x-ofsetlerinin elle yeniden ayarlanmasını gerektirir.

## Kışkırtıcı Sorular
- Bu diyagram gerçek bir ortamda keşif verisinin %100'ünü sessizce düşürüyorsa, boş görünen bir topoloji operatöre ne anlatıyor?
- "subnet" gerçek bir render yolu mu almalı, yoksa alt başlık ve legend yapılamayacağı bir kategoriyi vaat etmeyi mi bırakmalı?
- Bu diyagramdaki düğümler, Dashboard'ın olay-akışı satırlarında (bu oturumda zaten düzeltilen) olduğu gibi kendi detay sayfalarına tıklanabilir olmalı mı?
