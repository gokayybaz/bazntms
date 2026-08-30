# CLAUDE.md

bazNTMS: Go backend (hub + agent + ctl) + Vite/React frontend, tek binary'ye
gömülü çok platformlu ağ trafiği izleme sistemi. Türkçe kod yorumları ve commit
mesajları kullanılır — bu konvansiyona uyun.

## Build & test komutları

```bash
# Her şeyi derle (frontend + hub + agent + ctl)
make

# Backend
go build ./...
go vet ./...
go test ./...                    # `make test` bunu ÇALIŞTIRMAZ, ayrı çağırın
gofmt -l .                       # bicimlendirme kontrolü

# Frontend (cd frontend içinde)
npm ci
npm run lint                     # oxlint — uyarılar CI'ı kırmaz, hatalar kırar
npm run test                     # vitest run
npm run build                    # tsc -b && vite build (../web/dist'e yazar)
npm run dev                      # Vite dev server, :8080'e proxy (vite.config.ts)

# make test yalnızca: go vet + gofmt -l + tsc -b (go test/npm test'i kapsamaz)
```

CI (`.github/workflows/ci.yml`): ayrı `frontend` job'u (lint+test+build) +
Linux/macOS/Windows'ta `go vet && go test` + `govulncheck`.

## Mimari

Ayrıntılı mimari: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — yakalama
motoru tasarım kararları, collector zamanlayıcıları, uyarı motoru kuralları,
**frontend routing/sayfa yapısı** dahil.

Kısa özet: `internal/*` iş mantığı paketleri, `cmd/{bazntms-hub,bazntms-agent,
bazntmsctl,bazntms-loadgen}` giriş noktaları, `frontend/src/pages/*` her biri
kendi verisini kendi çeken rotalar (React Router, ortak store yok).

## Konvansiyonlar

- **Yorumlar ve commit mesajları Türkçe.** Kod (değişken/fonksiyon adları)
  İngilizce kalabilir.
- **Go**: parametreli SQL sorguları zorunlu (asla string concat ile sorgu
  kurmayın); yeni store metodu eklerken hem `internal/store/store.go`'daki
  `Store` arayüzüne hem `sqlStore` implementasyonuna ekleyin.
- **Frontend**: her sayfa/bileşen kendi API tiplerini yerelde tanımlar (paylaşılan
  `types.ts` yalnızca gerçekten çok yerde kullanılan tipler için — bkz.
  `lib/isms.tsx`, `lib/alertKinds.ts` paylaşım örnekleri). Harici grafik
  kütüphanesi kullanılmaz — SVG elle yazılır (`ThroughputChart.tsx` örnek alın).
  Tailwind class'ları doğrudan JSX'te, ayrı CSS dosyası yok.
- **Docker image'ları**: `Dockerfile.hub` non-root (`bazntms` kullanıcısı,
  `/data` WORKDIR). `Dockerfile.agent` **kasıtlı olarak root** — setcap +
  non-root exec denendi, container'a `cap_add: [NET_RAW, NET_ADMIN]`
  verilmeden binary hiç çalışmadı (Docker'ın varsayılan capability bounding
  set'i bunları içermiyor); Dockerfile'daki yorumda gerekçe var.

## Gerçek veriyle test etme

Docker Desktop + `deploy/docker-compose.scale.yml` bu depoda geliştirme
sırasında birincil test yöntemidir (ölçek mimarisini aynalar: N×ingest +
1×controller + nginx LB + gerçek/sentetik agent'lar):

```bash
docker compose -f deploy/docker-compose.scale.yml up -d --build
# dashboard: http://localhost:8080 (şifre: demo123, enroll token: scale-enroll-token)
# agent API: http://localhost:8081 (lb → ingest havuzu)
```

Frontend'i tek başına değiştirirken `npm run build` sonrası yalnızca
`hub-controller` + `hub-ingest` image'larını yeniden build etmek yeterli
(agent'lara dokunmaz):

```bash
docker compose -f deploy/docker-compose.scale.yml up -d --build hub-controller hub-ingest
```

`npm run build`, `web/dist/.gitkeep`'i siler (`emptyOutDir: true`) —
commit'lemeden önce `git checkout -- web/dist/.gitkeep` ile geri getirin.

## Bilinen durum

- `vault.key` (kimlik kasası master key) bir noktada git geçmişine
  commit edilmişti; `git filter-repo` ile tüm geçmişten temizlendi
  (2026-08-30). Dosya artık `.gitignore`'da, ilk çalıştırmada otomatik üretilir.
- Kapsamlı proje değerlendirmesi ve öncelik sırası için önceki oturumdaki
  analiz notlarına bakın (bu dosyada tekrarlanmadı — zamanla eskir).
