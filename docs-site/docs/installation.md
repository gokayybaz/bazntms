---
sidebar_position: 1
---

# Kurulum

## Hızlı başlangıç (sihirbaz)

```bash
./bazntmsctl setup            # interaktif: port, şifre, depo, NATS → hub.yaml
./bazntms-hub -config hub.yaml
```

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
