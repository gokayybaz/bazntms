import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import type { AgentWithRates, Bucket } from '../types'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { Card } from '../components/Card'
import { ThroughputChart } from '../components/ThroughputChart'
import { ProcessesCard } from '../components/ProcessesCard'

interface AgentConnSample {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
  pid: number
  process?: string
}

const RANGES = [
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
  { label: '24 saat', minutes: 1440 },
]

function relTime(unix: number): string {
  if (!unix) return '—'
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

export function AgentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [agent, setAgent] = useState<AgentWithRates | null>(null)
  const [connections, setConnections] = useState<AgentConnSample[]>([])
  const [history, setHistory] = useState<Bucket[]>([])
  const [minutes, setMinutes] = useState(60)
  const [loaded, setLoaded] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [connQuery, setConnQuery] = useState('')

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/agents/${id}`)
        if (res.status === 401) return
        if (res.status === 404) {
          if (!stop) setNotFound(true)
          return
        }
        const data: { agent: AgentWithRates; connections: AgentConnSample[] } = await res.json()
        if (!stop) {
          setAgent(data.agent)
          setConnections(data.connections ?? [])
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const t = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id])

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/agents/${id}/history?minutes=${minutes}`)
        if (res.status === 401) return
        if (!stop) setHistory(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const t = window.setInterval(load, 15_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id, minutes])

  const filteredConns = useMemo(() => {
    const q = connQuery.trim().toLowerCase()
    if (!q) return connections
    return connections.filter((c) =>
      [c.local_addr, c.remote_addr ?? '', c.process ?? '', c.status ?? '', String(c.pid)].join(' ').toLowerCase().includes(q),
    )
  }, [connections, connQuery])

  if (notFound) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <p className="py-16 text-center text-sm text-slate-600">
          Agent bulunamadı. <Link to="/agentlar" className="text-cyan-400 hover:underline">Agent listesine dön</Link>
        </p>
      </div>
    )
  }

  if (!loaded || !agent) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <p className="py-16 text-center text-sm text-slate-600">Yükleniyor…</p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <Link to="/agentlar" className="text-xs text-slate-500 hover:text-cyan-400">
          ← Agent'lar
        </Link>
      </div>

      {/* başlık */}
      <div className="flex flex-wrap items-center gap-3">
        <span
          className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
            agent.online
              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
              : 'border-slate-600 bg-slate-800 text-slate-500'
          }`}
        >
          <span className={`size-1.5 rounded-full ${agent.online ? 'bg-emerald-400' : 'bg-slate-500'}`} />
          {agent.online ? 'online' : 'offline'}
        </span>
        <h1 className="font-mono text-xl font-bold text-slate-100">{agent.name}</h1>
        {agent.site && <span className="rounded bg-slate-800 px-2 py-0.5 font-mono text-xs text-slate-400">{agent.site}</span>}
      </div>

      {/* özet şeridi */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">IP Adresi</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200">{agent.remote_ip || '—'}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Sürüm</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200">{agent.version || '—'}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Bağlantı</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{formatNum(agent.conns)}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">İlk Görülme</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{relTime(agent.first_seen)}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Son Görülme</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{relTime(agent.last_seen)}</p>
        </div>
      </div>

      {/* throughput grafiği */}
      <Card
        title="Trafik Geçmişi"
        right={
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
        }
      >
        <ThroughputChart history={history} running={agent.online} rangeMinutes={minutes} subtitle={`son ${minutes} dk · tüm arayüzler toplamı`} />
      </Card>

      {/* arayüzler */}
      <Card title="Arayüzler" right={<span className="text-xs text-slate-500">{agent.rates?.length ?? 0} arayüz</span>}>
        {!agent.rates || agent.rates.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Henüz arayüz verisi yok.</p>
        ) : (
          <div className="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
            {agent.rates.map((r) => (
              <div key={r.name} className="rounded-md border border-slate-800 bg-slate-900/50 p-3">
                <p className="truncate font-mono text-sm font-semibold text-slate-100">{r.name}</p>
                <p className="mt-1.5 font-mono text-xs">
                  <span className="text-cyan-300/90">↓ {formatBits(r.rx_bps * 8)}</span>
                  <span className="mx-1.5 text-slate-700">|</span>
                  <span className="text-violet-300/90">↑ {formatBits(r.tx_bps * 8)}</span>
                </p>
                <p className="mt-1 font-mono text-[10.5px] text-slate-600">
                  toplam {formatBytes(r.rx_bytes + r.tx_bytes)} · {Math.round(r.pps)} pps
                </p>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* süreç trafiği */}
      <Card title="Süreç Trafiği" right={<span className="text-xs text-slate-500">bu agent</span>}>
        <ProcessesCard agentId={agent.id} />
      </Card>

      {/* bağlantılar */}
      <Card title="Bağlantılar" right={<span className="text-xs text-slate-500">son telemetri anı · {formatNum(connections.length)} bağlantı</span>}>
        <input
          value={connQuery}
          onChange={(e) => setConnQuery(e.target.value)}
          placeholder="Filtrele: adres, süreç, durum…"
          className="mb-3 w-64 rounded-lg border border-slate-700/80 bg-slate-900 px-3 py-1.5 text-sm outline-none placeholder:text-slate-600 focus:border-cyan-500/60"
        />
        {filteredConns.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Eşleşen bağlantı yok.</p>
        ) : (
          <div className="max-h-96 overflow-y-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
                <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
                  <th className="px-3 py-2 font-medium">Protokol</th>
                  <th className="px-3 py-2 font-medium">Yerel Adres</th>
                  <th className="px-3 py-2 font-medium">Uzak Adres</th>
                  <th className="px-3 py-2 font-medium">Durum</th>
                  <th className="px-3 py-2 font-medium">Süreç</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {filteredConns.slice(0, 300).map((c, i) => (
                  <tr key={`${c.proto}-${c.local_addr}-${c.remote_addr ?? ''}-${i}`} className="hover:bg-slate-800/30">
                    <td className="px-3 py-1.5">
                      <span
                        className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-bold uppercase ${
                          c.proto === 'udp' ? 'bg-violet-500/10 text-violet-400' : 'bg-cyan-500/10 text-cyan-400'
                        }`}
                      >
                        {c.proto}
                      </span>
                    </td>
                    <td className="px-3 py-1.5 font-mono text-xs text-slate-300">{c.local_addr}</td>
                    <td className="px-3 py-1.5 font-mono text-xs text-slate-400">{c.remote_addr || '—'}</td>
                    <td className="px-3 py-1.5 font-mono text-xs text-slate-400">{c.status || '—'}</td>
                    <td className="px-3 py-1.5 text-slate-300">
                      {c.process ? (
                        <>
                          {c.process}
                          {c.pid > 0 && <span className="ml-1 font-mono text-[10px] text-slate-600">[{c.pid}]</span>}
                        </>
                      ) : (
                        <span className="text-slate-600">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
