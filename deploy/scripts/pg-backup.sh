#!/bin/sh
# bazNTMS PostgreSQL/TimescaleDB yedekleme (Faz 5.4 DR)
# Kullanim: pg-backup.sh <postgres-dsn> <cikti-dizini> [sakinca-suresi-gun]
# Ornek (cron, gunluk 02:00): 0 2 * * * /opt/bazntms/pg-backup.sh "postgres://bazntms:..." /var/backups/bazntms
set -eu

DSN="${1:?kullanim: pg-backup.sh <dsn> <cikti-dizini> [retention-gun]}"
OUT="${2:-/var/backups/bazntms}"
RETENTION_DAYS="${3:-14}"

STAMP="$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUT"

echo "[backup] custom format dump basladi"
pg_dump "$DSN" --format=custom --compress=6 \
  --file "$OUT/bazntms-$STAMP.dump"

# salt-metin yedek de tut (kismi geri yukleme/inceleme kolayligi)
pg_dump "$DSN" --schema-only \
  --file "$OUT/bazntms-$STAMP.schema.sql"

echo "[backup] vault master key hatirlatmasi: device secret'larini yedeklemek
icin vault.key dosyasini ayri guvenli kanaldan yedekleyin (bknz. DR runbook)."

# eski yedekleri temizle
find "$OUT" -name 'bazntms-*.dump' -mtime "+$RETENTION_DAYS" -delete
find "$OUT" -name 'bazntms-*.schema.sql' -mtime "+$RETENTION_DAYS" -delete

echo "[backup] tamam: $OUT/bazntms-$STAMP.dump"
