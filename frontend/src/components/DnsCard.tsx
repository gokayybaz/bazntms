import { useEffect, useState } from 'react'
import { formatNum } from '../lib/format'

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
]

export function DnsCard({ agentId }: { agentId?: number } = {}) {
  const [rows, setRows] = useState<DnsRow[]>([])
  const [minutes, setMinutes] = useState(60)
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

  const maxHits = Math.max(1, ...rows.map((r) => r.queries + r.responses))

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="flex rounded-lg border border-slate-700/80 p-0.5">
          {RANGES.map((r) => (
            <button
              key={r.minutes}
              onClick={() => setMinutes(r.minutes)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition ${
                minutes === r.minutes ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
        <span className="ml-auto text-[11px] text-slate-500">UDP/53 · süreç atıflı · agent'ta -pcap açık olmalı</span>
      </div>

      {!loaded ? (
        <p className="py-8 text-center text-sm text-slate-600">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center text-sm text-slate-600">
          Henüz DNS görünürlüğü verisi yok — agent'ları <code className="text-slate-400">-pcap</code> ile çalıştırın.
        </p>
      ) : (
        <div className="max-h-96 overflow-y-auto pr-1">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
                <th className="pb-2 font-medium">#</th>
                <th className="pb-2 font-medium">Alan Adı</th>
                <th className="pb-2 font-medium">Süreç</th>
                <th className="pb-2 pl-3 font-medium" style={{ width: '26%' }}>
                  Sorgu/Yanıt
                </th>
                <th className="pb-2 text-right font-medium">Agent</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {rows.map((r, i) => (
                <tr key={`${r.domain}-${r.process}`} className="hover:bg-slate-800/30">
                  <td className="py-1.5 font-mono text-xs text-slate-600">{i + 1}</td>
                  <td className="py-1.5 pr-3">
                    <span className="block max-w-[260px] truncate font-medium text-slate-200" title={r.domain}>
                      {r.domain}
                    </span>
                  </td>
                  <td className="py-1.5 pr-3 font-mono text-xs text-slate-400">
                    <span className="block max-w-[140px] truncate" title={r.process}>
                      {r.process || '—'}
                    </span>
                  </td>
                  <td className="py-1.5 pl-3">
                    <div className="flex items-center gap-2">
                      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
                        <div
                          className="h-full rounded-full bg-amber-500"
                          style={{ width: `${Math.max(3, ((r.queries + r.responses) / maxHits) * 100)}%` }}
                        />
                      </div>
                      <span className="w-16 text-right font-mono text-xs text-slate-400">
                        {formatNum(r.queries)}/{formatNum(r.responses)}
                      </span>
                    </div>
                  </td>
                  <td className="py-1.5 text-right font-mono text-xs text-slate-500">{r.agent_count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
