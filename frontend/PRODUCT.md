# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

İki eş ağırlıklı birincil kullanıcı; kod tabanındaki RBAC rolleri
(admin/netops/analyst/viewer + site-scope) bu ayrımı zaten yansıtıyor:

1. **NetOps/güvenlik operatörü** — vardiya sırasında veya bir olay anında
   dashboard'a bakıp saniyeler içinde canlı trafiği, agent filosu durumunu
   ve uyarı akışını okuma; şüpheli bir olayda agent/cihaz detay sayfasına
   inip derinlemesine inceleme yapma.
2. **Uyumluluk/denetim görevlisi** — periyodik (aylık/yıllık) 5651 +
   ISO 27001 döngüsünde risk defteri güncelleme, SoA gözden geçirme, iç
   denetim/bulgu takibi, tek tıkla denetçi kanıt paketi çıkarma.

Aynı ürünün iki ayrı sayfa ailesi bu iki işi ayrı optimize eder: canlı-
izleme tarafı (Dashboard, Agent'lar, Cihazlar, Topoloji, Uyarılar) ve
uyumluluk tarafı (Uyumluluk alt sayfaları: Risk, SoA, Politikalar,
Denetimler, Yönetişim).

## Product Purpose

bazNTMS: merkezi hub + uç agent + ağ cihazı entegrasyonları üçlüsüne
kurulu, kendi altyapısında (self-hosted) çalışan bir ağ trafiği izleme
platformu. Tek makinede başlayan bir kurulumu 5.000 agent ölçeğine
taşıyabilir. Başarı: bir operatörün trafiği/olayı gerçek zamanlı
görebilmesi + bir denetçinin 5651/ISO 27001 kanıtını tek tıkla
çıkarabilmesi.

## Positioning

Komşu açık kaynak NTMS'lerin gerçekçi biçimde iddia edemeyeceği
mekanizma: hub+agent+cihaz üçlü mimarisi (yalnızca paket yakalama değil,
agent filosu + SNMP/NetFlow/syslog/FortiGate entegrasyonu tek panelde),
agent↔hub karşılıklı TLS, ve — en belirgin fark — hash-zincirli 5651 log
imzalama (Merkle checkpoint + RFC 3161 zaman damgası) + ISO 27001 ISMS
yönetişim modülü gömülü. MIT lisanslı, tek binary'ye gömülü (kurulum
sürtünmesi yok), kendi altyapısında çalışır (vendor lock-in yok).

## Operating Context

NOC/güvenlik operasyon odası veya BT ekibi — vardiya sırasında dashboard'a
bakma, olay/uyarı anında derinlemesine inceleme. Ayrı olarak, düzenli
uyumluluk/denetim döngüsü. RBAC site-scope ile çoklu-site/çoklu-müşteri
(MSP benzeri) kurulumları da destekliyor.

## Capabilities and Constraints

- **Sıfır harici tasarım bağımlılığı**: ikon/animasyon/chart kütüphanesi
  yok. Tüm grafikler elle yazılmış SVG — `frontend/src/components/
  ThroughputChart.tsx` referans örnek (ham hex renk çizim elemanlarında,
  Tailwind class legend/metinde).
- **Tailwind v4, CSS-native config** — `tailwind.config.js` yok. Class'lar
  doğrudan JSX'te satır içi; ayrı CSS/CSS-module dosyası açılmaz.
  `frontend/src/index.css` yalnızca global reset + `--font-mono` token'ı
  taşır, renk token'ı yok (çıplak Tailwind renk adları kullanılıyor).
- **Koyu tema zorunlu** (`html { color-scheme: dark }`) — açık tema yok.
- **JetBrains Mono** tüm sayısal/teknik veri için; UI metni sistem
  sans-serif.
- Her sayfa/bileşen kendi API tipini yerelde tanımlar — paylaşılan
  `types.ts` yalnızca gerçekten çok yerde kullanılan tipler için.
- 38 dosyadan (23 component + ~18 sayfa) yalnızca 6'sının test kapsamı
  var: Overview, TopologyCard, TrafficFlowDiagram, AgentDetailPage,
  AgentsListPage, AlertsPage. Kapsam dışı dosyalara dokunmak sessiz
  regresyon riski taşır.

## Brand Commitments

- Ad: **bazNTMS** — sabit, landing page + README + dokümantasyon genelinde
  kullanılıyor.
- **Renk-anlam sözleşmesi** (kod genelinde tutarlı tekrarlanan bir kural,
  merkezi bir token dosyasında değil): cyan = indirilen/rx trafik,
  violet = gönderilen/tx trafik, emerald = online/sağlıklı durum,
  amber = uyarı/paket hızı, rose = kritik alarm, sky = süreç-tipi alarm,
  orange = vendor (FortiGate vb.) rozeti.
- **Kart/tipografi kalıbı**: her stat kartında 10-11px uppercase tracked
  etiket / font-mono 2xl bold değer / ~10.5px dim açıklama üçlüsü.
  `Card.tsx` paylaşılan kart konteyneri (`rounded-md border
  border-slate-800 bg-slate-900/70`).

## Evidence on Hand

Kod tabanının kendisi: `frontend/src/components/*`, `frontend/src/pages/*`.
`docs/enterprise-plan.html` — kurumsal yol haritası ve lisans
stratejisi (MIT + gelecekte open-core olasılığı, karar bekliyor).
`CLAUDE.md` — proje konvansiyonları. Canlı kullanıcı testi, gerçek
müşteri referansı veya kullanım metriği **yok** — bunlar icat
edilmemeli.

## Product Principles

1. Her görsel karar mevcut renk-anlam sözleşmesini korur — yeni bir renk
   icat etmek yerine cyan/violet/emerald/amber/rose/sky/orange'ın anlamı
   genişletilir veya yeniden kullanılır.
2. Sıfır yeni bağımlılık — ikon/animasyon/chart ihtiyacı her zaman elle
   SVG ile çözülür (ThroughputChart.tsx kalıbı).
3. Canlı-izleme yüzeyleri taranabilirlik/saniyeler-içinde-okunabilirlik
   için; uyumluluk yüzeyleri yapı/izlenebilirlik/denetim-hazırlığı için
   optimize edilir — ikisi aynı görsel dilde, farklı yoğunluk/etkileşim
   önceliğiyle.
4. Refinement > redesign: mevcut kimlik (Card.tsx, tipografi üçlüsü,
   renk sözleşmesi) korunur, üstüne inşa edilir — sıfırdan değiştirilmez.

## Accessibility & Inclusion

Kodda belirli bir standart taahhüdü yok (WCAG uyum iddiası yok), ama
landing page critique'inde WCAG AA kontrast hedefi (4.5:1) fiilen
uygulandı — aynı disiplin frontend'e de taşınmalı. Koyu tema zorunlu
olduğu için açık-tema erişilebilirlik ihtiyacı yok.
