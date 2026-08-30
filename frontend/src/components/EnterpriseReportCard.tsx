import { useState } from 'react'

const RANGES = [
  { label: '7 gün', days: 7 },
  { label: '30 gün', days: 30 },
  { label: '90 gün', days: 90 },
]

export function EnterpriseReportCard() {
  const [days, setDays] = useState(30)

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex rounded-lg border border-slate-700/80 p-0.5">
          {RANGES.map((r) => (
            <button
              key={r.days}
              onClick={() => setDays(r.days)}
              className={`rounded-md px-3 py-1 text-xs font-medium transition ${
                days === r.days ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
        <a
          href={`/api/report?type=enterprise&days=${days}`}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-3.5 py-1.5 text-sm font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          HTML Görüntüle
        </a>
      </div>
      <p className="text-xs text-slate-500">
        SLA (agent online oranı, cihaz poll sağlığı, paket düşme oranı) + kapasite/banding (p50/p95/p99 verim, dönemsel büyüme) +
        en yoğun uç noktalar/süreçler + uyarı sayaçları. Tarayıcıdan yazdırılabilir (Ctrl/Cmd+P → PDF).
      </p>
    </div>
  )
}
