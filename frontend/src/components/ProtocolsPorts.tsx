import type { PortStat, Snapshot } from '../types'
import { formatBytes, formatNum } from '../lib/format'

const PROTO_COLORS: Record<string, string> = {
  TCP: 'bg-cyan-500',
  UDP: 'bg-violet-500',
  ICMP: 'bg-amber-500',
  ICMPv6: 'bg-amber-500',
  other: 'bg-slate-500',
}

export function ProtocolsCard({ stats }: { stats: Snapshot }) {
  const entries = Object.entries(stats.protocols)
  const total = entries.reduce((a, [, v]) => a + v, 0)

  if (total === 0) {
    return <p className="py-6 text-center text-sm text-slate-600">Henüz veri yok.</p>
  }

  const sorted = entries.sort((a, b) => b[1] - a[1]).slice(0, 8)

  return (
    <div>
      <div className="flex h-3 overflow-hidden rounded-full bg-slate-800">
        {sorted.map(([name, count]) => (
          <div
            key={name}
            className={PROTO_COLORS[name] ?? PROTO_COLORS.other}
            style={{ width: `${(count / total) * 100}%` }}
            title={`${name}: ${count}`}
          />
        ))}
      </div>
      <ul className="mt-3 space-y-1.5">
        {sorted.map(([name, count]) => (
          <li key={name} className="flex items-center gap-2 text-sm">
            <span className={`size-2 rounded-sm ${PROTO_COLORS[name] ?? PROTO_COLORS.other}`} />
            <span className="font-medium text-slate-300">{name}</span>
            <span className="ml-auto font-mono text-xs text-slate-400">{formatNum(count)}</span>
            <span className="w-12 text-right font-mono text-xs text-slate-600">
              %{Math.round((count / total) * 100)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

export function PortsCard({ ports }: { ports: PortStat[] }) {
  const max = Math.max(1, ...ports.map((p) => p.bytes))

  if (ports.length === 0) {
    return <p className="py-6 text-center text-sm text-slate-600">Henüz veri yok.</p>
  }

  return (
    <ul className="space-y-2">
      {ports.slice(0, 10).map((p) => (
        <li key={p.port} className="flex items-center gap-3 text-sm">
          <span className="w-14 shrink-0 font-mono font-semibold text-slate-200">{p.port}</span>
          <span className="w-24 shrink-0 truncate text-xs text-slate-500" title={p.name}>
            {p.name || '—'}
          </span>
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
            <div
              className="h-full rounded-full bg-emerald-500"
              style={{ width: `${Math.max(2, (p.bytes / max) * 100)}%` }}
            />
          </div>
          <span className="w-20 text-right font-mono text-xs text-slate-300">{formatBytes(p.bytes)}</span>
          <span className="w-16 text-right font-mono text-[10px] text-slate-600">{formatNum(p.packets)} pk</span>
        </li>
      ))}
    </ul>
  )
}
