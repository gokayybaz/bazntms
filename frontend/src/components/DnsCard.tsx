import type { DomainStat } from '../types'
import { formatNum } from '../lib/format'

export function DnsCard({ domains }: { domains: DomainStat[] }) {
  if (domains.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-slate-600">
        Henüz sorgu yok — yakalama başlatınca UDP/53 trafiği burada listelenir.
      </p>
    )
  }

  const max = Math.max(1, ...domains.map((d) => d.queries))

  return (
    <div className="max-h-80 overflow-y-auto">
      <ul className="space-y-2">
        {domains.map((d) => (
          <li key={d.domain} className="flex items-center gap-3 text-sm">
            <span className="w-56 shrink-0 truncate font-medium text-slate-200" title={d.domain}>
              {d.domain}
            </span>
            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
              <div
                className="h-full rounded-full bg-sky-500"
                style={{ width: `${Math.max(2, (d.queries / max) * 100)}%` }}
              />
            </div>
            <span className="w-16 text-right font-mono text-xs text-sky-300">{formatNum(d.queries)}</span>
            <span className="w-28 truncate text-right font-mono text-[10px] text-slate-600" title={d.ips?.join(', ')}>
              {d.ips?.length ? d.ips.slice(0, 2).join(', ') : '—'}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}
