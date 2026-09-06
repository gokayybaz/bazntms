import { DevicesCard } from '../components/DevicesCard'
import { FlowsCard } from '../components/FlowsCard'
import { SyslogCard } from '../components/SyslogCard'
import { Panel } from '../components/Panel'

export function DevicesPage({ refreshKey }: { refreshKey: number }) {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-3 py-3">
      <div className="flex items-baseline gap-2 font-mono">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Cihazlar</h1>
        <span className="text-[10px] text-tui-dim">SNMP/FortiGate cihazları · NetFlow · Syslog</span>
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <Panel title="Cihazlar (SNMP)" right={<span className="font-mono text-[10px] text-tui-dim">router · switch · firewall · ap</span>}>
          <DevicesCard refreshKey={refreshKey} />
        </Panel>
        <div className="space-y-3">
          <Panel title="NetFlow v5 Akışları" right={<span className="font-mono text-[10px] text-tui-dim">son 15 dk · top 20</span>}>
            <FlowsCard />
          </Panel>
          <Panel title="Syslog Olayları" right={<span className="font-mono text-[10px] text-tui-dim">RFC3164</span>}>
            <SyslogCard />
          </Panel>
        </div>
      </div>
    </div>
  )
}
