#!/bin/sh
# nfpm postremove — servis ve kalinti state dosyalarini temizler.
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop bazntms-agent.service 2>/dev/null || true
    systemctl disable bazntms-agent.service 2>/dev/null || true
    systemctl daemon-reload || true
fi
rm -f /var/lib/bazntms/agent.state.json /var/lib/bazntms/agent.state.json.queue.jsonl
echo "bazNTMS agent kaldirildi (state dosyalari temizlendi)."
exit 0
