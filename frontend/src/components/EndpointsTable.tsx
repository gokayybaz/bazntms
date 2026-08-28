import type { EndpointStat } from '../types'
import { flagEmoji, formatBytes, formatNum } from '../lib/format'

export function EndpointsTable({ endpoints }: { endpoints: EndpointStat[] }) {
  const max = Math.max(1, ...endpoints.map((e) => e.total))

  if (endpoints.length === 0) {
    return <p className="py-8 text-center text-sm text-slate-600">Henüz veri yok — yakalama başlatın.</p>
  }

  return (
    <div className="overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
            <th className="pb-2 font-medium">Uç Nokta</th>
            <th className="pb-2 font-medium text-right">İndirme</th>
            <th className="pb-2 font-medium text-right">Gönderme</th>
            <th className="pb-2 pl-6 font-medium w-[38%]">Toplam</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800/60">
          {endpoints.slice(0, 15).map((e) => {
            const label = e.hostname || e.ip
            const flag = flagEmoji(e.country)
            return (
              <tr key={e.ip} className="group">
                <td className="py-2 pr-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <span
                      className={`mt-0.5 size-1.5 shrink-0 rounded-full ${e.local ? 'bg-emerald-400' : 'bg-slate-500'}`}
                      title={e.local ? 'yerel' : 'uzak'}
                    />
                    <div className="min-w-0">
                      <p className="truncate font-medium text-slate-200" title={e.asn ? `${e.ip} — ${e.asn}` : label}>
                        {flag && <span className="mr-1">{flag}</span>}
                        {label}
                        {e.local && (
                          <span className="ml-1.5 rounded bg-emerald-500/10 px-1 py-0.5 text-[9px] font-semibold uppercase text-emerald-400 ring-1 ring-emerald-500/30">
                            bu cihaz
                          </span>
                        )}
                      </p>
                      <p className="truncate text-[11px] text-slate-500">
                        {e.hostname || (e.asn ? e.asn : e.ip)}
                      </p>
                    </div>
                  </div>
                </td>
                <td className="py-2 text-right font-mono text-xs text-cyan-300/90">{formatBytes(e.in)}</td>
                <td className="py-2 pl-3 text-right font-mono text-xs text-violet-300/90">{formatBytes(e.out)}</td>
                <td className="py-2 pl-6">
                  <div className="flex items-center gap-2">
                    <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
                      <div
                        className="h-full rounded-full bg-cyan-500"
                        style={{ width: `${Math.max(2, (e.total / max) * 100)}%` }}
                      />
                    </div>
                    <span className="w-20 text-right font-mono text-xs text-slate-300">{formatBytes(e.total)}</span>
                    <span className="w-16 text-right font-mono text-[10px] text-slate-600">
                      {formatNum(e.packets)} pk
                    </span>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
