# bazNTMS — Tehdit Modeli

**Son güncelleme:** 2026-09-06 (Faz 16 S16.5) · Kapsam: hub + agent + ctl.
Yaşayan belge — mimari değişince güncellenir.

Bu belge varlıkları, güven sınırlarını, sınır başına tehditleri ve mevcut
karşı önlemleri özetler. Bilinen açık bulgular (B1–B8) Faz 11–16'da kapatıldı;
her biri için ilgili karar/commit aşağıda.

---

## 1. Varlıklar

| Varlık | Nerede | Neden değerli |
|--------|--------|---------------|
| **Cihaz kimlik bilgileri** | `devices` tablosu (SNMP community/v3 parola, FortiGate API token) — AES-256-GCM şifreli | Ele geçirilirse ağ altyapısına doğrudan erişim |
| **Vault master anahtarı** | `-vault-key-file` (dosya) VEYA `BAZNTMS_VAULT_MASTER_KEY` (env) | Tüm cihaz kimliklerinin şifresini çözer |
| **Panel oturum token'ları** | `sessions` tablosu (`sha256` hash) veya süreç belleği | Oturum ele geçirme → RBAC yetkileriyle erişim |
| **Agent enrollment token'ları** | `-enroll-token` (statik) + `enroll_tokens` tablosu (`sha256` hash) | Sahte agent kaydı → filo verisi kirletme |
| **Agent kalıcı token'ları** | Agent state dosyası (`0600`) + `agents.token_hash` | Agent kimliğine bürünme (telemetri gönderme) |
| **5651 delil zinciri** | `compliance_logs` (hash-zincir) + `log_checkpoints` (Merkle + RFC3161 + ed25519) | Adli bütünlük — mahkeme delili |
| **Denetim kaydı** | `audit_events` (SHA-256 zinciri) | Yönetim eylemlerinin bütünlüğü |
| **mTLS CA özel anahtarı** | `-tls-dir/ca.key` (`0600`, `O_EXCL`) | Sahte agent/sunucu sertifikası üretimi |
| **Filo telemetrisi** | zaman-serisi tabloları | Ağ topolojisi + davranış istihbaratı; çoklu-sahada saha-izolasyonu gerekli |

---

## 2. Güven sınırları

```
[Agent]──(1)──▶[Hub HTTP/WS]──(2)──▶[Postgres/TimescaleDB]
   ▲                │  ▲                    ▲
   │                │  │(3)                 │(4)
[Tarayıcı]──(2)─────┘  └──[NATS JetStream]──┘
[Ağ cihazı]──(5)──▶[Hub UDP alıcıları]
```

| # | Sınır | Kimlik doğrulama | Notlar |
|---|-------|------------------|--------|
| 1 | Agent ↔ Hub | Enrollment: `X-Enroll-Token`. Sonrası: `Authorization: Bearer <agent_token>` **veya** mTLS istemci sertifikası (`-tls`) | CN'i hub belirler — agent kimlik iddia edemez |
| 2 | Tarayıcı/script ↔ Hub | `nm_session` çerezi veya `Authorization: Bearer` (login token / API token). `-auth-password` yoksa **dev modu**: kimlik yok | RBAC: `admin`/`site-admin`/`netops`/`analyst`/`viewer` |
| 3 | Hub ↔ NATS | NATS kimlik doğrulaması dağıtıma bağlı (compose'da yok — güvenilir ağ varsayımı) | Kuyruk kapalıysa (`-nats` boş) yok |
| 4 | Hub ↔ DB | Postgres kimlik bilgileri DSN'de | Güvenilir ağ / k8s NetworkPolicy varsayımı |
| 5 | Ağ cihazı ↔ Hub UDP | Yok (NetFlow/syslog kimlik doğrulaması yok — protokol sınırı) | Kaynak IP ile cihaz eşleştirilir; sahte datagram enjekte edilebilir → güvenilir yönetim ağı varsayımı |

**Kapsam dışı:** DB/dosya sistemi/NATS/yönetim ağına doğrudan erişimi olan
saldırgan (tam uzlaşma sayılır). Hub'ı çalıştıran OS kullanıcısı.

---

## 3. Sınır başına tehditler ve karşı önlemler

### 3.1 Tarayıcı/script ↔ Hub (sınır 2)

| Tehdit | Karşı önlem | Kalan risk |
|--------|-------------|------------|
| Oturum çalma (XSS) | Frontend React (auto-escape); `dangerouslySetInnerHTML` yok. Çerez `HttpOnly` + `SameSite=Lax` + `Secure` (TLS'te) | — |
| Cross-Site WebSocket Hijacking | **B5** (S16.1): `checkOrigin` — Origin izin listesi (`-public-url`/`-tls-hosts` + localhost); same-origin fallback. WS zaten auth-gate arkasında | İzin listesi boşsa "hepsini kabul + uyarı" (opt-in sertleştirme) |
| Yetki yükseltme (rol/site manipülasyonu) | **S14.B2**: `requirePerm` + `PermGlobalAdmin` (ISMS/5651/uyarı-config/audit-verify yalnız global admin). `sanitizeIdentity` (site'siz site-admin → viewer), `roleSiteConsistent`. `enforceCreateScope` — site-admin global admin oluşturamaz / sahasını değiştiremez. Oturum okumada da `sanitizeIdentity` (S16.5 derinlik savunması) | — |
| Çapraz-saha veri sızıntısı (IDOR) | **S14.B4**: `SiteScope` + `agentInScope`/`deviceInScope` tüm liste/detay uçlarında. `handleAgentDelete/Rename` artık kapsam denetler. Parametrize `site_leak_test.go` (~20 uç + `agent_id`/`device_id` tahmini) | Yeni liste ucu eklerken `site_leak_test.go`'ya satır eklenmeli |
| Bildirim sırlarının sızması | **B1** (Faz 11): `GET /api/alerts` → `PermGlobalAdmin` + yanıtta `•••` maskeleme; PUT'ta maskeli alan = "değiştirme" | — |
| Legacy şifre arka kapısı | **B6** (Faz 11): en az bir etkin RBAC admin varken legacy `-auth-password` girişi reddedilir (break-glass: admin kalmazsa geri döner) | — |
| Brute-force | IP başına 5/dk deneme limiti (replika-başına — paylaşımlı limiter kapsam dışı) | Çoklu replikada N× daha yüksek toplam hız |
| SQL enjeksiyonu | Tüm sorgular parametreli (CLAUDE.md kuralı); `q()` yalnız `?`→`$n` çevirir. Migrasyonlar statik DDL / gömülü | — |

### 3.2 Agent ↔ Hub (sınır 1)

| Tehdit | Karşı önlem | Kalan risk |
|--------|-------------|------------|
| Sahte agent kaydı | Enrollment token zorunlu. **B7/S12.6**: statik token bootstrap-only konumlandı; DB token'ları iptal edilebilir/süreli/site-bağlı. **S14.B1**: çoklu-sahada site'siz token reddedilir | Statik token sızarsa yeniden başlatana dek geçerli (DB token'ları önerilir) |
| Agent'ın site iddiası | **A3/S12.7**: site enroll token'ından türetilir; site-bağlı token'da `hello.Site` yok sayılır | Site'siz token'da `hello.Site` fallback (tek-saha modu) |
| Agent kimliğine bürünme | Kalıcı token (`sha256` hash) veya mTLS. TOFU: `-hub-ca` yoksa ilk `hello` `InsecureSkipVerify` | `-hub-ca` verilmezse ilk bağlantı MITM'e açık (TOFU) |
| Offline agent satırı ele geçirme (`machine_id`) | **S13.6**: reuse yalnız `machine_id`+`site` eşleşen VE **çevrimdışı** satırda; yeni token verilir (eski telemetri okunamaz — telemetri yalnız-yazma). Saldırı için geçerli enroll token + 128-bit `machine_id` bilgisi gerekir | Marjinal — anlamlı erişim kazancı yok |
| Otomatik güncelleme MITM / auth | **B2** (Faz 11): `update.NewClient` Bearer + mTLS transport enjekte eder; imza (ed25519/sha256) doğrulaması | — |
| `agent.yml` token sızması | **B4** (Faz 11): postinstall `0600 root` + MSI registry DACL (`P` bayrağı — Users mirası kesilir) | — |

### 3.3 Depolama & sırlar

| Tehdit | Karşı önlem | Kalan risk |
|--------|-------------|------------|
| DB yedeği + master anahtar → tüm cihaz sırları | AES-256-GCM. **B8/S16.3**: `KeyProvider` — `env` modunda master diske hiç yazılmaz (secret manager / KMS enjeksiyonu) | `env` modunda master `/proc/<pid>/environ` (aynı UID) / crash dump'ta görünür; zarf şifrelemesi + gerçek KMS ertelendi |
| Oturum token'ı DB dökümünden forge | Yalnız `sha256(token)` saklanır — preimage gerekir | — |
| 5651 delil kurcalama | Hash-zincir + saatlik Merkle checkpoint + RFC3161 TSA + ed25519 manifest imzası + `bazntmsctl verify` offline doğrulama | — |
| Denetim kaydı kurcalama | SHA-256 zinciri (`ts\|username\|role\|action\|target\|detail\|ip`) | `site` alanı zincirde değil (yönlendirme meta verisi — bilinçli, bkz. S14.B2) |
| Çoklu replika `vault.key` yarışı | **S15.7**: `O_EXCL` — biri yazar diğeri okur | — |

### 3.4 Ölçek & kullanılabilirlik (sınır 3/4)

| Tehdit | Karşı önlem |
|--------|-------------|
| Controller tek arıza noktası | **A4/S15.3**: `-session-store=db` paylaşımlı oturum. **C1/S15.5-6**: uyarı+poller Postgres advisory-lock lider seçimi. 2× controller + nginx failover (`docker-compose.scale.yml`) |
| Kuyruk mesaj kaybı sessizliği | **C4/S15.8**: `MaxDeliver` sonrası `ingest.dead` DLQ + `bazntms_ingest_dead_total` metriği |
| Şema yükseltme bozulması | **C2/S13**: sürümlü migrasyon runner + CI `db-upgrade-test` (önceki release → HEAD) |

---

## 4. Bilinen kısıtlar (kabul edilmiş)

- **Tek-saha modu** (`-multi-site` kapalı): `site` yalnızca görünüm ayrımı;
  agent `hello.Site` beyanı kabul edilir. Sert izolasyon için `-multi-site`.
- **Kiracı izolasyonu yok** (`tenant_id` yok): senaryo C (barındırılan çok-kurumlu
  SaaS) kapsam dışı — bkz. `docs/DEPLOYMENT-MODEL.md`.
- **NATS/DB/yönetim ağı** kimlik doğrulaması dağıtıma bırakılmıştır (compose
  demo'sunda yok). Üretimde k8s NetworkPolicy / NATS auth / TLS beklenir.
- **UDP alıcıları** (NetFlow/syslog) kimlik doğrulaması yapmaz (protokol sınırı)
  — güvenilir yönetim ağı varsayılır.
- **Rate limiter** replika-başına (paylaşımlı değil).
- **TOFU**: `-hub-ca` verilmezse agent ilk bağlantısı MITM'e açık.
- **Vault**: `env` modu master'ı bellekte tutar; zarf şifrelemesi + gerçek KMS
  (age/AWS KMS/Vault Transit) ertelendi (bkz. `docs/decisions/0006`).

---

## 5. Güvenlik incelemesi geçmişi

| Faz | Kapsam | Sonuç |
|-----|--------|-------|
| Faz 11 | B1–B4, B6 acil düzeltmeler + CI (race/lint/CodeQL/gofmt) | Kapandı |
| Faz 14B | Çoklu-saha izolasyonu (B, `site-admin`, sızıntı testi) | Kapandı |
| Faz 16 S16.1 | B5 — WS origin izin listesi | Kapandı |
| Faz 16 S16.3 | B8 — vault `KeyProvider` (`env`) | Kısmi (zarf/KMS ertelendi) |
| Faz 16 S16.5 | `security-review` diff geçişi (v0.3.1..HEAD) | **Yüksek-güvenilirlikli yeni açık yok** — dal ağırlıklı sertleştirme + kendi `site_leak_test.go` regresyon ağı |
