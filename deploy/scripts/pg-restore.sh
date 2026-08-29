#!/bin/sh
# bazNTMS PostgreSQL/TimescaleDB geri yukleme (Faz 5.4 DR)
# Kullanim: pg-restore.sh <yedek.dump> <postgres-dsn>
# Dikkat: hedef veri tabanindaki mevcut veriyi EZER — once durumu dogrulayin.
set -eu

DUMP="${1:?kullanim: pg-restore.sh <yedek.dump> <dsn>}"
DSN="${2:?kullanim: pg-restore.sh <yedek.dump> <dsn>}"

echo "[restore] mevcut baglantilari kesiliyor"
psql "$DSN" -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid();" >/dev/null

echo "[restore] dump geri yaziliyor: $DUMP"
pg_restore --dsn "$DSN" --clean --if-exists --no-owner "$DUMP"

echo "[restore] TimescaleDB hypertable/cagg kontrolu (timescaledb kuruluysa otomatik)"
echo "[restore] tamam — hub'i yeniden baslatin ve /readyz'i kontrol edin"
