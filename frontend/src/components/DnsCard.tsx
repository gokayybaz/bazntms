import { useMemo, useState } from 'react'
import { formatNum } from '../lib/format'
import { usePolledJson } from '../lib/usePolledJson'
import { PanelState } from './PanelState'
import { RangeTabs } from './RangeTabs'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface DnsRow {
  domain: string
  process: string
  queries: number
  responses: number
  agent_count: number
}

const RANGES = [
  { label: '15 dk', value: 15 },
  { label: '1 saat', value: 60 },
  { label: '6 saat', value: 360 },
] as const

export function DnsCard({ agentId }: { agentId?: number } = {}) {
  const [minutes, setMinutes] = useState<15 | 60 | 360>(60)
  const { data, loaded } = usePolledJson<DnsRow[]>(
    `/api/v1/dns?minutes=${minutes}&limit=30${agentId ? `&agent_id=${agentId}` : ''}`,
    15_000,
  )
  const rows = useMemo(() => (Array.isArray(data) ? data : []), [data])

  const maxHits = useMemo(() => Math.max(1, ...rows.map((r) => r.queries + r.responses)), [rows])

  const cols: TuiColumn<DnsRow>[] = [
    { key: 'domain', header: 'Alan Adı', sortable: true, render: (r) => <span className="text-ink-hi">{r.domain}</span> },
    { key: 'process', header: 'Süreç', sortable: true, render: (r) => <span className="text-tui-dim">{r.process || '—'}</span> },
    {
      key: 'hits',
      header: 'Sorgu/Yanıt',
      width: '26%',
      sortable: true,
      sortValue: (r) => r.queries + r.responses,
      render: (r) => (
        <span className="flex items-center gap-2">
          <span className="h-1.5 flex-1 bg-panel-2">
            <span className="block h-full bg-amber-500" style={{ width: `${Math.max(3, ((r.queries + r.responses) / maxHits) * 100)}%` }} />
          </span>
          <span className="w-16 shrink-0 text-right">
            {formatNum(r.queries)}/{formatNum(r.responses)}
          </span>
        </span>
      ),
    },
    { key: 'agent_count', header: 'Agent', align: 'right', sortable: true, sortValue: (r) => r.agent_count, render: (r) => <span className="text-tui-dim">{r.agent_count}</span> },
  ]

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <RangeTabs ranges={RANGES} value={minutes} onChange={setMinutes} />
        <span className="ml-auto font-mono text-[10px] text-tui-dim">UDP/53 · süreç atıflı · agent'ta -pcap açık olmalı</span>
      </div>

      {!loaded ? (
        <PanelState kind="loading" />
      ) : rows.length === 0 ? (
        <PanelState
          kind="empty"
          message="Henüz DNS görünürlüğü verisi yok."
          className="mx-auto max-w-md"
          hint={
            <>
              Agent'ta <code className="text-tui-dim">collect.pcap</code> ve hub'da{' '}
              <code className="text-tui-dim">-agent-pcap</code> açık olmalı. Süreç trafiği doluyor ama DNS boşsa:
              sorgular yakalanan arayüzden geçmiyordur — DoH/DoT ya da VPN/Tailscale MagicDNS (ayrı{' '}
              <code className="text-tui-dim">utun</code> arayüzü) tipik nedendir.
            </>
          }
        />
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(r) => `${r.domain}-${r.process}`}
          filterText={(r) => `${r.domain} ${r.process}`}
          filterLabel="Alan adı / süreç filtrele…"
          initialSort={{ key: 'hits', dir: 'desc' }}
          scrollClass="max-h-96"
          className="border-0"
        />
      )}
    </div>
  )
}
