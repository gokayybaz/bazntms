# Dağıtım modeli — bazNTMS (Faz 14 S14.0 kararı)

**Tarih:** 2026-09-06 · **Durum:** kabul edildi · **Seçilen senaryo: B (MSP / çoklu-saha)**

Bu belge "kimin verisi kimden ayrı" sorusunu sunucu-otoriter yanıtlar. Faz 14+
kapsamı bu karara bağlıdır (bkz. [mimari değerlendirme artifact'i]).

## Değerlendirilen senaryolar

| | A — tek kurum | **B — MSP / çoklu-saha** | C — barındırılan çok-kurumlu SaaS |
|---|---|---|---|
| Hedef kitle | Kendi ağını izleyen tek kurum | Birden çok müşteri sahasını / şubesini tek hub'dan yöneten operatör (MSP) veya sert ayrılmış departmanlı tek kurum | Anthropic/3. taraf tarafından barındırılan, birbirini tanımayan kurumlar |
| İzolasyon birimi | yok (tek görünüm) | `site` (serbest metin etiket, sunucu-bağlı) | `tenant_id` (şemada zorunlu, her tabloda) |
| Sır izolasyonu | tek vault anahtarı | tek vault anahtarı (operatör hepsine güvenir) | kiracı-başına vault veri anahtarı + KMS |
| Yönetim | tek `admin` | global `admin` + saha-kısıtlı `site-admin` | kiracı yaşam döngüsü, self-servis onboarding, faturalama |
| Efor | Faz 14 atlanır | **~1–2 hafta** | ~1–2 ay (14a şema · 14b store · 14c auth · 14d onboarding) |

## Seçilen: B

**Neden B:** Hedef "kullanıcı kendi hub'ını kurar, kendi agent'larını ekler" —
kurulumu yapan operatör tüm sahalara güvenir (kendi altyapısı / kendi
müşterileri). Kiracılar arası düşmanca izolasyon (C) gerekmiyor; ama tek bir
hub'ın birden çok sahayı yönetmesi ve her sahanın kendi yöneticisinin yalnız
kendi sahasını görmesi gerekiyor.

### İzolasyon sözleşmesi

- **`site` sunucu-otoriter bir yetki sınırıdır.** Agent'ın `hello.Site`
  beyanına GÜVENİLMEZ; site enroll token'ından türetilir (A3, S12.7 — zaten
  yapıldı). Çoklu-saha modunda (`-multi-site`) site'siz enroll token ile kayıt
  reddedilir (S14.B1).
- **Kimlik `site` alanı** (`users.site` / `api_tokens.site`): boş = **global**
  (tüm sahalar), dolu = **o sahaya kilitli**.
- Saha-kısıtlı bir kimlik hiçbir uçtan başka sahanın verisine ulaşamaz —
  `agent_id` / `device_id` tahmini dâhil. Bunu `SiteScope` + `agentInScope` /
  `deviceInScope` guard'ları + tek "sızıntı testi" (S14.B4) garanti eder.

### Roller (S14.B2)

| Rol | Kapsam | Yetki |
|-----|--------|-------|
| `admin` | global (`site` boş olmalı) | her şey + ISMS/compliance/alert-config/audit-zincir-doğrulama/çoklu-saha ayarları |
| `site-admin` | **kendi sahası** (`site` dolu olmalı) | kendi sahasının kullanıcı / API token / enroll token / agent / cihazlarını yönetir; ISMS/compliance/alert-config GÖREMEZ |
| `netops` | site boşsa global, doluysa kısıtlı | cihaz yönetimi + operate + analyze (admin değil) |
| `analyst` | " | analyze + view |
| `viewer` | " | view |

- `site-admin` başka sahada kullanıcı/token açamaz, global `admin` oluşturamaz
  (yetki yükseltme engeli).
- `PermAdmin` = "kapsam içinde yönetim" — hem `admin` hem `site-admin`'de var.
- **Yeni** `PermGlobalAdmin` = "saha-üstü / kurumsal": ISMS, 5651 compliance,
  uyarı yapılandırması, audit zinciri doğrulama, çoklu-saha ayarları — yalnız
  `admin`.

### Kapsam DIŞI (bilinçli)

- `tenant_id` şeması, kiracı-başına vault anahtarı, KMS → senaryo C. Faz 13
  migrasyon çerçevesi ([[0002-migration-framework]]) bunu ileride mümkün kılar.
- Faturalama, SLA, kiracı self-servis onboarding.
- Saha başına ayrı retention / ayrı uyarı kuralları — operatör global yönetir.

## Faz 14B uygulama sırası

`S14.0` (bu belge) → `S14.B1` (multi-site enroll token zorunluluğu) →
`S14.B2` (site-admin rolü + users/tokens/enroll-tokens/agents kapsam denetimi +
`PermGlobalAdmin`) → `S14.B3` (yönetim UI saha-farkında) → `S14.B4` (tek
parametrize sızıntı testi).

**Bitti tanımı:** bir sahanın `site-admin`'i hiçbir uçtan başka sahanın
verisine/kullanıcısına ulaşamıyor; `security/site_leak_test.go` yeşil.

[mimari değerlendirme artifact'i]: https://claude.ai/code/artifact/4fe68195-7de5-4f83-92b6-f36bcf68db15
