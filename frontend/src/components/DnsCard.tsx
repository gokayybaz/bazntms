import { useEffect, useMemo, useState } from 'react'
import { formatNum } from '../lib/format'
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
  { label: '15 dk', minutes: 15 },
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
] as const

export function DnsCard({ agentId }: { agentId?: number } = {}) {
  const [rows, setRows] = useState<DnsRow[]>([])
  const [minutes, setMinutes] = useState<15 | 60 | 360>(60)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const agentParam = agentId ? `&agent_id=${agentId}` : ''
        const res = await fetch(`/api/v1/dns?minutes=${minutes}&limit=30${agentParam}`)
        if (res.status === 401) return
        const data = await res.json()
        if (!stop) {
          setRows(Array.isArray(data) ? data : [])
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 15_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [minutes, agentId])

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
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">
          Henüz DNS görünürlüğü verisi yok — agent'ları <code className="text-tui-dim">-pcap</code> ile çalıştırın.
        </p>
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
