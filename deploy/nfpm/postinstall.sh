#!/bin/sh
# nfpm postinstall — systemd birimini yenile ve etkinlestir (baslatma;
# konfigurasyon doldurulmadan calismasin).
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable bazntms-agent.service || true
fi
echo "bazNTMS agent kuruldu. Konfigurasyon: /etc/bazntms/agent.yml"
echo "Baslatmak icin: cp /etc/bazntms/agent.yml.example /etc/bazntms/agent.yml && systemctl start bazntms-agent"
exit 0
