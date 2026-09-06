import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import type { AgentWithRates } from '../types'
import { formatBits, formatNum } from '../lib/format'
import { Panel } from '../components/Panel'
import { Sparkline } from '../components/Sparkline'
import { TuiTable } from '../components/TuiTable'
import type { TuiColumn } from '../components/TuiTable'
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

const busiestRate = (a: AgentWithRates) =>
  [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]

export function AgentsListPage() {
  const navigate = useNavigate()
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [loaded, setLoaded] = useState(false)
  const [onlyOnline, setOnlyOnline] = useState(false)
  // poll başarısız olursa görünür bir uyarı — eskiden sessizce yutuluyordu
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

  // oturum-içi trend: her poll'da agent'ın en-yoğun arayüz toplamını biriktir
  // (htop'un süreç geçmişi gibi — ayrı bir history endpoint'i gerekmez)
  const trendRef = useRef<Map<number, number[]>>(new Map())
  useEffect(() => {
    const m = trendRef.current
    for (const a of agents) {
      const b = busiestRate(a)
      const v = b ? (b.rx_bps + b.tx_bps) * 8 : 0
      const arr = m.get(a.id) ?? []
      arr.push(v)
      if (arr.length > 20) arr.shift()
      m.set(a.id, arr)
    }
  }, [agents])

  const rows = useMemo(() => (onlyOnline ? agents.filter((a) => a.online) : agents), [agents, onlyOnline])

  const cols: TuiColumn<AgentWithRates>[] = [
    {
      key: 'online',
      header: 'Durum',
      width: '5.5rem',
      sortable: true,
      sortValue: (a) => (a.online ? 0 : 1),
      render: (a) => (
        <span className={a.online ? 'text-emerald-400' : 'text-tui-dim'}>{a.online ? '● online' : '○ offline'}</span>
      ),
    },
    {
      key: 'name',
      header: 'Ad',
      sortable: true,
      render: (a) => (
        <Link to={`/agentlar/${a.id}`} className="font-semibold text-rx hover:underline">
          {a.name}
        </Link>
      ),
    },
    { key: 'site', header: 'Site', sortable: true, render: (a) => <span className="text-tui-dim">{a.site || '—'}</span> },
    { key: 'remote_ip', header: 'IP', render: (a) => <span className="text-tui-dim">{a.remote_ip || '—'}</span> },
    { key: 'version', header: 'Sürüm', sortable: true, render: (a) => <span className="text-tui-dim">{a.version || '—'}</span> },
    {
      key: 'conns',
      header: 'Bağlantı',
      align: 'right',
      sortable: true,
      sortValue: (a) => a.conns,
      render: (a) => formatNum(a.conns),
    },
    {
      key: 'rate',
      header: 'En Yoğun Arayüz',
      align: 'right',
      sortValue: (a) => {
        const b = busiestRate(a)
        return b ? b.rx_bps + b.tx_bps : 0
      },
      sortable: true,
      render: (a) => {
        const b = busiestRate(a)
        return b ? (
          <span>
            <span className="text-rx">↓{formatBits(b.rx_bps * 8)}</span>
            <span className="mx-1 text-rule-hi">|</span>
            <span className="text-tx">↑{formatBits(b.tx_bps * 8)}</span>
          </span>
        ) : (
          <span className="text-tui-dim">—</span>
        )
      },
    },
    {
      key: 'trend',
      header: 'Trend',
      width: '7rem',
      render: (a) => {
        const t = trendRef.current.get(a.id) ?? []
        return t.length > 1 ? <Sparkline data={t} points={20} label={`${a.name} trafik trendi`} /> : <span className="text-rule-hi">·</span>
      },
    },
    {
      key: 'last_seen',
      header: 'Son Görülme',
      align: 'right',
      sortable: true,
      sortValue: (a) => a.last_seen,
      render: (a) => <span className="text-tui-dim">{relTime(a.last_seen)}</span>,
    },
  ]

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3">
      <div className="flex flex-wrap items-baseline gap-2 font-mono">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Agent'lar</h1>
        <span className="text-[10px] text-tui-dim">tüm filo · Enter / tıklama → detay</span>
        <label className="ml-auto flex cursor-pointer items-center gap-1.5 text-[11px] text-tui-dim select-none">
          <input type="checkbox" checked={onlyOnline} onChange={(e) => setOnlyOnline(e.target.checked)} className="accent-cyan-500" />
          yalnızca online
        </label>
      </div>

      {dataStale && (
        <p className="border border-amber-500/40 bg-amber-500/10 px-3 py-1.5 font-mono text-[11px] text-amber-400">
          ⚠ Liste güncellenemiyor — bağlantı sorunu olabilir, gösterilenler son başarılı polldan.
        </p>
      )}

      {!loaded ? (
        <Panel>
          <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
        </Panel>
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(a) => String(a.id)}
          onActivate={(a) => navigate(`/agentlar/${a.id}`)}
          filterText={(a) => `${a.name} ${a.site} ${a.remote_ip} ${a.version}`}
          filterLabel="Filtrele: ad, site, ip, sürüm…"
          initialSort={{ key: 'online', dir: 'asc' }}
          empty="Eşleşen agent yok."
          scrollClass="max-h-[34rem]"
        />
      )}

      <Panel title="Süreç Trafiği (Tüm Agent'lar)" right={<span className="font-mono text-[10px] text-tui-dim">süreç → agent atıflı</span>}>
        <ProcessesCard />
      </Panel>

      <Panel title="Uygulama Görünürlüğü (Tüm Agent'lar)" right={<span className="font-mono text-[10px] text-tui-dim">L7 · SNI + HTTP Host</span>}>
        <L7Card />
      </Panel>

      <Panel title="DNS Görünürlüğü (Tüm Agent'lar)" right={<span className="font-mono text-[10px] text-tui-dim">UDP/53 · süreç atıflı</span>}>
        <DnsCard />
      </Panel>
    </div>
  )
}
