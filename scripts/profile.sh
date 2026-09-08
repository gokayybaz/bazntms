#!/usr/bin/env bash
# Faz 21 profil koşum kabı (S21.6).
#
# Yük sürerken çalışan bir hub'ın (-pprof <addr> ile başlatılmış; block/mutex
# için ayrıca -pprof-rates N) CPU/heap/goroutine/block/mutex profillerini toplar,
# `go tool pprof -top` özetlerini çıkarır, docs/perf/pprof/<utc>/ altına yazar.
# İkinci argüman bir önceki koşunun dizini ise -base diff'i de üretir.
#
# Kullanım:
#   PPROF_URL=http://localhost:6060/debug/pprof scripts/profile.sh 30
#   scripts/profile.sh 30 docs/perf/pprof/20260101T000000Z   # diff'li
set -euo pipefail

DUR="${1:?kullanım: profile.sh <cpu-profil-saniyesi> [onceki-dizin]}"
BASE="${2:-}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PPROF_URL="${PPROF_URL:-http://localhost:6060/debug/pprof}"

command -v go >/dev/null || { echo "go bulunamadı" >&2; exit 1; }
curl -fsS "$PPROF_URL/" >/dev/null 2>&1 || { echo "pprof erişilemiyor: $PPROF_URL (hub -pprof ile mi?)" >&2; exit 1; }

OUT="$ROOT/docs/perf/pprof/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$OUT"
echo ">> profiller → $OUT"

fetch() { # ad url
  echo "   $1"
  curl -fsS "$2" -o "$OUT/$1.pb.gz" || { echo "   ($1 alınamadı — profilleme kapalı olabilir)"; return 0; }
  go tool pprof -top -nodecount=30 "$OUT/$1.pb.gz" > "$OUT/$1.top.txt" 2>/dev/null || true
  if [ -n "$BASE" ] && [ -f "$BASE/$1.pb.gz" ]; then
    go tool pprof -top -nodecount=30 -base "$BASE/$1.pb.gz" "$OUT/$1.pb.gz" > "$OUT/$1.diff.txt" 2>/dev/null || true
  fi
}

echo "   cpu (${DUR}s örnekleniyor)"
curl -fsS "$PPROF_URL/profile?seconds=$DUR" -o "$OUT/cpu.pb.gz"
go tool pprof -top -nodecount=30 "$OUT/cpu.pb.gz" > "$OUT/cpu.top.txt" 2>/dev/null || true
if [ -n "$BASE" ] && [ -f "$BASE/cpu.pb.gz" ]; then
  go tool pprof -top -nodecount=30 -base "$BASE/cpu.pb.gz" "$OUT/cpu.pb.gz" > "$OUT/cpu.diff.txt" 2>/dev/null || true
fi

fetch heap      "$PPROF_URL/heap"
fetch goroutine "$PPROF_URL/goroutine"
fetch block     "$PPROF_URL/block"
fetch mutex     "$PPROF_URL/mutex"

echo
echo "=== CPU ilk satırlar ==="
sed -n '1,12p' "$OUT/cpu.top.txt" 2>/dev/null || true
echo
echo "=== heap (inuse_space) ilk satırlar ==="
sed -n '1,10p' "$OUT/heap.top.txt" 2>/dev/null || true
echo
echo ">> tam çıktı: $OUT/*.top.txt  (interaktif: go tool pprof $OUT/cpu.pb.gz)"
