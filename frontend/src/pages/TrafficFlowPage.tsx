import { Panel } from '../components/Panel'
import { TrafficFlowCard } from '../components/TrafficFlowCard'

export function TrafficFlowPage() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Canlı Akış</h1>
        <span className="text-[10px] text-tui-dim">agent filosu → router/güvenlik duvarı → internet · animasyonlu paket akışı</span>
      </div>

      <Panel>
        <TrafficFlowCard />
      </Panel>
    </div>
  )
}
