---
sidebar_position: 1
---

# Kurulum

## Hızlı başlangıç (sihirbaz)

```bash
make                          # frontend + hub + agent + ctl tek komutla
./bazntmsctl setup            # interaktif: port, şifre, depo, NATS → bazntms-hub.yml
./bazntms-hub -config bazntms-hub.yml
```

`bazntmsctl` ayrıca GitHub Release sayfasından platformunuza uygun
ön-derli binary olarak indirilebilir (`bazntmsctl-<os>-<arch>`).

## Docker Compose (tek-node demo)

```bash
docker compose -f deploy/docker-compose.yml up --build
# → http://localhost:8080 (şifre: demo123)
```

## Kubernetes

```bash
helm install bazntms deploy/helm/bazntms \
  --set config.database.path="postgres://..." \
  --set config.nats.url="nats://..." \
  --set auth.existingSecret=bazntms-auth
```

Kaynaklar: `docs/CONFIGURATION.md`, `deploy/`.
