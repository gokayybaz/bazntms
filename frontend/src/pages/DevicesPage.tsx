import { DevicesCard } from '../components/DevicesCard'
import { FlowsCard } from '../components/FlowsCard'
import { SyslogCard } from '../components/SyslogCard'
import { Card } from '../components/Card'

export function DevicesPage({ refreshKey }: { refreshKey: number }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Cihazlar</h1>
        <span className="text-xs text-slate-500">SNMP/FortiGate cihazları · NetFlow · Syslog</span>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Cihazlar (SNMP)" right={<span className="text-xs text-slate-500">router · switch · firewall · ap</span>}>
          <DevicesCard refreshKey={refreshKey} />
        </Card>
        <div className="space-y-4">
          <Card title="NetFlow v5 Akışları" right={<span className="text-xs text-slate-500">son 15 dk · top 20</span>}>
            <FlowsCard />
          </Card>
          <Card title="Syslog Olayları" right={<span className="text-xs text-slate-500">RFC3164</span>}>
            <SyslogCard />
          </Card>
        </div>
      </div>
    </div>
  )
}
