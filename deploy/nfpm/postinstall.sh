#!/bin/sh
# nfpm postinstall (deb + rpm ortak) — systemd birimini yenile/etkinlestir.
#
# Zaten bir config varsa (yukseltme / yeniden kurulum): dokunma, sadece
# etkinlestir. Yoksa (ilk kurulum) VE gercek bir terminale bagliysak
# (`dpkg -i`/`rpm -ivh` interaktif calistirildi — otomasyon/CI degil):
# Hub URL/Enroll Token/Site sorulur, Windows MSI ve macOS .pkg
# sihirbazlarinin Linux esdegeri. Debian'in "debconf" sistemi
# kullanilmadi (rpm tarafinda esdegeri yok, iki paket formati icin ayri
# mekanizma gerektirirdi); bunun yerine POSIX `read` ile basit, her iki
# formatta da ayni sekilde calisan bir terminal istemi kullanildi.
#
# Terminal yoksa (Ansible/cloud-init/CI gibi otomasyonla kuruldu) ya da
# kullanici bos gecerse: sessizce eskisi gibi bos bir agent.yml yazip
# elle doldurmasi icin yonlendirilir — kurulum ASLA bu adimdan basarisiz
# olmaz.
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable bazntms-agent.service || true
fi

if [ -f /etc/bazntms/agent.yml ]; then
    systemctl restart bazntms-agent.service 2>/dev/null || true
    echo "bazNTMS agent guncellendi (mevcut /etc/bazntms/agent.yml korundu)."
    exit 0
fi

HUBURL=""
ENROLLTOKEN=""
SITE=""

if [ -t 0 ] && [ -t 1 ]; then
    echo ""
    echo "bazNTMS agent kuruldu. Sunucu bilgisini simdi girebilirsiniz"
    echo "(bos gecip sonra /etc/bazntms/agent.yml'i elle de doldurabilirsiniz):"
    printf "  Hub adresi (URL): "
    read -r HUBURL || true
    printf "  Kayit (enroll) belirteci: "
    read -r ENROLLTOKEN || true
    printf "  Site / konum adi: "
    read -r SITE || true
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
chmod 644 /etc/bazntms/agent.yml

if [ -n "$HUBURL" ]; then
    systemctl start bazntms-agent.service 2>/dev/null || true
    echo "bazNTMS agent kuruldu ve baslatildi (hub: ${HUBURL})."
else
    echo "bazNTMS agent kuruldu. /etc/bazntms/agent.yml dosyasini doldurup 'systemctl start bazntms-agent' ile baslatin."
fi
exit 0
