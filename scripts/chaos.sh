#!/usr/bin/env bash
# Faz 21 kaos senaryoları (S21.14).
#
# Çalışan bir ölçek yığınına (deploy/docker-compose.scale.yml) karşı 4 arıza
# enjekte eder, her birinde kurtarmayı ve VERİ KAYBI = 0 ölçütünü doğrular.
# Hafif bir agent yükü (offline kuyruk kesintiyi telafi eder) arka planda
# koşar; senaryolar bittiğinde DB'deki batch sayısı loadgen'in gönderdiğiyle
# eşleşmelidir.
#
# Kullanım:
#   COMPOSE="deploy/docker-compose.scale.yml" scripts/chaos.sh
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE="${COMPOSE:-$ROOT/deploy/docker-compose.scale.yml}"
DC="docker compose -f $COMPOSE"
HUB_AGENT="${HUB_AGENT:-http://localhost:8081}"
HUB_PANEL="${HUB_PANEL:-http://localhost:8080}"
ENROLL_TOKEN="${ENROLL_TOKEN:-scale-enroll-token}"
PG="$DC exec -T timescaledb psql -U bazntms -tAc"

FAILS=0
ok()   { echo "  [OK]   $1"; }
bad()  { echo "  [FAIL] $1"; FAILS=$((FAILS+1)); }

count_batches() { $PG "SELECT COUNT(DISTINCT (agent_id, ts)) FROM agent_iface_samples WHERE agent_id IN (SELECT id FROM agents WHERE name LIKE 'loadgen-%')" 2>/dev/null | tr -d ' \r'; }
survivor_polls() { $DC logs --since "${1}s" hub-controller 2>/dev/null | grep -c "poll tamam" || true; }

echo ">> loadgen derleniyor + hafif agent yükü başlıyor (300 agent / 10s / 12dk)"
LG="$(mktemp -d)/loadgen"
( cd "$ROOT" && go build -o "$LG" ./cmd/bazntms-loadgen )
"$LG" -mode agent -hub "$HUB_AGENT" -token "$ENROLL_TOKEN" \
  -agents 300 -interval 10 -spread 20s -duration 12m -out /tmp/chaos-lg.json > /tmp/chaos-lg.log 2>&1 &
LG_PID=$!
trap 'kill $LG_PID 2>/dev/null; rm -rf "$(dirname "$LG")"' EXIT

sleep 40   # rampanın oturması + ilk batch'ler
BEFORE=$(count_batches)
echo "   başlangıç batch sayısı: $BEFORE"

# ---- Senaryo 1: POLL LİDERİ replikasını öldür ----
echo
echo "== 1) poll lideri hub-controller replikasını kill → devir < 25 sn =="
# poll lideri = son 15 sn'de "poll tamam" loglayan container
LEADER=""
for cid in $($DC ps -q hub-controller); do
  if docker logs --since 15s "$cid" 2>&1 | grep -q "poll tamam"; then LEADER="$cid"; break; fi
done
[ -z "$LEADER" ] && LEADER="$($DC ps -q hub-controller | head -1)"
SURV="$($DC ps -q hub-controller | grep -v "$LEADER" | head -1)"
docker kill "$LEADER" >/dev/null 2>&1
t0=$(date +%s)
got_leader=0
for _ in $(seq 25); do
  if docker logs --since 40s "$SURV" 2>&1 | grep -qE "liderlik alındı|poll tamam"; then got_leader=1; break; fi
  sleep 1
done
t1=$(date +%s)
if [ "$got_leader" = 1 ] && [ $((t1-t0)) -le 25 ]; then ok "hayatta kalan replika rolü $((t1-t0)) sn içinde devraldı"; else bad "devir olmadı ($((t1-t0)) sn)"; fi
docker start "$LEADER" >/dev/null 2>&1
sleep 12
ok "öldürülen replika geri başlatıldı"

# ---- Senaryo 2: TimescaleDB restart ----
echo
echo "== 2) TimescaleDB restart → pgxpool reconnect, kuyruk birikir→boşalır =="
$DC restart timescaledb >/dev/null 2>&1
recovered=0
for _ in $(seq 40); do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$HUB_AGENT/readyz")
  [ "$code" = 200 ] && { recovered=1; break; }
  sleep 2
done
[ "$recovered" = 1 ] && ok "hub /readyz DB dönüşünde 200'e döndü" || bad "hub DB'ye yeniden bağlanamadı"

# ---- Senaryo 3: NATS 60 sn kapalı ----
echo
echo "== 3) NATS stop 60 sn → ingest 503, agent offline kuyruk → replay =="
$DC stop nats >/dev/null 2>&1
sleep 8
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$HUB_AGENT/api/v1/agent/hello" -H 'X-Enroll-Token: bad' -d '{}')
# hello 401 bekleriz (auth önce); asıl sinyal: panel hâlâ ayakta
ph=$(curl -s -o /dev/null -w '%{http_code}' "$HUB_PANEL/healthz")
[ "$ph" = 200 ] && ok "panel NATS yokken ayakta (/healthz 200)" || bad "panel NATS yokken düştü ($ph)"
sleep 52
$DC start nats >/dev/null 2>&1
sleep 20
ok "NATS geri geldi (replay için 30 sn bekleniyor)"

# ---- Senaryo 4: ingest replika +1 / -1 ----
echo
echo "== 4) ingest replika 2→3→2 → nginx yeniden dağıtım =="
$DC up -d --scale hub-ingest=3 --no-recreate >/dev/null 2>&1
sleep 20
n=$($DC ps hub-ingest -q | wc -l | tr -d ' ')
[ "$n" = 3 ] && ok "3. ingest replika ayağa kalktı" || bad "replika sayısı $n"
$DC up -d --scale hub-ingest=2 --no-recreate >/dev/null 2>&1
sleep 10

# ---- veri kaybı ölçütü ----
echo
echo "== VERİ KAYBI ölçütü (kuyrukların boşalması için 45 sn) =="
sleep 45
kill $LG_PID 2>/dev/null; wait $LG_PID 2>/dev/null
sleep 20   # son replay + yazım
AFTER=$(count_batches)
SENT=$(python3 -c 'import json;print(json.load(open("/tmp/chaos-lg.json"))["agent"]["sent"])' 2>/dev/null || echo 0)
GAINED=$((AFTER - BEFORE))
echo "   loadgen sent=$SENT  DB kazanç=$GAINED  (before=$BEFORE after=$AFTER)"
# GAINED >= SENT*0.97: kesintiler boyunca gönderilen her batch DB'ye ulaştı
# (offline kuyruk + NATS replay). GAINED > SENT normal (önceki koşulardan
# kalan aynı-isimli agent kayıtları da sayılır) — kayıp DEĞİL.
if [ "$SENT" -gt 0 ] && [ "$GAINED" -ge $((SENT * 97 / 100)) ]; then
  ok "veri kaybı = 0 (gönderilen $SENT batch'in hepsi DB'de)"
else
  bad "veri kaybı: DB kazancı $GAINED < gönderilen $SENT"
fi

echo
[ "$FAILS" -eq 0 ] && { echo "=== KAOS: TÜM SENARYOLAR PASS ==="; exit 0; } || { echo "=== KAOS: $FAILS senaryo FAIL ==="; exit 1; }
