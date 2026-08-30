import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import type { AgentWithRates } from '../types'
import { formatBits, formatNum } from '../lib/format'
import { Card } from '../components/Card'

function relTime(unix: number): string {
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

export function AgentsListPage() {
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [loaded, setLoaded] = useState(false)
  const [query, setQuery] = useState('')
  const [onlyOnline, setOnlyOnline] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.status === 401) return
        if (!stop) {
          setAgents(await res.json())
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return agents.filter((a) => {
      if (onlyOnline && !a.online) return false
      if (!q) return true
      return [a.name, a.site, a.remote_ip, a.version].join(' ').toLowerCase().includes(q)
    })
  }, [agents, query, onlyOnline])

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Agent'lar</h1>
        <span className="text-xs text-slate-500">tüm filo · detay için bir agent'a tıklayın</span>
      </div>

      <Card>
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filtrele: ad, site, ip, sürüm…"
            className="w-64 rounded-lg border border-slate-700/80 bg-slate-900 px-3 py-1.5 text-sm outline-none placeholder:text-slate-600 focus:border-cyan-500/60"
          />
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-400 select-none">
            <input type="checkbox" checked={onlyOnline} onChange={(e) => setOnlyOnline(e.target.checked)} className="accent-cyan-500" />
            yalnızca online
          </label>
          <span className="ml-auto text-xs text-slate-500">
            {formatNum(filtered.length)} / {formatNum(agents.length)} agent
          </span>
        </div>

        {!loaded ? (
          <p className="py-8 text-center text-sm text-slate-600">Yükleniyor…</p>
        ) : filtered.length === 0 ? (
          <p className="py-8 text-center text-sm text-slate-600">Eşleşen agent yok.</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="bg-slate-900/95">
                <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
                  <th className="px-3 py-2 font-medium">Durum</th>
                  <th className="px-3 py-2 font-medium">Ad</th>
                  <th className="px-3 py-2 font-medium">Site</th>
                  <th className="px-3 py-2 font-medium">IP</th>
                  <th className="px-3 py-2 font-medium">Sürüm</th>
                  <th className="px-3 py-2 text-right font-medium">Bağlantı</th>
                  <th className="px-3 py-2 text-right font-medium">En Yoğun Arayüz</th>
                  <th className="px-3 py-2 font-medium">Son Görülme</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {filtered.map((a) => {
                  const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
                  return (
                    <tr key={a.id} className="hover:bg-slate-800/30">
                      <td className="px-3 py-2">
                        <span
                          className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
                            a.online
                              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                              : 'border-slate-600 bg-slate-800 text-slate-500'
                          }`}
                        >
                          <span className={`size-1.5 rounded-full ${a.online ? 'bg-emerald-400' : 'bg-slate-500'}`} />
                          {a.online ? 'online' : 'offline'}
                        </span>
                      </td>
                      <td className="px-3 py-2">
                        <Link to={`/agentlar/${a.id}`} className="font-mono font-semibold text-cyan-300 hover:text-cyan-200 hover:underline">
                          {a.name}
                        </Link>
                      </td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-400">{a.site || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-400">{a.remote_ip || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-500">{a.version || '—'}</td>
                      <td className="px-3 py-2 text-right font-mono text-xs text-slate-300">{formatNum(a.conns)}</td>
                      <td className="px-3 py-2 text-right font-mono text-xs">
                        {busiest ? (
                          <>
                            <span className="text-cyan-300/90">↓ {formatBits(busiest.rx_bps * 8)}</span>
                            <span className="mx-1.5 text-slate-700">|</span>
                            <span className="text-violet-300/90">↑ {formatBits(busiest.tx_bps * 8)}</span>
                          </>
                        ) : (
                          <span className="text-slate-600">—</span>
                        )}
                      </td>
                      <td className="px-3 py-2 font-mono text-[11px] text-slate-500">{relTime(a.last_seen)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
