import type { Snapshot } from '../types'
import { formatBits, formatBytes, formatNum } from '../lib/format'

export function StatCards({ stats }: { stats: Snapshot }) {
  const cards = [
    {
      label: 'İndirilen Hız',
      value: formatBits(stats.bps_in),
      sub: 'gelen trafik',
      accent: 'border-l-cyan-500',
      valueCls: 'text-cyan-300',
    },
    {
      label: 'Gönderilen Hız',
      value: formatBits(stats.bps_out),
      sub: 'giden trafik',
      accent: 'border-l-violet-500',
      valueCls: 'text-violet-300',
    },
    {
      label: 'Toplam Veri',
      value: formatBytes(stats.total_bytes),
      sub: `${formatNum(stats.total_packets)} paket`,
      accent: 'border-l-emerald-500',
      valueCls: 'text-emerald-300',
    },
    {
      label: 'Paket Hızı',
      value: `${formatNum(stats.pps)} pps`,
      sub:
        stats.dropped > 0
          ? `${formatNum(stats.dropped)} paket düştü`
          : 'kayıp yok',
      accent: 'border-l-amber-500',
      valueCls: 'text-amber-300',
    },
  ]

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {cards.map((c) => (
        <div key={c.label} className={`rounded-md border border-slate-800 border-l-2 bg-slate-900/70 p-4 ${c.accent}`}>
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">{c.label}</p>
          <p className={`mt-1 font-mono text-2xl font-bold ${c.valueCls}`}>{c.value}</p>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-600">{c.sub}</p>
        </div>
      ))}
    </div>
  )
}
