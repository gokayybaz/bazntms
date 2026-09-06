import { TopologyCard } from '../components/TopologyCard'
import { Panel } from '../components/Panel'

export function TopologyPage({ refreshKey }: { refreshKey: number }) {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3">
      <div className="flex items-baseline gap-2 font-mono">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Ağ Topolojisi</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">LLDP/CDP/ARP keşfi + agent subnetleri</span>
      </div>

      <Panel>
        <TopologyCard refreshKey={refreshKey} />
      </Panel>
    </div>
  )
}
