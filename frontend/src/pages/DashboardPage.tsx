import type { AlertEvent } from '../types'
import type { FleetSummary } from '../lib/useLive'
import { Overview } from '../components/Overview'

export function DashboardPage({
  refreshKey,
  alertEvents,
  fleet,
}: {
  refreshKey: number
  alertEvents: AlertEvent[]
  fleet: FleetSummary | null
}) {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3">
      <div className="flex items-baseline gap-2 font-mono">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Genel Bakış</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">agent filosu + cihaz telemetrisi · tek ekran</span>
      </div>
      <Overview refreshKey={refreshKey} alertEvents={alertEvents} fleet={fleet} />
    </div>
  )
}
