#!/bin/sh
# nfpm postinstall (deb + rpm ortak) — systemd birimini yenile/etkinlestir.
#
# Zaten bir config varsa (yukseltme / yeniden kurulum): dokunma, sadece
# etkinlestir. Yoksa (ilk kurulum) VE gercek bir denetim terminali
# (controlling tty) varsa: Hub URL/Enroll Token/Site sorulur, Windows MSI
# ve macOS .pkg sihirbazlarinin Linux esdegeri.
#
# ONEMLI: stdin degil, DOGRUDAN /dev/tty kullanilir. Gercek Docker
# konteynerinde canli test edildi: dpkg'nin postinst'i normal stdin'i
# terminale bagli GOSTERIR, ama RPM'in %post'u ASLA gostermez (rpm
# scriptlet'leri stdin'i her zaman yeniden yonlendirir — paket
# formatlarindan bagimsiz, klasik Unix cozumu: denetim terminaline
# /dev/tty ile dogrudan eris). Bu yuzden [ -t 0 ] yerine /dev/tty'nin
# acilip acilamadigina bakiliyor — ikisinde de dogru calisan tek yol bu.
#
# Terminal yoksa (Ansible/cloud-init/CI/Docker --build gibi otomasyon)
# ya da kullanici bos gecerse: sessizce eskisi gibi bos bir agent.yml
# yazip elle doldurmasi icin yonlendirilir — kurulum ASLA bu adimdan
# basarisiz olmaz.
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable bazntms-agent.service || true
fi

if [ -f /etc/bazntms/agent.yml ]; then
    # eski surumlerden 0644 kalmis olabilir — enroll token dunyaya acik
    # kalmasin (B4). Icerige dokunmadan izinleri sikilastir.
    chown root:root /etc/bazntms/agent.yml 2>/dev/null || true
    chmod 600 /etc/bazntms/agent.yml 2>/dev/null || true
    systemctl restart bazntms-agent.service 2>/dev/null || true
    echo "bazNTMS agent guncellendi (mevcut /etc/bazntms/agent.yml korundu)."
    exit 0
fi

HUBURL=""
ENROLLTOKEN=""
SITE=""

# ONEMLI: bare `exec 3<>/dev/tty` basarisiz olursa POSIX kurallarina gore
# ("redirection error" non-interaktif shell'i DERHAL sonlandirir) `set -e`
# ve if-kosulu muafiyeti bile devreye girmeden TUM kurulumu (dpkg exit 2)
# dusuruyordu — Docker'da canli dogrulandi. Once bir ALT KABUKTA (subshell)
# guvenle test edip, YALNIZCA basariliysa ana kabukta gercek exec'i
# yapiyoruz — boylece basarisizlik alt kabukla sinirli kalip normal bir
# "false" komutu gibi if tarafindan yakalaniyor.
if (: <>/dev/tty) 2>/dev/null; then
    exec 3<>/dev/tty
    echo "" >&3
    echo "bazNTMS agent kuruldu. Sunucu bilgisini simdi girebilirsiniz" >&3
    echo "(bos gecip sonra /etc/bazntms/agent.yml'i elle de doldurabilirsiniz):" >&3
    printf "  Hub adresi (URL): " >&3
    read -r HUBURL <&3 || true
    printf "  Kayit (enroll) belirteci: " >&3
    read -r ENROLLTOKEN <&3 || true
    printf "  Site / konum adi: " >&3
    read -r SITE <&3 || true
    exec 3>&-
fi

# yaml_quote: kullanicinin girdigi serbest metni YAML tek-tirnakli skaler
# olarak guvenli hale getirir (icindeki '#'/':' vb. YAML'i bozmasin diye)
# — YAML kurali: gomulu tek tirnak ikiye katlanir.
yaml_quote() {
    printf "'%s'" "$(printf '%s' "$1" | sed "s/'/''/g")"
}

mkdir -p /etc/bazntms
cat > /etc/bazntms/agent.yml <<EOF
# bazNTMS agent yapilandirmasi (kurulum sirasinda uretildi).
hub:
  url: $(yaml_quote "$HUBURL")
  token: $(yaml_quote "$ENROLLTOKEN")

agent:
  name:
  site: $(yaml_quote "$SITE")

collect:
  interval_seconds: 30
  pcap: false
  pcap_interface: auto
  pcap_record: false
  pcap_dir: /var/lib/bazntms/captures

log:
  level: info
  format: json
EOF
# agent.yml enroll token'i tasir → yalnizca sahibi (root; servis User=root)
# okuyabilsin. Dunya/grup okumasi kapali (B4).
chown root:root /etc/bazntms/agent.yml
chmod 600 /etc/bazntms/agent.yml

if [ -n "$HUBURL" ]; then
    systemctl start bazntms-agent.service 2>/dev/null || true
    echo "bazNTMS agent kuruldu ve baslatildi (hub: ${HUBURL})."
else
    echo "bazNTMS agent kuruldu. /etc/bazntms/agent.yml dosyasini doldurup 'systemctl start bazntms-agent' ile baslatin."
fi
exit 0
