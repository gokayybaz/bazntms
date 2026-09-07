---
## Doğrulama (tedarik zinciri)

```bash
# cosign blob imzası (herhangi bir binary / paket)
cosign verify-blob --bundle bazntms-agent-linux-amd64.bundle \
  --certificate-identity-regexp 'https://github.com/__REPO__/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  bazntms-agent-linux-amd64

# SLSA build provenance (binary + konteyner imajı)
gh attestation verify bazntms-agent-linux-amd64 --repo __REPO__
gh attestation verify oci://ghcr.io/__OWNER__/bazntms-hub:__V__ --repo __REPO__

# Helm chart
helm pull oci://ghcr.io/__OWNER__/charts/bazntms --version __V__
```

SBOM: `bazntms-sbom.spdx.json` (bu sürümün ek dosyaları arasında).
Konteyner imajları: `ghcr.io/__OWNER__/bazntms-{hub,agent}:__V__` (amd64 + arm64).
