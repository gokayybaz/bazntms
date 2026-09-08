#!/usr/bin/env bash
# Faz 21 birleşik yük koşusu (S21.3).
#
# Bir profil (loadtest/profiles/<ad>.env) alır, çalışan bir hub yığınına karşı
# agent + flow + cihaz yükünü birlikte sürer, /metrics'i koşu penceresinin
# başında ve sonunda örnekler, perf_summary.py ile PASS/FAIL raporu üretir
# (docs/perf/runs/<utc>-<profil>.md).
#
# Yığın önceden ayağa kaldırılmış olmalı, ör:
#   docker compose -f deploy/docker-compose.scale.yml up -d --build
#
# Kullanım:
#   scripts/loadtest.sh target
#   HUB_PANEL=http://localhost:8080 HUB_AGENT=http://localhost:8081 \
#     ENROLL_TOKEN=... AUTH_PASSWORD=... scripts/loadtest.sh baseline
set -euo pipefail

PROFILE="${1:?kullanım: loadtest.sh <baseline|target|burst> [profil.env]}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENVFILE="${2:-$ROOT/loadtest/profiles/$PROFILE.env}"
[ -f "$ENVFILE" ] || { echo "profil bulunamadı: $ENVFILE" >&2; exit 1; }

HUB_PANEL="${HUB_PANEL:-http://localhost:8080}"
HUB_AGENT="${HUB_AGENT:-http://localhost:8081}"
METRICS_URL="${METRICS_URL:-$HUB_PANEL/metrics}"
PROM_URL="${PROM_URL:-http://localhost:9090}"   # boş = kapalı; çok-replika yığında --profile obs ile kullan
FLOW_TARGET="${FLOW_TARGET:-127.0.0.1:12055}"
ENROLL_TOKEN="${ENROLL_TOKEN:-scale-enroll-token}"
AUTH_PASSWORD="${AUTH_PASSWORD:-demo123}"
STACK="${STACK:-scale}"

# profil değişkenleri
set -a
# shellcheck disable=SC1090
source "$ENVFILE"
set +a

: "${AGENTS:?}" "${INTERVAL:?}" "${DURATION:?}"
FLOW_RATE="${FLOW_RATE:-0}"
FLOW_BURST="${FLOW_BURST:-0}"
FLOW_BURST_AFTER="${FLOW_BURST_AFTER:-2m}"
FLOW_BURST_FOR="${FLOW_BURST_FOR:-5m}"
FLOW_PROTO="${FLOW_PROTO:-mix}"
FLOW_EXPORTERS="${FLOW_EXPORTERS:-8}"
DEVICES="${DEVICES:-0}"
DEVICE_POLL="${DEVICE_POLL:-60}"
WARMUP="${WARMUP:-0s}"

to_secs() { # 10m / 90s / 1h → saniye
  local v="$1" n="${1%[smh]}" u="${1: -1}"
  case "$u" in
    s) echo "$n" ;;
    m) echo $((n * 60)) ;;
    h) echo $((n * 3600)) ;;
    *) echo "$v" ;;
  esac
}
DUR_S=$(to_secs "$DURATION")
WARM_S=$(to_secs "$WARMUP")
WINDOW_S=$((DUR_S - WARM_S))
[ "$WINDOW_S" -gt 0 ] || { echo "DURATION > WARMUP olmalı" >&2; exit 1; }

WORK="$(mktemp -d)"
cleanup() {
  local p
  for p in $(jobs -p); do kill "$p" 2>/dev/null || true; done
  rm -rf "$WORK"
}
trap cleanup EXIT
UTC="$(date -u +%Y%m%dT%H%M%SZ)"
OUTDIR="$ROOT/docs/perf/runs"
mkdir -p "$OUTDIR"
OUT="$OUTDIR/${UTC}-${PROFILE}.md"

echo ">> loadgen derleniyor"
LG="$WORK/loadgen"
( cd "$ROOT" && go build -o "$LG" ./cmd/bazntms-loadgen )

echo ">> hub sağlık denetimi ($HUB_AGENT/healthz)"
curl -fsS "$HUB_AGENT/healthz" >/dev/null || { echo "hub erişilemiyor" >&2; exit 1; }

scrape() { curl -fsS "$METRICS_URL" > "$1" || { echo "metrics çekilemedi: $METRICS_URL" >&2; exit 1; }; }

echo ">> yük başlıyor: profil=$PROFILE süre=${DUR_S}s (ölçüm penceresi ${WINDOW_S}s, ısınma ${WARM_S}s)"

# agent + flow
FLOW_ARGS=()
if [ "$FLOW_RATE" -gt 0 ]; then
  FLOW_ARGS=(-flow-target "$FLOW_TARGET" -flow-rate "$FLOW_RATE" -flow-proto "$FLOW_PROTO" \
             -flow-exporters "$FLOW_EXPORTERS" -flow-burst "$FLOW_BURST" \
             -flow-burst-after "$FLOW_BURST_AFTER" -flow-burst-for "$FLOW_BURST_FOR")
  MODE=mixed
else
  MODE=agent
fi
"$LG" -mode "$MODE" -hub "$HUB_AGENT" -token "$ENROLL_TOKEN" \
  -agents "$AGENTS" -interval "$INTERVAL" -duration "$DURATION" -warmup "$WARMUP" \
  -out "$WORK/lg.json" "${FLOW_ARGS[@]}" &
LG_PID=$!

# cihazlar (ayrı süreç — devpoll zamanlayıcıyı hub sürer)
if [ "$DEVICES" -gt 0 ]; then
  "$LG" -mode device -hub "$HUB_PANEL" -password "$AUTH_PASSWORD" \
    -devices "$DEVICES" -device-poll "$DEVICE_POLL" -duration "$DURATION" &
fi

sleep "$WARM_S"
echo ">> ısınma bitti — /metrics öncesi örnek"
scrape "$WORK/before.prom"
sleep "$WINDOW_S"
echo ">> ölçüm penceresi bitti — /metrics sonrası örnek"
scrape "$WORK/after.prom"

wait "$LG_PID" || true

PROM_ARG=()
if [ -n "$PROM_URL" ] && curl -fsS "$PROM_URL/-/ready" >/dev/null 2>&1; then
  echo ">> Prometheus bulundu ($PROM_URL) — toplam metrikler tüm replikalardan"
  PROM_ARG=(--prom "$PROM_URL")
fi

python3 "$ROOT/scripts/perf_summary.py" \
  --before "$WORK/before.prom" --after "$WORK/after.prom" \
  --loadgen "$WORK/lg.json" --profile "$ENVFILE" --label "$PROFILE" \
  --elapsed "$WINDOW_S" --stack "$STACK" --out "$OUT" "${PROM_ARG[@]}"
