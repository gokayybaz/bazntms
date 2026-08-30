import type { AlertEvent } from '../types'
import { Overview } from '../components/Overview'

export function DashboardPage({ refreshKey, alertEvents }: { refreshKey: number; alertEvents: AlertEvent[] }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Genel Bakış</h1>
        <span className="text-xs text-slate-500">agent filosu + cihaz telemetrisi tek ekranda</span>
      </div>
      <Overview refreshKey={refreshKey} alertEvents={alertEvents} />
    </div>
  )
}
