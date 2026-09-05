import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import type { AgentWithRates } from '../types'
import { formatBits, formatNum } from '../lib/format'
import { Card } from '../components/Card'
import { StatusPill } from '../components/StatusPill'
import { ProcessesCard } from '../components/ProcessesCard'
import { L7Card } from '../components/L7Card'
import { DnsCard } from '../components/DnsCard'

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
  // poll başarısız olursa görünür bir uyarı — eskiden sessizce yutuluyordu,
  // liste "canlı" görünmeye devam ederdi (impeccable critique 2026-09-05)
  const [dataStale, setDataStale] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.status === 401) return
        if (!res.ok) {
          if (!stop) setDataStale(true)
          return
        }
        if (!stop) {
          setAgents(await res.json())
          setLoaded(true)
          setDataStale(false)
        }
      } catch {
        if (!stop) setDataStale(true)
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
        <span className="text-xs text-dim-aa">tüm filo · detay için bir agent'a tıklayın</span>
      </div>

      {dataStale && (
        <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-400">
          ⚠ Liste güncellenemiyor — bağlantı sorunu olabilir, gösterilenler son başarılı polldan.
        </p>
      )}

      <Card>
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filtrele: ad, site, ip, sürüm…"
            aria-label="Agent filtresi"
            className="w-64 rounded-lg border border-slate-700/80 bg-slate-900 px-3 py-1.5 text-sm outline-none placeholder:text-dim-aa focus:border-cyan-500/60"
          />
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-400 select-none">
            <input type="checkbox" checked={onlyOnline} onChange={(e) => setOnlyOnline(e.target.checked)} className="accent-cyan-500" />
            yalnızca online
          </label>
          <span className="ml-auto text-xs text-dim-aa">
            {formatNum(filtered.length)} / {formatNum(agents.length)} agent
          </span>
        </div>

        {!loaded ? (
          <p className="py-8 text-center text-sm text-dim-aa">Yükleniyor…</p>
        ) : filtered.length === 0 ? (
          <p className="py-8 text-center text-sm text-dim-aa">Eşleşen agent yok.</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="bg-slate-900/95">
                <tr className="text-left text-[11px] uppercase tracking-wider text-dim-aa">
                  <th scope="col" className="px-3 py-2 font-medium">Durum</th>
                  <th scope="col" className="px-3 py-2 font-medium">Ad</th>
                  <th scope="col" className="px-3 py-2 font-medium">Site</th>
                  <th scope="col" className="px-3 py-2 font-medium">IP</th>
                  <th scope="col" className="px-3 py-2 font-medium">Sürüm</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Bağlantı</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">En Yoğun Arayüz</th>
                  <th scope="col" className="px-3 py-2 font-medium">Son Görülme</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {filtered.map((a) => {
                  const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
                  return (
                    <tr key={a.id} className="hover:bg-slate-800/30">
                      <td className="px-3 py-2">
                        <StatusPill tone={a.online ? 'emerald' : 'slate'} label={a.online ? 'online' : 'offline'} />
                      </td>
                      <td className="px-3 py-2">
                        <Link to={`/agentlar/${a.id}`} className="font-mono font-semibold text-cyan-300 hover:text-cyan-200 hover:underline">
                          {a.name}
                        </Link>
                      </td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-400">{a.site || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-400">{a.remote_ip || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-400">{a.version || '—'}</td>
                      <td className="px-3 py-2 text-right font-mono text-xs text-slate-300">{formatNum(a.conns)}</td>
                      <td className="px-3 py-2 text-right font-mono text-xs">
                        {busiest ? (
                          <>
                            <span className="text-cyan-300/90">↓ {formatBits(busiest.rx_bps * 8)}</span>
                            <span className="mx-1.5 text-slate-700">|</span>
                            <span className="text-violet-300/90">↑ {formatBits(busiest.tx_bps * 8)}</span>
                          </>
                        ) : (
                          <span className="text-dim-aa">—</span>
                        )}
                      </td>
                      <td className="px-3 py-2 font-mono text-[11px] text-dim-aa">{relTime(a.last_seen)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Card title="Süreç Trafiği (Tüm Agent'lar)" right={<span className="text-xs text-dim-aa">süreç → agent atıflı</span>}>
        <ProcessesCard />
      </Card>

      <Card title="Uygulama Görünürlüğü (Tüm Agent'lar)" right={<span className="text-xs text-dim-aa">L7 · SNI + HTTP Host</span>}>
        <L7Card />
      </Card>

      <Card title="DNS Görünürlüğü (Tüm Agent'lar)" right={<span className="text-xs text-dim-aa">UDP/53 · süreç atıflı</span>}>
        <DnsCard />
      </Card>
    </div>
  )
}
