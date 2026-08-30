import { TopologyCard } from '../components/TopologyCard'
import { Card } from '../components/Card'

export function TopologyPage({ refreshKey }: { refreshKey: number }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Ağ Topolojisi</h1>
        <span className="text-xs text-slate-500">LLDP/CDP/ARP keşfi + agent subnetleri</span>
      </div>

      <Card>
        <TopologyCard refreshKey={refreshKey} />
      </Card>
    </div>
  )
}
