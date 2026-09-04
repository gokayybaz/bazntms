---
target: frontend Overview.tsx (Dashboard)
total_score: 27
max_score: 40
na_heuristics: 
p0_count: 3
p1_count: 2
target_identity: "file:/Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/Overview.tsx"
target_fingerprint: "sha256:71aae3b86b6023e32e9070eb4b0f105d8e5eb69d2934c6ef2fa29087c9de9c67"
target_path: /Users/gokaybaz/Desktop/projects/bazntms/frontend/src/components/Overview.tsx
timestamp: 2026-09-04T16-21-05Z
slug: frontend-src-components-overview-tsx
---
Method: dual-agent (A: general-purpose · B: general-purpose)

## Design Health Score (Operate mode — Persuade'daki gibi n/a yok, tüm 10 puanlandı)

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3/4 | WS:CANLI rozeti güçlü, ama REST-polling hataları sessizce yutuluyor |
| 2 | Match System / Real World | 4/4 | NOC diline tam uyum (SNMP, LLDP/CDP, NetFlow, syslog, pid) |
| 3 | User Control and Freedom | 2/4 | Satırlar tıklanamıyor, akış durdurma/dondurma kontrolü yok |
| 4 | Consistency and Standards | 4/4 | Card/Triad kalıbı 8 bölümde kırılmadan uygulanmış |
| 5 | Error Prevention | 3/4 | Girdi yüzeyi az ama sessizce yutulan hatalar örtük risk taşıyor |
| 6 | Recognition Rather Than Recall | 4/4 | Renk-kodlu rozetler, yazılı uyarı etiketleri |
| 7 | Flexibility and Efficiency | 1/4 | Klavye kısayolu yok, toplu işlem yok, detaya geçiş yok |
| 8 | Aesthetic and Minimalist Design | 3/4 | Yoğunluk kasıtlı ve iyi yönetilmiş ama mobil çakışma + renk ihlali puan kırıyor |
| 9 | Error Recovery | 1/4 | Hub çökse/oturum düşse "canlı" rozeti yanıltıcı kalabilir |
| 10 | Help and Documentation | 2/4 | Boş durum metni iyi ama kalıcı yardım/kısayol listesi yok |
| **Total** | | **27/40** | **Acceptable — sağlam disiplin, güven/erişilebilirlik boşlukları var** |

## Design Specificity Verdict

**LLM assessment:** Jenerik bir admin panel template değil — Card.tsx kalıbı, Triad (Label→Data→Caption) üçlüsü ve flat-by-default elevation kuralı sayfanın tüm 8 bölümünde istisnasız tekrarlanıyor; rx=cyan/tx=violet çifti trafik kartlarında doğru yönde kullanılmış. Ama tam da en kritik yerde — uyarı rengi sözleşmesinde — çatlak var: `ALERT_KIND_STYLES` içinde `ioc` uyarı türü DESIGN.md'nin 7 rengi dışından bir `red-500` kullanıyor (rose zaten "kritik alarm" için ayrılmışken), ve `target` türü `violet`'i **tek başına** birincil vurgu olarak kullanıyor — DESIGN.md'nin kendi "Fixed Meaning Rule"ı bunu açıkça yasaklıyor. Aynı violet-tek-başına kalıbı olay akışındaki "AGENT" rozetinde de tekrarlanıyor — tek seferlik değil, alışkanlık. Özetle: gerçek bir kimlik var ama güvenlik/uyarı gibi sözleşmenin en çok önem taşıdığı yüzeyde disiplin gevşiyor.

**Deterministic scan:** CLI (statik JSX, yalnızca Overview.tsx): 12 bulgu — `design-system-font-size` 7 (DESIGN.md'nin deklare ettiği tip skalasının dışında 8.5-9.5px kullanımlar — DESIGN.md'nin skalası eksik, kod daha granüler), `side-tab` 4 (DESIGN.md'de zaten dokümante edilmiş kasıtlı kalıp — aksiyon gerekmiyor), `ai-color-palette` 1.

Canlı tarayıcı taraması (render edilmiş CSS dahil, tam sayfa): 1114 ham bulgu → 20'si doğrulanmış yanlış pozitif (dedektörün kendi overlay etiketleri birbirini örtüyor, `text-occlusion` kuralının tamamı) → **1094 gerçek bulgu**. Baskın: `undersized-ui-text` 830, `tiny-text` 162, `ai-color-palette` 52 (marka tutarlılığı, landing page'deki gibi çoğunlukla kabul edilebilir), `text-occlusion` (yanlış pozitif, hariç), `nested-cards` 15, `low-contrast` 14 (gerçek WCAG AA hatası — örn. `#45556c on #0b1225` = 2.5:1), `text-overflow` 14 (gerçek: `truncate` classes taşmayı önlemiyor, 99-160px taşma), `side-tab` 4, `all-caps-body`/`wide-tracking`/`flat-type-hierarchy` 1'er.

**Visual overlays:** Enjeksiyon başarılı, ham veri toplandı; overlay kendisi kalıcı görünür bırakılmadı (Vite HMR/sayfa devam ediyor).

## Overall Impression

Sağlam bir temel var — Card/Triad/flat-elevation disiplini gerçek ve kod-taraması bunu doğruluyor. Ama iki cephede ciddi boşluk var: (1) **güven** — ağ hatası sessizce yutuluyor, bir izleme aracı için "canlı görünüp aslında bayat" riski kabul edilemez; (2) **erişilebilirlik/okunabilirlik ölçeği** — 830+162 alt-eşik metin örneği ve 14 gerçek kontrast hatası, "yoğun ama okunabilir" ile "yoğun ve okunamaz" arasındaki çizgiyi aştığını gösteriyor. Mobilde sidebar hiç daralmıyor, içerik sıkışıp taşıyor — masaüstü-öncelikli bir ürün mobilde kullanılamaz durumda.

## What's Working

- **Triad + flat-by-default disiplini gerçek**: 8 bölümün hepsinde kırılmadan tekrarlanmış, dekoratif değil.
- **Boş durum metni ürüne özel yazılmış**: "Henüz akış yok — online agent bekleyin ya da cihazları NetFlow/Syslog için hub'a yönlendirin" — jenerik "veri yok" yerine somut yönlendirme.
- **Rozet/renk taksonomisi tarama hızını gerçekten destekliyor** — PRODUCT.md'nin "saniyeler içinde okuma" hedefiyle örtüşüyor.

## Priority Issues

- **[P0] Ağ hatası tamamen sessiz — "canlı" görünüp aslında donmuş veri riski.** Why it matters: `agents`/`devices`/`flows`/`syslog` fetch'lerinin hepsi hatayı yutuyor, 401'de sessizce dönüyor; hub çökerse operatör "WS: CANLI" rozetini görmeye devam edip bayat sayılara güvenebilir — bir izleme ürünü için güven açığı. Fix: her poll hook'una `lastError`/`stale` state'i + kart başlığında görünür "bağlantı koptu" rozeti. Suggested command: /impeccable harden
- **[P0] Mobilde layout kırık: sidebar daralmıyor, stat değerleri çakışıyor, kartlar 99-160px taşıyor.** Why it matters: masaüstü-öncelikli bir NOC aracı mobilde tamamen kullanılamaz hale geliyor — sayının kendisi ürün, okunamazsa görev tamamlanamaz. Fix: sidebar'a mobil breakpoint'te collapse/drawer davranışı, stat değer satırlarına `truncate`+`min-w-0`, dar ekranda `text-2xl`→`text-xl`. Suggested command: /impeccable adapt
- **[P0] Sistemik alt-eşik metin: 830 undersized-ui-text + 162 tiny-text, bir kısmı AA kontrastını da geçemiyor (14 low-contrast, en kötüsü 2.5:1).** Why it matters: captions/etiketler/timestamp/pid katmanının büyük kısmı 11px eşiğinin altında; bu hacimde "yoğunluk kasıtlı" savunması geçerliliğini yitiriyor. Fix: DESIGN.md'nin tip skalasını genişletip (8.5-10px aralığı için de bir alt-kural tanımla) gerçek bir taban belirle, dim renk tonunu en az slate-400'e çek. Suggested command: /impeccable typeset
- **[P1] Uyarı renk sözleşmesi ihlali: `ioc`→sözleşme-dışı red, `target`→violet tek başına.** Why it matters: DESIGN.md'nin Fixed Meaning Rule'ını iki şekilde çiğniyor, en kritik uyarı türü (IOC) icat edilmiş bir renkle işaretleniyor. Fix: `ioc`→rose, `target` violet yerine mevcut 7 renkten biri. Suggested command: /impeccable colorize
- **[P1] "Tam liste aşağıda" var olmayan içeriğe yönlendiriyor.** Why it matters: Agent Filosu kartındaki metin bu sayfada olmayan bir içeriğe işaret ediyor (gerçek liste `/Agent'lar`'da) — heuristic #2'yi kırıyor. Fix: metni `/agentlar`'a Link'e çevir. Suggested command: /impeccable clarify
- **[P2] Card-inside-card (15 hit) + düz tipografi hiyerarşisi (h1 13px→h2 11px→body 10.5px, adım oranı 1.18, hedef 1.25).** Suggested command: /impeccable layout
- **[P3] Etkileşimsiz olay satırları (Alex tıklayıp detaya geçemiyor) + filtre butonları 24×24px dokunma hedefinin altında, focus-visible tanımlı değil (Sam).** Suggested command: /impeccable onboard

## Persona Red Flags

**Alex (Power User):** Olay akışındaki hiçbir satır tıklanabilir değil — şüpheli bir IP görüp ilgili agent'ın detay sayfasına geçmek için sidebar'dan manuel arama gerekiyor. Tek bir klavye kısayolu yok.

**Sam (Accessibility-Dependent):** Akış filtre butonları ~18-20px yükseklikte, WCAG 2.2 AA'nın 24×24px minimum dokunma hedefinin altında, `focus-visible` zenginleştirmesi tanımlı değil.

## Minor Observations

- Yalnızca "Açık Uyarı" KPI'sı state'e göre renk değiştiriyor; "Aktif Bağlantı"/"Olay Hızı" hiçbir eşikte renk değiştirmiyor.
- `pid` etiketleri `text-slate-700` ile neredeyse görünmez kontrastta.
- `side-tab` (4) ve `ai-color-palette` (52, canlı) çoğunlukla DESIGN.md'de zaten dokümante edilmiş kasıtlı kalıp/marka tutarlılığı — aktif backlog'a alınmadı (landing page'deki brand-consistency kalıntısıyla aynı mantık).

## Questions to Consider

1. IOC/tehdit uyarısı zaten en kritik uyarı türüyken neden rose yerine sözleşme-dışı bir red icat edildi — bilinçli bir "daha da kritik" sinyali mi, yoksa DESIGN.md hiç kontrol edilmeden mi kodlandı?
2. Sessizce yutulan fetch hataları — "kullanıcıyı hata mesajıyla boğma" felsefesinin bilinçli sonucu mu, yoksa gözden kaçmış bir MVP kısayolu mu?
3. Sayfa 0 uyarı ile 20 açık uyarı arasında aynı sabit bölüm sırasını koruyor — aktif olay anında sayfa kendini yeniden düzenlemeli mi?
