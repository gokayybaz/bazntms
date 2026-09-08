#!/usr/bin/env bash
# Faz 21 sorgu p95 ölçümü (S21.11).
#
# Çalışan bir hub yığınına (yük koşusu sonrası dolu DB) karşı panel/history/
# report uçlarını N kez çağırır, min/medyan/p95/max duvar süresini yazar.
# Hedef: panel sorgusu p95 < 1 sn.
#
# Kullanım:
#   HUB=http://localhost:8080 AUTH_PASSWORD=demo123 scripts/query_bench.sh [tekrar]
set -euo pipefail

HUB="${HUB:-http://localhost:8080}"
AUTH_PASSWORD="${AUTH_PASSWORD:-demo123}"
N="${1:-8}"

TOK=$(curl -fsS -X POST "$HUB/api/login" -H 'Content-Type: application/json' \
  -d "{\"password\":\"$AUTH_PASSWORD\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
[ -n "$TOK" ] || { echo "login başarısız" >&2; exit 1; }

AID=$(curl -fsS "$HUB/api/v1/agents" -H "Authorization: Bearer $TOK" \
  | python3 -c 'import json,sys;d=json.load(sys.stdin);print((d[0] if isinstance(d,list) else d.get("agents",[{}]))[0].get("id",1))' 2>/dev/null || echo 1)

bench() { # ad url
  local name="$1" url="$2" t
  local times=()
  for _ in $(seq "$N"); do
    t=$(curl -fsS -o /dev/null -w '%{time_total}' "$url" -H "Authorization: Bearer $TOK" || echo 99)
    times+=("$t")
  done
  printf '%s\n' "${times[@]}" | python3 -c '
import sys
v=sorted(float(x) for x in sys.stdin)
n=len(v)
p95=v[min(n-1,int(round(0.95*(n-1))))]
print(f"  {sys.argv[1]:<34} min={v[0]*1000:6.0f}ms  med={v[n//2]*1000:6.0f}ms  p95={p95*1000:6.0f}ms  max={v[-1]*1000:6.0f}ms")
' "$name"
}

echo "hub=$HUB  tekrar=$N  örnek agent=$AID"
echo
bench "GET /api/v1/agents"              "$HUB/api/v1/agents"
bench "GET /api/v1/agents/:id"          "$HUB/api/v1/agents/$AID"
bench "GET /api/v1/agents/:id/history"  "$HUB/api/v1/agents/$AID/history"
bench "GET /api/v1/flows"               "$HUB/api/v1/flows"
bench "GET /api/v1/l7"                  "$HUB/api/v1/l7"
bench "GET /api/v1/dns"                 "$HUB/api/v1/dns"
bench "GET /api/v1/processes"           "$HUB/api/v1/processes"
bench "GET /api/v1/geo"                 "$HUB/api/v1/geo"
bench "GET /api/v1/topology"            "$HUB/api/v1/topology"
bench "GET /api/report (fleet 7g)"      "$HUB/api/report?days=7"
bench "GET /api/report?type=enterprise&days=30" "$HUB/api/report?type=enterprise&days=30"
bench "GET /api/report?type=enterprise&days=90" "$HUB/api/report?type=enterprise&days=90"
bench "GET /api/report?type=compliance" "$HUB/api/report?type=compliance"
