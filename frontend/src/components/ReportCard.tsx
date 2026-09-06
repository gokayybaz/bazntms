import { useState } from 'react'
import { RangeTabs } from './RangeTabs'

const RANGES = [
  { label: '24 saat', value: 1 },
  { label: '7 gün', value: 7 },
  { label: '30 gün', value: 30 },
] as const

const linkCls =
  'border border-rx/40 bg-rx/10 px-2.5 py-0.5 font-mono text-[11px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20'

export function ReportCard() {
  const [days, setDays] = useState<1 | 7 | 30>(7)

  return (
    <div className="space-y-3 font-mono">
      <div className="flex flex-wrap items-center gap-2">
        <RangeTabs ranges={RANGES} value={days} onChange={setDays} />
        <a href={`/api/report?days=${days}&format=html`} target="_blank" rel="noreferrer" className={linkCls}>
          HTML Görüntüle
        </a>
        <a href={`/api/report?days=${days}&format=pdf`} className="border border-rx bg-rx px-2.5 py-0.5 text-[11px] font-bold uppercase tracking-[0.04em] text-ground transition hover:opacity-90">
          PDF İndir
        </a>
      </div>
      <p className="text-[11px] leading-relaxed text-tui-dim">
        Kaynak: agent arayüz telemetrisi + NetFlow + agent süreç trafiği (hub'ın yerel yakalaması değil). Rapor içeriği:
        yönetici özeti, günlük trafik grafiği, agent filosu, en yoğun uç noktalar (ülke/ASN), süreç bazlı trafik, protokol
        dağılımı ve uyarı olayları. HTML sürümü tarayıcıdan da yazdırılabilir (Ctrl/Cmd+P → PDF).
      </p>
    </div>
  )
}
