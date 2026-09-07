#!/usr/bin/env bash
# bazntms-hub konteyner imajı duman testi (Faz 19 S19.3).
# Verilen imajı çalıştırır ve /healthz'in beklenen sürümü döndürdüğünü doğrular.
#
#   .github/scripts/image_smoke_test.sh <imaj> [beklenen-sürüm]
#
# Örnek:
#   .github/scripts/image_smoke_test.sh ghcr.io/gokayybaz/bazntms-hub:v0.3.3 v0.3.3
#   .github/scripts/image_smoke_test.sh bazntms-hub:local            # sürüm kontrolü atlanır
set -euo pipefail

IMG="${1:?kullanım: image_smoke_test.sh <imaj> [beklenen-sürüm]}"
WANT="${2:-}"
NAME="bazntms-hub-smoke-$$"

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$NAME" -p 18080:8080 "$IMG" -dev >/dev/null

got=""
for _ in $(seq 1 30); do
  if body=$(curl -sf http://127.0.0.1:18080/healthz 2>/dev/null); then
    got=$(printf '%s' "$body" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
    break
  fi
  sleep 2
done

if [ -z "$got" ]; then
  echo "HATA: /healthz 60 sn içinde yanıt vermedi / version taşımadı" >&2
  docker logs "$NAME" >&2 || true
  exit 1
fi
echo "/healthz version = $got"

# /readyz de "ready" dönmeli (DB erişilebilir)
curl -sf http://127.0.0.1:18080/readyz | grep -q '"status":"ready"' \
  || { echo "HATA: /readyz 'ready' değil" >&2; exit 1; }
echo "/readyz = ready"

# root olmayan kullanıcı
uid=$(docker exec "$NAME" id -u)
[ "$uid" != "0" ] || { echo "HATA: konteyner root çalışıyor (uid=0)" >&2; exit 1; }
echo "kullanıcı uid = $uid (root değil)"

if [ -n "$WANT" ] && [ "$got" != "$WANT" ]; then
  echo "HATA: sürüm uyuşmuyor — beklenen '$WANT', gelen '$got'" >&2
  exit 1
fi
echo "OK — $IMG"
