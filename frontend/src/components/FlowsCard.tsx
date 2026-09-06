import { useEffect, useState } from 'react'
import { formatBytes } from '../lib/format'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface FlowRow {
  ts: number
  device: string
  src: string
  dst: string
  src_port: number
  dst_port: number
  proto: string
  packets: number
  octets: number
}

const cols: TuiColumn<FlowRow>[] = [
  {
    key: 'ts',
    header: 'Saat',
    width: '5rem',
    sortable: true,
    sortValue: (f) => f.ts,
    render: (f) => <span className="text-tui-dim">{f.ts ? new Date(f.ts * 1000).toLocaleTimeString('tr-TR') : '—'}</span>,
  },
  { key: 'device', header: 'Cihaz', sortable: true, render: (f) => <span className="text-tui-dim">{f.device}</span> },
  {
    key: 'flow',
    header: 'Akış',
    render: (f) => (
      <span className="text-ink">
        {f.src}:{f.src_port} → {f.dst}:{f.dst_port}
      </span>
    ),
  },
  { key: 'proto', header: 'Proto', width: '4rem', sortable: true, render: (f) => <span className="uppercase text-tui-dim">{f.proto}</span> },
  { key: 'packets', header: 'Paket', align: 'right', sortable: true, sortValue: (f) => f.packets, render: (f) => <span className="text-tui-dim">{f.packets}</span> },
  { key: 'octets', header: 'Octet', align: 'right', sortable: true, sortValue: (f) => f.octets, render: (f) => <span className="text-emerald-400">{formatBytes(f.octets)}</span> },
]

export function FlowsCard() {
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/flows?minutes=15&limit=20')
        if (res.status === 401) return
        const data = await res.json()
        if (!stop) {
          setFlows(data)
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 10_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  if (!loaded) return <p className="py-6 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
  if (flows.length === 0) {
    return (
      <p className="py-6 text-center font-mono text-[11px] text-tui-dim">
        Akış yok — cihazları NetFlow v5 export için hub'ın <code className="text-tui-dim">-flow-port</code> adresine yönlendirin.
      </p>
    )
  }

  return (
    <TuiTable
      columns={cols}
      rows={flows}
      getKey={(f) => `${f.ts}-${f.src}-${f.dst}-${f.src_port}-${f.dst_port}`}
      filterText={(f) => `${f.src} ${f.dst} ${f.device} ${f.proto}`}
      filterLabel="Akış filtrele (ip, cihaz, proto)…"
      initialSort={{ key: 'ts', dir: 'desc' }}
      scrollClass="max-h-72"
      className="border-0"
    />
  )
}
