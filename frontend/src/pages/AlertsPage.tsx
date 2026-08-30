import type { AlertEvent } from '../types'
import { formatNum } from '../lib/format'
import { AlertsCard } from '../components/AlertsCard'
import { Card } from '../components/Card'

export function AlertsPage({ alertEvents }: { alertEvents: AlertEvent[] }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyarılar</h1>
        <span className="text-xs text-slate-500">olay akışı + eşik ayarları + bildirim kanalları</span>
      </div>

      <Card title="Uyarılar" right={<span className="text-xs text-slate-500">{formatNum(alertEvents.length)} olay</span>}>
        <AlertsCard events={alertEvents} />
      </Card>
    </div>
  )
}
