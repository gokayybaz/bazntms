#!/usr/bin/env bash
# mac-udp-relay.sh — Docker Desktop for Mac UDP iletim kısıtı için geçici çözüm.
#
# Sorun: Docker Desktop, Mac'in fiziksel LAN arayüzüne başka bir host'tan (router,
# switch, firewall) gelen UDP datagramlarını published container portuna İLETMEZ;
# yalnızca 127.0.0.1'den geleni iletir. Bu yüzden hub-controller NetFlow/syslog
# portları docker-compose.scale.yml'de loopback'e kaydırılmış numaralarla bağlıdır
# (127.0.0.1:12055 -> konteyner 2055, 127.0.0.1:15514 -> konteyner 5514).
#
# Bu script, Mac'te native bir süreç olarak gerçek portları (2055/5514) dinler ve
# her paketi loopback'teki kaydırılmış porta iletir; oradan Docker konteynere sokar:
#
#   router --LAN--> Mac:2055  --[socat]-->  127.0.0.1:12055  --[Docker]-->  konteyner:2055
#
# Kullanım:
#   deploy/scripts/mac-udp-relay.sh          # ön planda, Ctrl+C ile durdurun
#   deploy/scripts/mac-udp-relay.sh &        # arka planda
#
# Not: socat paketi kendi IP'sinden (127.0.0.1) yeniden gönderir; konteyner
# source_ip'yi 127.0.0.1 görür. Cihaz eşleştirmesi SNMP sys_name / syslog
# hostname üzerinden yürür (bkz. DeviceDetailPage deviceSyslog filtresi).
#
# Linux'ta gerek yok — orada compose'da doğrudan "2055:2055/udp" kullanın.
set -euo pipefail

# gerçek port -> loopback hedef port
RELAYS=(
	"2055:12055"   # NetFlow v5
	"5514:15514"   # syslog
)

if ! command -v socat >/dev/null 2>&1; then
	echo "socat bulunamadı — kurun:  brew install socat" >&2
	exit 1
fi

pids=()
cleanup() {
	echo
	echo "röle durduruluyor..."
	for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
	wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

for spec in "${RELAYS[@]}"; do
	src="${spec%%:*}"
	dst="${spec##*:}"
	if lsof -nP -iUDP:"$src" >/dev/null 2>&1; then
		echo "uyarı: UDP $src zaten bir süreç tarafından kullanılıyor — atlanıyor" >&2
		continue
	fi
	# UDP4-RECV: peer-bağımsız, tek yönlü — NetFlow/syslog gibi çok kaynaklı
	# tek yön akışlar için fork'suz ve durumsuz.
	socat -u "UDP4-RECV:${src}" "UDP4-SENDTO:127.0.0.1:${dst}" &
	pids+=($!)
	echo "röle: Mac:${src}/udp  ->  127.0.0.1:${dst}/udp  (pid $!)"
done

if [ "${#pids[@]}" -eq 0 ]; then
	echo "hiçbir röle başlatılamadı" >&2
	exit 1
fi

echo "hazır — router NetFlow/syslog hedefini  <Mac-LAN-IP>:2055 / :5514  olarak ayarlayın."
echo "durdurmak için Ctrl+C."
wait
