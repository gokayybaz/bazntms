import { useState } from 'react'
import { RangeTabs } from './RangeTabs'

const RANGES = [
  { label: '7 gün', value: 7 },
  { label: '30 gün', value: 30 },
  { label: '90 gün', value: 90 },
] as const

export function EnterpriseReportCard() {
  const [days, setDays] = useState<7 | 30 | 90>(30)

  return (
    <div className="space-y-3 font-mono">
      <div className="flex flex-wrap items-center gap-2">
        <RangeTabs ranges={RANGES} value={days} onChange={setDays} />
        <a
          href={`/api/report?type=enterprise&days=${days}`}
          target="_blank"
          rel="noreferrer"
          className="border border-rx/40 bg-rx/10 px-2.5 py-0.5 text-[11px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20"
        >
          HTML Görüntüle
        </a>
      </div>
      <p className="text-[11px] leading-relaxed text-tui-dim">
        SLA (agent online oranı, cihaz poll sağlığı, SNMP arayüz iskarta/hata) + kapasite/banding (p50/p95/p99 verim, dönemsel
        büyüme) + en yoğun uç noktalar/süreçler + uyarı sayaçları. Veri penceresi filo ham verisinin saklama süresiyle sınırlıdır.
        Tarayıcıdan yazdırılabilir (Ctrl/Cmd+P → PDF).
      </p>
    </div>
  )
}
