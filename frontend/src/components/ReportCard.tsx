import { useState } from 'react'

const RANGES = [
  { label: '24 saat', days: 1 },
  { label: '7 gün', days: 7 },
  { label: '30 gün', days: 30 },
]

export function ReportCard() {
  const [days, setDays] = useState(7)

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
          href={`/api/report?days=${days}&format=html`}
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-3.5 py-1.5 text-sm font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          HTML Görüntüle
        </a>
        <a
          href={`/api/report?days=${days}&format=pdf`}
          className="rounded-lg bg-cyan-600 px-3.5 py-1.5 text-sm font-semibold text-white transition hover:brightness-110"
        >
          PDF İndir
        </a>
      </div>
      <p className="text-xs text-slate-500">
        Rapor içeriği: yönetici özeti, günlük trafik grafiği, en yoğun hedefler (ülke/ASN), en aktif süreçler, DNS sorguları,
        protokol dağılımı, uyarı olayları ve son AI analizleri. HTML sürümü tarayıcıdan da yazdırılabilir (Ctrl/Cmd+P → PDF).
      </p>
    </div>
  )
}
