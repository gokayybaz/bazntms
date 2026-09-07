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
sed -i '' -E "s/^(version|appVersion): .*/\1: X.Y.Z/" deploy/helm/bazntms/Chart.yaml
$EDITOR CHANGELOG.md
git add CHANGELOG.md deploy/helm/bazntms/Chart.yaml
git commit -m "release: vX.Y.Z"
git push origin main
```

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
5. **supply-chain** — SBOM (`syft`, SPDX), Trivy **fs** taraması (Go bağımlılık
   ağacı; imaj taraması 4. adımda), `cosign sign-blob` (keyless) → her artefakt
   için `.sig` + `.bundle`.
6. **release** — `gh release create --generate-notes`, tüm artefaktları yükler.

Süre ~20-35 dk (arm64 imajları QEMU'da yavaş; gha cache ısınınca düşer).
`gh run watch` ile izleyin.

> **İlk yayında bir kez:** ghcr paketleri özel oluşur. GitHub → Packages →
> `bazntms-hub` / `bazntms-agent` → Package settings → **Change visibility →
> Public** (yoksa `helm install` / `docker pull` için pull secret gerekir).

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
| Konteyner imajı | GitHub release'de değil — `ghcr.io/.../bazntms-{hub,agent}:X.Y.Z` (`docker buildx imagetools inspect` ile 2 mimari doğrulanır) |

Konteyner imajı sürümü:

```bash
docker run --rm ghcr.io/gokayybaz/bazntms-hub:X.Y.Z -version   # "bazntms-hub vX.Y.Z …"
# veya çalışan bir hub'da:
curl -s localhost:8080/healthz | jq '{version, protocol_version}'
```

İmza doğrula (herhangi bir artefakt):

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/gokayybaz/bazntms/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle bazntms-agent-linux-amd64.bundle \
  bazntms-agent-linux-amd64
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

## 7) Faz 19'da eklenecek (henüz pipeline'da yok)

- Helm chart OCI push + `helm lint`/`kubeconform` kapısı; `Chart.yaml`
  version/appVersion'ın tag'den türetilmesi (S19.5–S19.6).
- SLSA build provenance attestation'ları (S19.7).
- İmzalı update manifest'i (ed25519) varsayılan (S19.8).
