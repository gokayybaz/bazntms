#!/usr/bin/env bash
# Faz 21 soak testi (S21.13).
#
# Ölçek yığınına uzun süre (varsayılan 8 saat) birleşik hedef yük uygular;
# her SAMPLE_EVERY (vars. 5 dk) RSS / goroutine / açık FD / heap / kuyruk
# birikimi / DB satır sayısı örnekler. Bitişte ilk saat platosunu son saatle
# karşılaştırır — monoton büyüme (sızıntı) varsa FAIL. docs/perf/soak-<utc>.md
# raporu üretir.
#
# CI kısa varyantı: DURATION=30m scripts/soak.sh
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="${COMPOSE:-$ROOT/deploy/docker-compose.scale.yml}"
DC="docker compose -f $COMPOSE"
HUB_AGENT="${HUB_AGENT:-http://localhost:8081}"
PROM_URL="${PROM_URL:-http://localhost:9090}"
ENROLL_TOKEN="${ENROLL_TOKEN:-scale-enroll-token}"
AUTH_PASSWORD="${AUTH_PASSWORD:-demo123}"
FLOW_TARGET="${FLOW_TARGET:-127.0.0.1:12055}"
DURATION="${DURATION:-8h}"
SAMPLE_EVERY="${SAMPLE_EVERY:-300}"
AGENTS="${AGENTS:-2000}"
FLOW_RATE="${FLOW_RATE:-25000}"
DEVICES="${DEVICES:-500}"

to_secs() { local n="${1%[smh]}" u="${1: -1}"; case "$u" in s) echo "$n";; m) echo $((n*60));; h) echo $((n*3600));; *) echo "$1";; esac; }
DUR_S=$(to_secs "$DURATION")

UTC="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$ROOT/docs/perf/soak-${UTC}.md"
CSV="$(mktemp)"
mkdir -p "$ROOT/docs/perf"
echo "t_sec,rss_mb,goroutines,open_fds,heap_alloc_mb,queue_pending,db_rows_M" > "$CSV"

q() { curl -s "$PROM_URL/api/v1/query" --data-urlencode "query=$1" 2>/dev/null | python3 -c 'import json,sys
try:
 d=json.load(sys.stdin); r=d["data"]["result"]
 print(sum(float(x["value"][1]) for x in r) if r else 0)
except Exception: print(0)'; }

db_rows() { $DC exec -T timescaledb psql -U bazntms -tAc \
  "SELECT (SELECT count(*) FROM flows)+(SELECT count(*) FROM agent_iface_samples)" 2>/dev/null | tr -d ' \r'; }

echo ">> soak: $DURATION ($DUR_S sn), örnek her ${SAMPLE_EVERY}s — $AGENTS agent / $FLOW_RATE flow-sn / $DEVICES cihaz"
LG="$(mktemp -d)/loadgen"
( cd "$ROOT" && go build -o "$LG" ./cmd/bazntms-loadgen )
"$LG" -mode mixed -hub "$HUB_AGENT" -token "$ENROLL_TOKEN" -agents "$AGENTS" -interval 30 -spread 60s \
  -flow-target "$FLOW_TARGET" -flow-rate "$FLOW_RATE" -flow-proto mix -flow-exporters 12 \
  -duration "$DURATION" > /tmp/soak-lg.log 2>&1 &
LG_PID=$!
"$LG" -mode device -hub "${HUB_AGENT/8081/8080}" -password "$AUTH_PASSWORD" -devices "$DEVICES" -device-poll 60 \
  -duration "$DURATION" > /tmp/soak-dev.log 2>&1 &
DEV_PID=$!
trap 'kill $LG_PID $DEV_PID 2>/dev/null; rm -rf "$(dirname "$LG")" "$CSV"' EXIT

start=$(date +%s)
while :; do
  now=$(date +%s); t=$((now-start))
  [ "$t" -ge "$DUR_S" ] && break
  rss=$(docker stats --no-stream --format '{{.MemUsage}}' $($DC ps -q hub-ingest hub-controller) 2>/dev/null \
        | awk -F/ '{v=$1; sub(/[A-Za-z]+/,"",v); if($1 ~ /GiB/) v*=1024; s+=v} END{printf "%.0f", s}')
  gor=$(q 'sum(go_goroutines{role=~"controller|ingest"})')
  fds=$(q 'sum(process_open_fds{role=~"controller|ingest"})')
  # heap_alloc = canlı nesneler (heap_inuse serbest span'leri de sayar; sızıntı
  # sinyali için canlı olan doğru ölçü)
  heap=$(q 'sum(go_memstats_heap_alloc_bytes{role=~"controller|ingest"})/1024/1024')
  qp=$(q 'max(bazntms_queue_pending)')
  rows=$(db_rows); rows_m=$(python3 -c "print(f'{${rows:-0}/1e6:.1f}')" 2>/dev/null || echo 0)
  printf '%d,%s,%.0f,%.0f,%.0f,%.0f,%s\n' "$t" "${rss:-0}" "${gor:-0}" "${fds:-0}" "${heap:-0}" "${qp:-0}" "$rows_m" >> "$CSV"
  echo "  [${t}s] rss=${rss}MB gor=${gor%.*} fd=${fds%.*} heap=${heap%.*}MB queue=${qp%.*} rows=${rows_m}M"
  sleep "$SAMPLE_EVERY"
done
kill $LG_PID $DEV_PID 2>/dev/null

# --- analiz: ısınma sonrası ilk üçte-bir vs son üçte-bir ortalaması ---
python3 - "$CSV" "$OUT" "$DURATION" <<'PY'
import csv, sys, statistics
csv_path, out_path, dur = sys.argv[1], sys.argv[2], sys.argv[3]
rows = list(csv.DictReader(open(csv_path)))
n = len(rows)
verdict, notes = "PASS", []
if n < 6:
    verdict, notes = "SKIP", ["yeterli örnek yok (%d) — daha uzun süre / sık örnek" % n]
else:
    k = max(1, n // 3)
    warm = n // 3   # ilk üçte-bir tamamen ısınma (rampa + pool + Go plato) sayılır
    # medyan: kuyruk-boşaltma burst'lerindeki geçici goroutine/FD sıçramalarına dayanıklı
    def band(lo, hi, col): return statistics.median(float(r[col]) for r in rows[lo:hi])
    # SIZINTI sinyalleri (FAIL eder): canlı heap, goroutine, açık FD
    for col, label, tol in [("heap_alloc_mb","canlı heap",1.35),
                            ("goroutines","goroutine",1.25),
                            ("open_fds","açık FD",1.25)]:
        first, last = band(warm, warm+k, col), band(n-k, n, col)
        r = last/first if first else 0
        tag = "OK" if r <= tol else "SIZINTI?"
        if r > tol:
            verdict = "FAIL"
        notes.append(f"{label}: {first:.0f} → {last:.0f} (×{r:.2f}, eşik ×{tol}) {tag}")
    # RSS: bilgi amaçlı — Go serbest span'leri OS'e geç iade eder; canlı heap
    # düzse RSS artışı sızıntı değildir (GOMEMLIMIT ile bastırılır).
    rf, rl = band(warm, warm+k, "rss_mb"), band(n-k, n, "rss_mb")
    notes.append(f"RSS (bilgi): {rf:.0f} → {rl:.0f} MB (×{rl/rf:.2f}) — canlı heap düzse sızıntı değil")
with open(out_path, "w") as f:
    f.write(f"# Soak testi — {dur}\n\n- Tarih (UTC): {rows[0]['t_sec'] if rows else '-'}\n")
    f.write(f"- Örnek sayısı: {n}\n- Sonuç: **{verdict}**\n\n## Analiz (ilk %25 ort → son %25 ort)\n\n")
    for x in notes: f.write(f"- {x}\n")
    f.write("\n## Ham örnekler\n\n| t (sn) | RSS MB | goroutine | açık FD | canlı heap MB | queue | DB satır (M) |\n|---|---|---|---|---|---|---|\n")
    for r in rows:
        f.write("| " + " | ".join(r[c] for c in ("t_sec","rss_mb","goroutines","open_fds","heap_alloc_mb","queue_pending","db_rows_M")) + " |\n")
print(f"\n=== SOAK: {verdict} ===")
for x in notes: print("  " + x)
print(f"\nrapor → {out_path}")
sys.exit(0 if verdict != "FAIL" else 1)
PY
