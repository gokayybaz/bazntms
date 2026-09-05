import { useMemo } from 'react'
import type { AlertEvent } from '../types'
import { formatNum } from '../lib/format'
import { AlertsCard } from '../components/AlertsCard'
import { KIND_LABELS, KIND_STYLES } from '../lib/alertKinds'
import { Card } from '../components/Card'

export function AlertsPage({ alertEvents }: { alertEvents: AlertEvent[] }) {
  const byKind = useMemo(() => {
    const counts = new Map<string, number>()
    for (const e of alertEvents) counts.set(e.kind, (counts.get(e.kind) ?? 0) + 1)
    return [...counts.entries()].sort((a, b) => b[1] - a[1])
  }, [alertEvents])

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyarılar</h1>
        <span className="text-xs text-dim-aa">olay akışı + eşik ayarları + bildirim kanalları</span>
      </div>

      {byKind.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {byKind.map(([kind, count]) => (
            <span
              key={kind}
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium ring-1 ${KIND_STYLES[kind] ?? 'bg-slate-500/10 text-slate-400 ring-slate-500/20'}`}
            >
              {KIND_LABELS[kind] ?? kind}
              <span className="font-mono font-semibold">{formatNum(count)}</span>
            </span>
          ))}
        </div>
      )}

      <Card title="Uyarılar" right={<span className="text-xs text-dim-aa">{formatNum(alertEvents.length)} olay</span>}>
        <AlertsCard events={alertEvents} />
      </Card>
    </div>
  )
}
