import { FullscreenCard } from '../components/FullscreenCard'
import { TrafficFlowCard } from '../components/TrafficFlowCard'

const HINT = 'agent filosu → switch/AP → router/güvenlik duvarı → internet · animasyonlu paket akışı'

export function TrafficFlowPage({ isAdmin = false }: { isAdmin?: boolean }) {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Canlı Akış</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">{HINT}</span>
      </div>

      <FullscreenCard title="Canlı Akış" hint={HINT} editable={isAdmin}>
        {(full, editing) => <TrafficFlowCard fill={full} editable={editing} />}
      </FullscreenCard>
    </div>
  )
}
