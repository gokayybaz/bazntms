# Release Runbook — Sürüm Çıkarma

Bakımcı dokümanı. Bir sürümü **üretmek** için (tüketmek için bkz.
[`UPGRADE-RUNBOOK.md`](UPGRADE-RUNBOOK.md)). Sürüm tümüyle otomatiktir:
`vX.Y.Z` etiketini push etmek `.github/workflows/release.yml`'i tetikler ve
GitHub sürümü artefaktlarıyla birlikte oluşur. Bu doküman **etiketten önce**
yapılması gerekenleri ve **sonrasında** doğrulanacakları listeler.

## 1) Sürüm numarası

SemVer, v1.0.0'a kadar:

| Değişiklik | Artış | Örnek |
|---|---|---|
| Kırıcı şema / API / config değişikliği, kaldırılan bayrak | minor | `0.3.x → 0.4.0` |
| Yeni özellik, geriye uyumlu | minor | `0.3.2 → 0.4.0` (v0 döneminde feature = minor) |
| Yalnızca düzeltme / doküman / CI | patch | `0.3.2 → 0.3.3` |

Protokol uyumu ayrı: agent ↔ hub `internal/version.ProtocolVersion`. Wire
formatı kırılırsa **önce** bu sayı artar ve `maxProtocolVersion` negotiation'ı
güncellenir — sürüm numarasından bağımsız.

## 2) Etiketten önce — kontrol listesi

```bash
# a. Ağaç temiz + main güncel
git switch main && git pull --ff-only && git status --porcelain   # boş olmalı

# b. Tam test kapısı
make test                        # go vet + gofmt + tsc -b
go test ./...                    # 22 paket (make test KAPSAMAZ)
( cd frontend && npm run lint && npm run test && npm run build )
git checkout -- web/dist/.gitkeep   # npm run build bunu siler (emptyOutDir)

# c. Ölçek yığını dumanı (mimari regresyon)
docker compose -f deploy/docker-compose.scale.yml up -d --build --wait
python3 .github/scripts/scale_smoke_test.py
docker compose -f deploy/docker-compose.scale.yml down -v

# d. DB yükseltme testi (bir önceki release şeması → HEAD) — CI'da da var
go run ./tools/dbcheck -db /tmp/legacy.db && go run ./tools/dbcheck -db /tmp/legacy.db -ping
```

Sürüm-eşitleme — bu yerlerin **hepsi** aynı `X.Y.Z`'yi göstermeli:

| Yer | Nasıl | Otomatik? |
|---|---|---|
| Git etiketi `vX.Y.Z` | `git tag -a` | elle |
| `internal/version.Version` | release.yml `-ldflags -X …=$GITHUB_REF_NAME` | ✅ CI |
| `deploy/helm/bazntms/Chart.yaml` `version` + `appVersion` | elle düzenle → commit | elle (S19.5'te CI'ya taşınacak) |
| `CHANGELOG.md` | `[Yayımlanmamış]` başlığını `[X.Y.Z] — YYYY-MM-DD` yap, altına yeni boş `[Yayımlanmamış]` aç, alttaki compare-link listesini güncelle | elle |

```bash
# Chart sürümünü ayarla + CHANGELOG'u kes → tek "release" commit'i
V=X.Y.Z
sed -i '' -E "s/^version: .*/version: $V/;         s/^appVersion: .*/appVersion: \"$V\"/" \
  deploy/helm/bazntms/Chart.yaml
$EDITOR CHANGELOG.md
git add CHANGELOG.md deploy/helm/bazntms/Chart.yaml
git commit -m "release: v$V"
git push origin main
```

> CI `helm` job'u her push'ta `helm lint` + `helm template | kubeconform`
> (varsayılan + agent/ingress açık) çalıştırır; `release.yml` preflight ise
> etiket ile `Chart.yaml` + `CHANGELOG` eşleşmesini zorlar.

## 2b) Platform atıf motoru — elle doğrulama

> **Yalnızca `internal/agent/bpf/`, `etw_windows.go`, `etwattr.go`, `attrsource_*.go`
> veya `l7helper.go` bu sürümde değiştiyse çalıştır.** CI yalnız saf ayrıştırma
> testlerini (`etwparse_test.go`, `attrcaps_test.go`, `TestETWLayout`) ve Linux
> canlı eBPF dumanını (`ebpf-smoke` job) kapsar; oturum/attach yaşam döngüsü
> gerçek makinede doğrulanmalıdır.
>
> **2026-09-07 durumu:** her iki arka uç da bir kez canlı doğrulandı (Linux
> eBPF scale compose'da, Windows ETW gerçek makinede). ETW callback'i çalışan
> struct düzeni artık `checkLayout()`'ta kritik ofset denetimleriyle kilitli.
> Port/endian doğru çıktı (443/53). Aşağıdaki liste sonraki değişikliklerde
> regresyon kontrolü için.

### Linux eBPF (kernel ≥ 5.8, BTF'li)

```bash
sudo ./bazntms-agent -collect-method=ebpf -pcap -hub-url http://localhost:8080 -enroll-token <t>
# log: "süreç atfı aktif  yöntem=ebpf"
```
- [ ] `/agentlar/:id` → süreç trafiği paneli doluyor (byte'lar `nethogs`/`nload` ile ~uyumlu)
- [ ] DNS paneli doluyor — `resolvectl query example.com` (systemd-resolved, 127.0.0.53) sonrası domain görünüyor
- [ ] L7 paneli: `curl https://example.com` sonrası SNI görünüyor (yardımcı pcap handle)
- [ ] `CAP_NET_RAW` düşür (`-collect-method=ebpf`, `setcap cap_bpf,cap_perfmon=ep`) → sayım + DNS sürüyor, L7 boş + 1× INFO
- [ ] kernel < 5.8 veya `/sys/kernel/btf/vmlinux` yok → log "eBPF atlandı: …", `yöntem=pcap`

### Windows ETW (yükseltilmiş / SYSTEM)

```powershell
# Yönetici PowerShell
.\bazntms-agent.exe -collect-method=etw -pcap -hub-url http://localhost:8080 -enroll-token <t>
```
- [ ] `go test -run TestETWLayout ./internal/agent/` yeşil (struct düzeni ABI ile uyumlu)
- [ ] log: "ETW atıf motoru aktif — süreç trafiği + DNS"
- [ ] `logman query -ets` çıktısında `bazNTMS-Attr` oturumu var
- [ ] Süreç trafiği paneli doluyor (byte'lar Resource Monitor ile ~uyumlu); TCP + UDP, v4 + v6
- [ ] DNS paneli doluyor — `Resolve-DnsName example.com` sonrası domain görünüyor
- [ ] Agent'ı durdur → `logman query -ets` artık `bazNTMS-Attr` göstermiyor (temiz `Stop`)
- [ ] Agent'ı çökert + yeniden başlat → "already exists" hatası yok (yetim oturum temizleniyor)
- [ ] **Npcap KURULU DEĞİL** → süreç trafiği + DNS yine akıyor (ETW pcap'e bağlı değil)
- [ ] `-collect-method=pcap` + Npcap yok → "Npcap kurulu degil" ipucu; `-record` aynı ipucu
- [ ] Yükseltilmemiş kullanıcı → log "ETW atlandı: süreç yükseltilmemiş", `yöntem=pcap` (veya Npcap yoksa temel telemetri)
- [ ] Port/adres doğru yönde: **TCP** hem send hem recv `uzak = daddr:dport`;
      **UDP** send=daddr, recv=saddr. Port big-endian (443/53 — 47873 değil).
      Uzak IP host'un kendi IP'si veya multicast (`224.*`/`239.*`/`ff0x::`)
      **görünmemeli** — `usableRemote()` eler.

> **Port yön/endian notu (doğrulandı):** `kernelNetFlow` ham UserData'yı
> big-endian port + `win:IPv4` bayt-sırası varsayımıyla çözer — 2026-09-07
> canlı testte doğru (443/53). Ters çıkarsa `etwparse.go`'da `binary.BigEndian`
> → `LittleEndian` çevir.

## 3) Etiketi kes

Yorumlu (annotated) etiket — mesaj gövdesi sürümün özetidir (GitHub sürüm
notlarının başında `--generate-notes` commit listesinin üstünde görünür):

```bash
git tag -a vX.Y.Z -m "bazNTMS vX.Y.Z

<CHANGELOG'daki bu sürümün başlıkları — 5-15 satır>
"
git push origin vX.Y.Z
```

`release.yml` (`push: tags: ['v*']`) şunları sırayla yapar:

0. **preflight** (yalnız etiket) — `Chart.yaml` `version`/`appVersion` ve
   `CHANGELOG.md`'de `## [X.Y.Z]` bölümü etiketle eşleşiyor mu. §2'deki bump
   adımı atlandıysa release burada durur.
1. **build** (5 hedef: linux/darwin/windows × amd64/arm64) — frontend + 3 binary
   (`bazntms`, `bazntms-agent`, `bazntmsctl`), `-trimpath`, sürüm ldflags;
   darwin runner'da `.pkg`.
2. **packages** — nfpm ile deb + rpm (amd64 + arm64).
3. **msi** — WiX v4.0.5 ile `bazntms-agent-amd64.msi`.
4. **images** — her bileşen için: yerel amd64 imaj → **Trivy imaj taraması**
   (CRITICAL bloklar) → (hub) `image_smoke_test.sh` → sonra çok-mimari
   (amd64+arm64) derle + push `ghcr.io/gokayybaz/bazntms-{hub,agent}`.
   Etiketler `X.Y.Z` / `X.Y` / `sha-<kısa>` / kararlıda `latest`
   (`docker/metadata-action`); temel imajlar digest'e sabit. Tarama/duman
   push'u kapıya alır — kirli/çökük imaj ghcr'a çıkmaz. Yalnız `v*` etiketinde
   push (`workflow_dispatch` = derle+tara, push etme).
5. **chart** — `helm package` → (etikette) `helm push` `oci://ghcr.io/
   gokayybaz/charts/bazntms` (sürüm `Chart.yaml`'dan, preflight zorlar).
5b. **update-signing** — `UPDATE_SIGNING_SEED` secret'ı tanımlıysa
   `bazntmsctl update sign` ile agent update manifest'ini ed25519 imzalar,
   `manifest.json` (+ cosign `.sig`/`.bundle`) release asset'i olur; hub
   `GitHubSyncer` imzaları geçirir. Secret yoksa: imzasız kanal (§7).
6. **supply-chain** — SBOM (`syft`, SPDX), Trivy **fs** taraması (Go bağımlılık
   ağacı; imaj taraması 4. adımda), **SLSA build provenance**
   (`actions/attest-build-provenance` — binary + paketler; imaj provenance'ı
   4. adımda registry'ye yazılır), `cosign sign-blob` (keyless) → her artefakt
   için `.sig` + `.bundle`.
7. **release** — `gh release create --generate-notes` + üretilen notların
   altına `.github/release-footer.md`'den doğrulama bölümü; tüm artefaktları
   yükler.

Süre ~20-35 dk (arm64 imajları QEMU'da yavaş; gha cache ısınınca düşer).
`gh run watch` ile izleyin.

> **İlk yayında bir kez:** ghcr paketleri (`bazntms-hub`, `bazntms-agent`,
> `charts/bazntms`) özel oluşur. GitHub → Packages → her biri → Package
> settings → **Change visibility → Public** (yoksa `helm install` / `docker
> pull` için pull secret gerekir).

## 4) Etiketten sonra — doğrulama

```bash
gh release view vX.Y.Z --json assets -q '.assets[].name' | sort
```

Beklenen artefakt matrisi (eksikse ilgili job'a bakın):

| Grup | Dosyalar |
|---|---|
| Hub/ctl binary | `bazntms-{linux,darwin,windows}-{amd64,arm64}[.exe]`, `bazntmsctl-*` |
| Agent binary | `bazntms-agent-*` (5 hedef) |
| Paketler | `bazntms-agent-{amd64,arm64}.{deb,rpm,pkg}`, `bazntms-agent-amd64.msi` |
| Tedarik zinciri | her binary/paket için `.sig` + `.bundle`, `bazntms-sbom.spdx.json` |
| Konteyner imajı | GitHub release'de değil — `ghcr.io/.../bazntms-{hub,agent}:X.Y.Z` (`docker buildx imagetools inspect` ile 2 mimari) |
| Helm chart | `oci://ghcr.io/gokayybaz/charts/bazntms` sürüm `X.Y.Z` |
| Provenance | her binary + imaj için SLSA attestation (`gh attestation verify`) |

Konteyner imajı sürümü:

```bash
docker run --rm ghcr.io/gokayybaz/bazntms-hub:X.Y.Z -version   # "bazntms-hub vX.Y.Z …"
# veya çalışan bir hub'da:
curl -s localhost:8080/healthz | jq '{version, protocol_version}'
```

İmza + provenance doğrula (herhangi bir artefakt) — bu komutlar release
notlarının altına da eklenir (`.github/release-footer.md`):

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/gokayybaz/bazntms/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle bazntms-agent-linux-amd64.bundle \
  bazntms-agent-linux-amd64

gh attestation verify bazntms-agent-linux-amd64 --repo gokayybaz/bazntms
gh attestation verify oci://ghcr.io/gokayybaz/bazntms-hub:X.Y.Z --repo gokayybaz/bazntms
helm pull oci://ghcr.io/gokayybaz/charts/bazntms --version X.Y.Z
```

Otomatik güncelleme: hub'ın `GitHubSyncer`'ı en son (draft/prerelease olmayan)
release'i ≤30 dk içinde çeker → `updates/stable/manifest.json`. Fleet ~30 dk +
agent yoklama aralığında yükselir; `/api/v1/agents` sürüm dağılımından izleyin.
Manuel: `curl -s localhost:8080/api/v1/agent/update/manifest?channel=stable`.

## 5) Ön sürüm (rc) / prova

```bash
git tag -a vX.Y.Z-rc1 -m "vX.Y.Z aday 1" && git push origin vX.Y.Z-rc1
```

`-rc` / `-beta` / `-alpha` içeren etiketler için `release` job'u
`--prerelease` işaretler; `GitHubSyncer` prerelease'i **atlar** → fleet
kendini güncellemez, artefakt doğrulaması güvenle yapılır. Prova bitince
gerçek `vX.Y.Z` etiketini kesin (rc etiketini silmeyin — geçmiş kalsın).

Tüm pipeline'ı etiketsiz denemek: Actions → Release → **Run workflow**
(`workflow_dispatch`); `release` job'u yalnız `refs/tags/v*`'te çalışır,
gerisi çalışır ve artefaktları workflow artifact'ı olarak bırakır.

## 6) Geri alma / hotfix

- **Bozuk release** (bir job düştü, `release` skip oldu): sorunu main'de
  düzelt, **aynı** `vX.Y.Z` etiketini sil + yeniden kes
  (`git tag -d vX.Y.Z && git push --delete origin vX.Y.Z`, sonra 3. adım).
  Not: etiket zaten bir GitHub release oluşturduysa önce onu sil
  (`gh release delete vX.Y.Z`).
- **Yayımlanmış ama regresyonlu release**: fleet'i durdurmak için en hızlı yol
  GitHub'da release'i **draft**'a çekmek — `GitHubSyncer` bir sonraki yoklamada
  onu görmez, yeni agent'lar eski sürümde kalır (zaten güncellenmişler geri
  gitmez; gerçek downgrade elle). Sonra hotfix `vX.Y.(Z+1)` kesin.
- Şema zaten migrate olduysa geri alma yoktur — etkilenen kurulumlar yedekten
  restore (bkz. DR-RUNBOOK).

## 7) İmzalı agent auto-update kanalı (opt-in)

Varsayılan: `GitHubSyncer` imzasız manifest üretir (güven: GitHub HTTPS +
hub→agent pinli TLS; agent zaten hub'a tam güvenir). Ek ed25519 tedarik-zinciri
imzası — özellikle "hub GitHub'a çıkamıyor" veya "release'i kim üretti kanıtı
gerek" senaryoları — için:

```bash
# 1. Anahtar çifti (imzalama makinesinde, bir kez)
bazntmsctl update keygen -out keys
#   → keys/seed.key (GİZLİ)  +  keys/public.hex (agent'lara dağıtılır)

# 2. Seed'i repo secret'ı yap — bundan sonraki her release manifest'i imzalar
gh secret set UPDATE_SIGNING_SEED < keys/seed.key

# 3. Agent'lara public key: agent.yml → update.public_key: <hex>  (veya -update-key)
#    Bu ayar dolu agent imzayı ZORUNLU kılar — imzasız/yanlış sürüme çıkmaz.

# 4. Doğrula (herhangi bir makinede, release asset'leriyle)
bazntmsctl update verify -pubkey keys/public.hex manifest.json
```

Rotasyon: yeni çift üret → `UPDATE_SIGNING_SEED`'i güncelle → tüm agent'lara
yeni `public_key`'i dağıt. Geçiş penceresi için agent'larda `public_key`'i
geçici boş bırakıp (yalnız SHA-256) yeni anahtar yayıldıktan sonra doldurun.
