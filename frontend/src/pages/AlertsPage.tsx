import { useMemo } from 'react'
import type { AlertEvent } from '../types'
import { formatNum } from '../lib/format'
import { AlertsCard } from '../components/AlertsCard'
import { KIND_LABELS, KIND_STYLES } from '../lib/alertKinds'
import { Panel } from '../components/Panel'

export function AlertsPage({ alertEvents }: { alertEvents: AlertEvent[] }) {
  const byKind = useMemo(() => {
    const counts = new Map<string, number>()
    for (const e of alertEvents) counts.set(e.kind, (counts.get(e.kind) ?? 0) + 1)
    return [...counts.entries()].sort((a, b) => b[1] - a[1])
  }, [alertEvents])

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex flex-wrap items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyarılar</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">olay akışı + eşik ayarları + bildirim kanalları</span>
      </div>

      {byKind.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {byKind.map(([kind, count]) => (
            <span
              key={kind}
              className={`inline-flex items-center gap-1.5 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] ${
                KIND_STYLES[kind] ?? 'border-rule-hi text-tui-dim'
              }`}
            >
              {KIND_LABELS[kind] ?? kind}
              <span className="font-bold">{formatNum(count)}</span>
            </span>
          ))}
        </div>
      )}

      <Panel title="Uyarılar" right={<span className="text-[10px] text-tui-dim">{formatNum(alertEvents.length)} olay</span>}>
        <AlertsCard events={alertEvents} />
      </Panel>
    </div>
  )
}
