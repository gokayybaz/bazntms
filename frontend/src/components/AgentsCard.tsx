import { useEffect, useState } from 'react'
import type { AgentWithRates } from '../types'
import { formatBits, formatNum } from '../lib/format'

function relTime(unix: number): string {
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

export function AgentsCard({ refreshKey }: { refreshKey: number }) {
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.status === 401) return
        const data = await res.json()
        if (!stop) {
          setAgents(data)
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [refreshKey])

  if (!loaded) {
    return <p className="py-8 text-center text-sm text-slate-600">Yükleniyor…</p>
  }
  if (agents.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-slate-600">
        Henüz agent yok — uçlara <code className="text-slate-400">bazntms-agent</code> kurun, enrollment ile burada görünür.
      </p>
    )
  }

  return (
    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      {agents.map((a) => {
        const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
        return (
          <div key={a.id} className="rounded-md border border-slate-800 bg-slate-900/50 p-3.5">
            <div className="flex items-center gap-2">
              <span
                className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
                  a.online
                    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                    : 'border-slate-600 bg-slate-800 text-slate-500'
                }`}
              >
                <span className={`size-1.5 ${a.online ? 'bg-emerald-400' : 'bg-slate-500'}`} />
                {a.online ? 'online' : 'offline'}
              </span>
              <span className="truncate font-mono text-sm font-semibold text-slate-100">{a.name}</span>
              {a.site && <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[10px] text-slate-400">{a.site}</span>}
              <span className="ml-auto font-mono text-[10px] text-slate-600">{relTime(a.last_seen)}</span>
            </div>

            <div className="mt-2 grid grid-cols-3 gap-2 font-mono text-[11px]">
              <div>
                <p className="text-slate-600">sürüm</p>
                <p className="truncate text-slate-300">{a.version || '—'}</p>
              </div>
              <div>
                <p className="text-slate-600">bağlantı</p>
                <p className="text-slate-300">{formatNum(a.conns)}</p>
              </div>
              <div>
                <p className="text-slate-600">ip</p>
                <p className="truncate text-slate-300" title={a.remote_ip}>{a.remote_ip || '—'}</p>
              </div>
            </div>

            {busiest && (
              <div className="mt-2 rounded border border-slate-800 bg-slate-950/60 px-2.5 py-1.5 font-mono text-[11px]">
                <p className="truncate text-slate-500">
                  en yoğun arayüz: <span className="text-slate-300">{busiest.name}</span>
                </p>
                <p className="mt-0.5">
                  <span className="text-cyan-300">↓ {formatBits(busiest.rx_bps * 8)}</span>
                  <span className="mx-2 text-slate-700">|</span>
                  <span className="text-violet-300">↑ {formatBits(busiest.tx_bps * 8)}</span>
                </p>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
