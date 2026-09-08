import { useMemo, useState } from 'react'
import { formatBytes, formatNum } from '../lib/format'
import { usePolledJson } from '../lib/usePolledJson'
import { PanelState } from './PanelState'
import { RangeTabs } from './RangeTabs'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface L7Row {
  host: string
  kind: string // tls | http
  process: string
  bytes: number
  hits: number
  agent_count: number
}

const RANGES = [
  { label: '15 dk', value: 15 },
  { label: '1 saat', value: 60 },
  { label: '6 saat', value: 360 },
] as const

export function L7Card({ agentId }: { agentId?: number } = {}) {
  const [minutes, setMinutes] = useState<15 | 60 | 360>(60)
  const { data, loaded } = usePolledJson<L7Row[]>(
    `/api/v1/l7?minutes=${minutes}&limit=30${agentId ? `&agent_id=${agentId}` : ''}`,
    15_000,
  )
  const rows = useMemo(() => (Array.isArray(data) ? data : []), [data])

  const maxHits = useMemo(() => Math.max(1, ...rows.map((r) => r.hits)), [rows])

  const cols: TuiColumn<L7Row>[] = [
    {
      key: 'host',
      header: 'Alan Adı',
      sortable: true,
      render: (r) => (
        <span className="flex items-center gap-1.5">
          <span className={`px-1 font-mono text-[10px] uppercase ${r.kind === 'tls' ? 'text-emerald-400' : 'text-amber-400'}`}>{r.kind}</span>
          <span className="text-ink-hi">{r.host}</span>
        </span>
      ),
    },
    { key: 'process', header: 'Süreç', sortable: true, render: (r) => <span className="text-tui-dim">{r.process || '—'}</span> },
    {
      key: 'hits',
      header: 'Gözlem',
      width: '26%',
      sortable: true,
      sortValue: (r) => r.hits,
      render: (r) => (
        <span className="flex items-center gap-2">
          <span className="h-1.5 flex-1 bg-panel-2">
            <span className="block h-full bg-rx" style={{ width: `${Math.max(3, (r.hits / maxHits) * 100)}%` }} />
          </span>
          <span className="w-10 shrink-0 text-right">{formatNum(r.hits)}</span>
        </span>
      ),
    },
    { key: 'bytes', header: 'Veri', align: 'right', sortable: true, sortValue: (r) => r.bytes, render: (r) => <span className="text-tui-dim">{formatBytes(r.bytes)}</span> },
    { key: 'agent_count', header: 'Agent', align: 'right', sortable: true, sortValue: (r) => r.agent_count, render: (r) => <span className="text-tui-dim">{r.agent_count}</span> },
  ]

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <RangeTabs ranges={RANGES} value={minutes} onChange={setMinutes} />
        <span className="ml-auto font-mono text-[10px] text-tui-dim">TLS SNI + HTTP Host · agent'ta -pcap açık olmalı</span>
      </div>

      {!loaded ? (
        <PanelState kind="loading" />
      ) : rows.length === 0 ? (
        <PanelState
          kind="empty"
          message="Henüz uygulama görünürlüğü verisi yok."
          hint={
            <>
              agent'ları <code className="text-tui-dim">-pcap</code> ile çalıştırın
            </>
          }
        />
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(r) => `${r.host}-${r.process}-${r.kind}`}
          filterText={(r) => `${r.host} ${r.process}`}
          filterLabel="Alan adı / süreç filtrele…"
          initialSort={{ key: 'hits', dir: 'desc' }}
          scrollClass="max-h-96"
          className="border-0"
        />
      )}
    </div>
  )
}
