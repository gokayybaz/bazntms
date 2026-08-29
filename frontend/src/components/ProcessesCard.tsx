import { useEffect, useState } from 'react'
import { formatBytes } from '../lib/format'

interface ProcessUsage {
  process: string
  bytes_in: number
  bytes_out: number
  total: number
  agent_count: number
}

const RANGES = [
  { label: '15 dk', minutes: 15 },
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
]

export function ProcessesCard() {
  const [rows, setRows] = useState<ProcessUsage[]>([])
  const [minutes, setMinutes] = useState(60)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/processes?minutes=${minutes}&limit=20`)
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
  }, [minutes])

  const max = Math.max(1, ...rows.map((r) => r.total))

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
        <span className="ml-auto text-[11px] text-slate-500">
          nethogs yöntemi: pcap + soket→PID eşlemesi · agent'ta -pcap açık olmalı
        </span>
      </div>

      {!loaded ? (
        <p className="py-8 text-center text-sm text-slate-600">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center text-sm text-slate-600">
          Henüz süreç trafiği yok — agent'ları <code className="text-slate-400">-pcap</code> ile çalıştırın ve
          hub'da <code className="text-slate-400">-agent-pcap</code> politikasını açın.
        </p>
      ) : (
        <div className="max-h-80 overflow-y-auto pr-1">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
                <th className="pb-2 font-medium">#</th>
                <th className="pb-2 font-medium">Süreç</th>
                <th className="pb-2 text-right font-medium">İndirme</th>
                <th className="pb-2 text-right font-medium">Gönderme</th>
                <th className="pb-2 pl-4 font-medium" style={{ width: '34%' }}>
                  Toplam
                </th>
                <th className="pb-2 text-right font-medium">Agent</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {rows.map((r, i) => (
                <tr key={r.process} className="hover:bg-slate-800/30">
                  <td className="py-1.5 font-mono text-xs text-slate-600">{i + 1}</td>
                  <td className="py-1.5 pr-3 font-medium text-slate-200">
                    <span className="block max-w-[220px] truncate" title={r.process}>
                      {r.process || 'bilinmeyen'}
                    </span>
                  </td>
                  <td className="py-1.5 text-right font-mono text-xs text-cyan-300/90">{formatBytes(r.bytes_in)}</td>
                  <td className="py-1.5 text-right font-mono text-xs text-violet-300/90">{formatBytes(r.bytes_out)}</td>
                  <td className="py-1.5 pl-4">
                    <div className="flex items-center gap-2">
                      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-800">
                        <div
                          className="h-full rounded-full bg-emerald-500"
                          style={{ width: `${Math.max(2, (r.total / max) * 100)}%` }}
                        />
                      </div>
                      <span className="w-20 text-right font-mono text-xs text-slate-300">{formatBytes(r.total)}</span>
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
