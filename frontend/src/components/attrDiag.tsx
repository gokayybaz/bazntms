import type { ReactNode } from 'react'

// attrDiag — Süreç Trafiği / Uygulama Görünürlüğü (L7) / DNS panelleri boşken
// "neden boş" ipucu. Agent'ın bildirdiği süreç-atıf teşhisinden türetilir:
//   attr_method : ebpf | pcap | etw | off  (undefined → filo geneli / eski agent)
//   attr_iface  : pcap yakalama arayüzü (yalnız method=pcap)
//   attr_note   : motor kapalıysa insan-okur neden
//
// 2026-09-09: `iface=Tailscale` vakası — auto arayüz seçimi sanal/VPN
// adaptörünü yakalıyordu, motor "çalışıyor" görünüyor ama üç panel de boştu.
// Eski boş-durum metni yanıltıcıydı (artık işlevsiz `-pcap` / `collect.pcap`).

export interface AttrDiag {
  method?: string
  iface?: string
  note?: string
}

// attrEmptyHint, boş panelin altına konacak ipucu düğümünü verir. `what` panelin
// adıdır ("süreç trafiği" | "uygulama görünürlüğü (L7)" | "DNS").
export function attrEmptyHint({ method, iface, note }: AttrDiag, what: string): ReactNode {
  if (method === 'off') {
    return (
      <>
        Derin toplama bu agent'ta kapalı{note ? <> — {note}</> : null}. Açmak için{' '}
        <code className="text-tui-dim">agent.yml</code> içindeki{' '}
        <code className="text-tui-dim">collect.method</code> satırını kaldırın (varsayılan{' '}
        <code className="text-tui-dim">auto</code>); hub da <code className="text-tui-dim">-agent-pcap</code>{' '}
        ile başlatılmış olmalı (v1.3.0'dan beri varsayılan açık).
      </>
    )
  }
  if (method === 'pcap' && iface) {
    return (
      <>
        Atıf motoru <span className="text-tui-dim">pcap</span> modunda{' '}
        <span className="text-ink-hi">{iface}</span> arayüzünü dinliyor ama {what} verisi gelmiyor —
        sanal/VPN adaptörü (Tailscale, WireGuard, utun…) yakalanıyor olabilir. Fiziksel adaptörü açıkça
        verin: <code className="text-tui-dim">agent.yml</code> →{' '}
        <code className="text-tui-dim">collect.pcap_interface: "Ethernet"</code> (PowerShell:{' '}
        <code className="text-tui-dim">Get-NetAdapter</code>).
      </>
    )
  }
  if (method) {
    return (
      <>
        Atıf motoru <span className="text-tui-dim">{method}</span> modunda çalışıyor ama {what} verisi yok —
        seçilen aralıkta bu tür trafik gözlenmemiş olabilir.
      </>
    )
  }
  return (
    <>
      Derin toplama gerekir: agent <code className="text-tui-dim">collect.method: auto</code> (varsayılan) ile
      çalışmalı, hub <code className="text-tui-dim">-agent-pcap</code> politikası açık olmalı. Tek agent
      detay sayfasındaki "Atıf" rozeti aktif yöntemi ve varsa nedeni gösterir.
    </>
  )
}
