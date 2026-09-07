import { useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { AgentWithRates, Bucket } from '../types'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { Panel } from '../components/Panel'
import { StatusPill } from '../components/StatusPill'
import { RangeTabs } from '../components/RangeTabs'
import { TuiTable } from '../components/TuiTable'
import type { TuiColumn } from '../components/TuiTable'
import { ThroughputChart } from '../components/ThroughputChart'
import { ProcessesCard } from '../components/ProcessesCard'
import { L7Card } from '../components/L7Card'
import { DnsCard } from '../components/DnsCard'

interface AgentConnSample {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
  pid: number
  process?: string
}

const RANGES = [
  { label: '1 saat', value: 60 },
  { label: '6 saat', value: 360 },
  { label: '24 saat', value: 1440 },
] as const

// silme onayı bekleme süresi (ms) — DevicesCard.tsx'teki iki-aşamalı silme deseni
const DELETE_CONFIRM_MS = 4000

// AttrMethodBadge — agent'ın aktif süreç-atıf arka ucu (Faz 20).
// ebpf/etw/pcap = aktif (cyan), off = kapalı (slate), boş = eski agent.
function AttrMethodBadge({ method }: { method?: string }) {
  if (!method) return <span className="text-tui-dim">—</span>
  return <StatusPill tone={method === 'off' ? 'slate' : 'cyan'} label={method} dot={false} />
}

function relTime(unix: number): string {
  if (!unix) return '—'
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

const connCols: TuiColumn<AgentConnSample>[] = [
  { key: 'proto', header: 'Proto', width: '4rem', sortable: true, render: (c) => <span className="uppercase text-tui-dim">{c.proto}</span> },
  { key: 'local_addr', header: 'Yerel Adres', sortable: true, render: (c) => <span className="text-ink">{c.local_addr}</span> },
  { key: 'remote_addr', header: 'Uzak Adres', sortable: true, render: (c) => <span className="text-tui-dim">{c.remote_addr || '—'}</span> },
  { key: 'status', header: 'Durum', width: '8rem', sortable: true, render: (c) => <span className="text-tui-dim">{c.status || '—'}</span> },
  {
    key: 'process',
    header: 'Süreç',
    sortable: true,
    render: (c) =>
      c.process ? (
        <span className="text-ink">
          {c.process}
          {c.pid > 0 && <span className="ml-1 text-[10px] text-tui-dim">[{c.pid}]</span>}
        </span>
      ) : (
        <span className="text-tui-dim">—</span>
      ),
  },
]

export function AgentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<AgentWithRates | null>(null)
  const [connections, setConnections] = useState<AgentConnSample[]>([])
  const [history, setHistory] = useState<Bucket[]>([])
  const [minutes, setMinutes] = useState<60 | 360 | 1440>(60)
  const [loaded, setLoaded] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [dataStale, setDataStale] = useState(false)
  // yeniden adlandırma/silme — satır-içi (native prompt/confirm değil)
  const [renaming, setRenaming] = useState(false)
  const [renameValue, setRenameValue] = useState('')
  const [renameBusy, setRenameBusy] = useState(false)
  const [renameError, setRenameError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const confirmTimer = useRef<number | null>(null)

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
        if (!res.ok) {
          if (!stop) setDataStale(true)
          return
        }
        const data: { agent: AgentWithRates; connections: AgentConnSample[] } = await res.json()
        if (!stop) {
          setAgent(data.agent)
          setConnections(data.connections ?? [])
          setLoaded(true)
          setDataStale(false)
        }
      } catch {
        if (!stop) setDataStale(true)
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
        /* yoksay — throughput grafiği ikincil */
      }
    }
    load()
    const t = window.setInterval(load, 15_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id, minutes])

  useEffect(
    () => () => {
      if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    },
    [],
  )

  const startRename = () => {
    if (!agent) return
    setRenameError('')
    setRenameValue(agent.name)
    setRenaming(true)
  }
  const cancelRename = () => {
    setRenaming(false)
    setRenameError('')
  }
  const submitRename = async () => {
    if (!agent || !id) return
    const name = renameValue.trim()
    if (!name || name === agent.name) {
      setRenaming(false)
      return
    }
    setRenameBusy(true)
    setRenameError('')
    try {
      const res = await fetch(`/api/v1/agents/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      })
      if (!res.ok) {
        setRenameError(await res.text())
        return
      }
      setAgent({ ...agent, name })
      setRenaming(false)
    } catch {
      setRenameError('bağlantı hatası')
    } finally {
      setRenameBusy(false)
    }
  }

  const handleDeleteClick = () => {
    if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    if (confirmDelete) {
      setConfirmDelete(false)
      void doDelete()
      return
    }
    setDeleteError('')
    setConfirmDelete(true)
    confirmTimer.current = window.setTimeout(() => setConfirmDelete(false), DELETE_CONFIRM_MS)
  }
  const doDelete = async () => {
    if (!agent || !id) return
    setDeleting(true)
    try {
      const res = await fetch(`/api/v1/agents/${id}`, { method: 'DELETE' })
      if (!res.ok) {
        setDeleteError(await res.text())
        setDeleting(false)
        return
      }
      navigate('/agentlar')
    } catch {
      setDeleteError('bağlantı hatası')
      setDeleting(false)
    }
  }

  const ifaceTotal = useMemo(() => {
    const rs = agent?.rates ?? []
    return {
      rx: rs.reduce((s, r) => s + r.rx_bps, 0),
      tx: rs.reduce((s, r) => s + r.tx_bps, 0),
    }
  }, [agent])

  if (notFound) {
    return (
      <div className="mx-auto max-w-[1600px] px-4 py-10">
        <p className="text-center font-mono text-[11px] text-tui-dim">
          Agent bulunamadı.{' '}
          <Link to="/agentlar" className="text-rx hover:underline">
            Agent listesine dön
          </Link>
        </p>
      </div>
    )
  }

  if (!loaded || !agent) {
    return (
      <div className="mx-auto max-w-[1600px] px-4 py-10">
        <p className="text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <Link to="/agentlar" className="text-[11px] text-tui-dim hover:text-rx">
        ← Agent'lar
      </Link>

      {dataStale && (
        <p className="border border-amber-500/40 bg-amber-500/10 px-3 py-1.5 text-[11px] text-amber-400">
          ⚠ Veriler güncellenemiyor — bağlantı sorunu olabilir, gösterilenler son başarılı polldan.
        </p>
      )}

      {/* başlık */}
      <div className="flex flex-wrap items-center gap-2">
        <StatusPill tone={agent.online ? 'emerald' : 'slate'} label={agent.online ? 'online' : 'offline'} />
        {renaming ? (
          <>
            <input
              autoFocus
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void submitRename()
                if (e.key === 'Escape') cancelRename()
              }}
              aria-label="Yeni agent adı"
              className="border border-rx/50 bg-ground px-2 py-0.5 text-[13px] text-ink-hi outline-none"
            />
            <button
              type="button"
              onClick={() => void submitRename()}
              disabled={renameBusy}
              className="border border-rx/40 bg-rx/10 px-2 py-0.5 text-[11px] uppercase text-rx transition hover:bg-rx/20 disabled:opacity-50"
            >
              {renameBusy ? 'kaydediliyor…' : 'Kaydet'}
            </button>
            <button
              type="button"
              onClick={cancelRename}
              className="border border-rule-hi px-2 py-0.5 text-[11px] uppercase text-tui-dim transition hover:text-ink-hi"
            >
              Vazgeç
            </button>
          </>
        ) : (
          <>
            <h1 className="text-[13px] font-bold uppercase tracking-[0.04em] text-ink-hi">{agent.name}</h1>
            {agent.site && <span className="border border-rule px-1.5 py-0.5 text-[11px] text-tui-dim">{agent.site}</span>}
            <button
              type="button"
              onClick={startRename}
              className="border border-rule-hi px-2 py-0.5 text-[11px] uppercase text-tui-dim transition hover:border-rx/40 hover:text-rx"
            >
              Adı Değiştir
            </button>
          </>
        )}
        <button
          type="button"
          onClick={handleDeleteClick}
          disabled={deleting}
          aria-label={confirmDelete ? `${agent.name} silinsin mi? Onaylamak için tekrar tıklayın` : `${agent.name} agent'ını sil`}
          title={confirmDelete ? `${agent.name} silinsin mi? Onaylamak için tekrar tıklayın` : `${agent.name} agent'ını sil`}
          className={`ml-auto border px-2 py-0.5 text-[11px] uppercase transition disabled:opacity-50 ${
            confirmDelete ? 'border-rose-400 bg-rose-400 text-ground' : 'border-rose-500/40 text-rose-400 hover:bg-rose-500/10'
          }`}
        >
          {deleting ? 'siliniyor…' : confirmDelete ? 'emin misiniz?' : "Agent'ı Sil"}
        </button>
      </div>
      {renameError && (
        <p className="border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-[11px] text-rose-400">⚠ Yeniden adlandırma başarısız: {renameError}</p>
      )}
      {deleteError && (
        <p className="border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-[11px] text-rose-400">⚠ Silme başarısız: {deleteError}</p>
      )}
      {confirmDelete && (
        <p className="text-[10px] text-tui-dim">
          "{agent.name}" silinsin mi? Agent'ın tüm telemetrisi de silinir. Bu işlem geri alınamaz — onaylamak için "Agent'ı Sil" butonuna tekrar tıklayın.
        </p>
      )}

      {/* özet + trafik geçmişi */}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1.6fr]">
        <Panel title="Özet">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-[11px]">
            {([
              ['IP', agent.remote_ip || '—'],
              ['Sürüm', `${agent.version || '—'} · pv${agent.protocol_version}`],
              ['Atıf', <AttrMethodBadge key="am" method={agent.attr_method} />],
              ['Bağlantı', formatNum(agent.conns)],
              ['Arayüz', String(agent.rates?.length ?? 0)],
              ['İlk Görülme', relTime(agent.first_seen)],
              ['Son Görülme', relTime(agent.last_seen)],
            ] as [string, ReactNode][]).map(([k, v]) => (
              <div key={k} className="min-w-0">
                <dt className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">{k}</dt>
                <dd className="truncate text-ink-hi">{v}</dd>
              </div>
            ))}
          </dl>
          <p className="mt-3 border-t border-rule pt-2 text-[11px]">
            <span className="text-rx">↓ {formatBits(ifaceTotal.rx * 8)}</span>
            <span className="mx-2 text-rule-hi">|</span>
            <span className="text-tx">↑ {formatBits(ifaceTotal.tx * 8)}</span>
            <span className="ml-2 text-[10px] text-tui-dim">tüm arayüzler</span>
          </p>
        </Panel>

        <Panel title="Trafik Geçmişi" right={<RangeTabs ranges={RANGES} value={minutes} onChange={setMinutes} />}>
          <ThroughputChart history={history} running={agent.online} rangeMinutes={minutes} subtitle={`son ${minutes} dk · tüm arayüzler toplamı`} />
        </Panel>
      </div>

      {/* arayüzler */}
      <Panel title="Arayüzler" right={<span className="text-[10px] text-tui-dim">{agent.rates?.length ?? 0} arayüz</span>}>
        {!agent.rates || agent.rates.length === 0 ? (
          <p className="py-6 text-center text-[11px] text-tui-dim">Henüz arayüz verisi yok.</p>
        ) : (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {agent.rates.map((r) => (
              <div key={r.name} className="border border-rule bg-panel-2/40 p-2.5">
                <p className="truncate text-[13px] font-semibold text-ink-hi">{r.name}</p>
                <p className="mt-1 text-[11px]">
                  <span className="text-rx">↓ {formatBits(r.rx_bps * 8)}</span>
                  <span className="mx-1.5 text-rule-hi">|</span>
                  <span className="text-tx">↑ {formatBits(r.tx_bps * 8)}</span>
                </p>
                <p className="mt-1 text-[10px] text-tui-dim">
                  toplam {formatBytes(r.rx_bytes + r.tx_bytes)} · {formatNum(r.rx_packets + r.tx_packets)} pkt · {Math.round(r.pps)} pps
                </p>
              </div>
            ))}
          </div>
        )}
      </Panel>

      <Panel title="Süreç Trafiği" right={<span className="text-[10px] text-tui-dim">bu agent</span>}>
        <ProcessesCard agentId={agent.id} />
      </Panel>
      <Panel title="Uygulama Görünürlüğü" right={<span className="text-[10px] text-tui-dim">L7 · SNI + HTTP Host</span>}>
        <L7Card agentId={agent.id} />
      </Panel>
      <Panel title="DNS Görünürlüğü" right={<span className="text-[10px] text-tui-dim">UDP/53 · süreç atıflı</span>}>
        <DnsCard agentId={agent.id} />
      </Panel>

      {/* bağlantılar */}
      <Panel title="Bağlantılar" right={<span className="text-[10px] text-tui-dim">son telemetri anı · {formatNum(connections.length)} bağlantı</span>}>
        {connections.length === 0 ? (
          <p className="py-6 text-center text-[11px] text-tui-dim">Bağlantı yok.</p>
        ) : (
          <TuiTable
            columns={connCols}
            rows={connections}
            getKey={(c) => `${c.proto}-${c.local_addr}-${c.remote_addr ?? ''}-${c.pid}-${c.status ?? ''}`}
            filterText={(c) => `${c.local_addr} ${c.remote_addr ?? ''} ${c.process ?? ''} ${c.status ?? ''} ${c.pid}`}
            filterLabel="Filtrele: adres, süreç, durum…"
            maxRows={300}
            scrollClass="max-h-96"
            className="border-0"
          />
        )}
      </Panel>
    </div>
  )
}
