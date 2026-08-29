import { useEffect, useState } from 'react'
import { formatBytes } from '../lib/format'

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

  if (!loaded) return <p className="py-6 text-center text-sm text-slate-600">Yükleniyor…</p>
  if (flows.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-slate-600">
        Akış yok — cihazları NetFlow v5 export için hub'ın <code className="text-slate-400">-flow-port</code> adresine yönlendirin.
      </p>
    )
  }

  return (
    <div className="max-h-72 overflow-y-auto rounded-lg border border-slate-800/60">
      <table className="w-full text-sm">
        <thead className="sticky top-0 bg-slate-900/95">
          <tr className="text-left text-[10px] uppercase tracking-wider text-slate-500">
            <th className="px-3 py-1.5 font-medium">Cihaz</th>
            <th className="px-3 py-1.5 font-medium">Akış</th>
            <th className="px-3 py-1.5 font-medium">Protokol</th>
            <th className="px-3 py-1.5 text-right font-medium">Paket</th>
            <th className="px-3 py-1.5 text-right font-medium">Octet</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800/50">
          {flows.map((f, i) => (
            <tr key={i} className="hover:bg-slate-800/30">
              <td className="px-3 py-1.5 font-mono text-xs text-slate-400">{f.device}</td>
              <td className="px-3 py-1.5 font-mono text-xs text-slate-300">
                {f.src}:{f.src_port} → {f.dst}:{f.dst_port}
              </td>
              <td className="px-3 py-1.5">
                <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[10px] uppercase text-slate-400">{f.proto}</span>
              </td>
              <td className="px-3 py-1.5 text-right font-mono text-xs text-slate-400">{f.packets}</td>
              <td className="px-3 py-1.5 text-right font-mono text-xs text-emerald-300/90">{formatBytes(f.octets)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
