import { useEffect, useMemo, useState } from 'react'
import { formatBytes } from '../lib/format'
import { RangeTabs } from './RangeTabs'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface ProcessUsage {
  process: string
  bytes_in: number
  bytes_out: number
  total: number
  agent_count: number
}

const RANGES = [
  { label: '15 dk', value: 15 },
  { label: '1 saat', value: 60 },
  { label: '6 saat', value: 360 },
] as const

export function ProcessesCard({ agentId }: { agentId?: number } = {}) {
  const [rows, setRows] = useState<ProcessUsage[]>([])
  const [minutes, setMinutes] = useState<15 | 60 | 360>(60)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const agentParam = agentId ? `&agent_id=${agentId}` : ''
        const res = await fetch(`/api/v1/processes?minutes=${minutes}&limit=20${agentParam}`)
        if (res.status === 401) return
        const data = await res.json()
        if (!stop) {
          setRows(data)
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

  const max = useMemo(() => Math.max(1, ...rows.map((r) => r.total)), [rows])

  const cols: TuiColumn<ProcessUsage>[] = [
    { key: 'process', header: 'Süreç', sortable: true, render: (r) => <span className="text-ink-hi">{r.process || 'bilinmeyen'}</span> },
    { key: 'bytes_in', header: 'İndirme', align: 'right', sortable: true, sortValue: (r) => r.bytes_in, render: (r) => <span className="text-rx">{formatBytes(r.bytes_in)}</span> },
    { key: 'bytes_out', header: 'Gönderme', align: 'right', sortable: true, sortValue: (r) => r.bytes_out, render: (r) => <span className="text-tx">{formatBytes(r.bytes_out)}</span> },
    {
      key: 'total',
      header: 'Toplam',
      width: '34%',
      sortable: true,
      sortValue: (r) => r.total,
      render: (r) => (
        <span className="flex items-center gap-2">
          <span className="h-1.5 flex-1 bg-panel-2">
            <span className="block h-full bg-emerald-500" style={{ width: `${Math.max(2, (r.total / max) * 100)}%` }} />
          </span>
          <span className="w-16 shrink-0 text-right">{formatBytes(r.total)}</span>
        </span>
      ),
    },
    { key: 'agent_count', header: 'Agent', align: 'right', sortable: true, sortValue: (r) => r.agent_count, render: (r) => <span className="text-tui-dim">{r.agent_count}</span> },
  ]

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <RangeTabs ranges={RANGES} value={minutes} onChange={setMinutes} />
        <span className="ml-auto font-mono text-[10px] text-tui-dim">
          nethogs yöntemi: pcap + soket→PID · agent'ta -pcap açık olmalı
        </span>
      </div>

      {!loaded ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">
          Henüz süreç trafiği yok — agent'ları <code className="text-tui-dim">-pcap</code> ile çalıştırın ve
          hub'da <code className="text-tui-dim">-agent-pcap</code> politikasını açın.
        </p>
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(r) => r.process}
          filterText={(r) => r.process}
          filterLabel="Süreç filtrele…"
          initialSort={{ key: 'total', dir: 'desc' }}
          scrollClass="max-h-80"
          className="border-0"
        />
      )}
    </div>
  )
}
