import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { AgentWithRates, Bucket } from '../types'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { Card } from '../components/Card'
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
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
  { label: '24 saat', minutes: 1440 },
]

// silme onayı bekleme süresi (ms) — DevicesCard.tsx'teki iki-aşamalı silme
// deseniyle aynı: ilk tık "emin misiniz?" durumuna alır, bu sürede ikinci
// tık gelmezse normale döner
const DELETE_CONFIRM_MS = 4000

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
  const navigate = useNavigate()
  const [agent, setAgent] = useState<AgentWithRates | null>(null)
  const [connections, setConnections] = useState<AgentConnSample[]>([])
  const [history, setHistory] = useState<Bucket[]>([])
  const [minutes, setMinutes] = useState(60)
  const [loaded, setLoaded] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [connQuery, setConnQuery] = useState('')
  // ana veri (agent+bağlantılar) pollu başarısız olursa görünür bir uyarı —
  // eskiden sessizce yutuluyordu, sayfa "canlı" görünmeye devam ederdi
  const [dataStale, setDataStale] = useState(false)
  // yeniden adlandırma/silme — eskiden native prompt()/confirm()/alert()
  // kullanıyordu, uygulamanın geri kalanının satır-içi rose-banner diline
  // uymuyordu (impeccable critique 2026-09-05); artık DevicesCard.tsx'teki
  // iki-aşamalı silme + satır-içi düzenleme desenleriyle aynı
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
        /* yoksay — throughput grafiği ikincil, ayrı bir donukluk göstergesi
           eklemeye değmez, ana veri (yukarıdaki effect) zaten kapsıyor */
      }
    }
    load()
    const t = window.setInterval(load, 15_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id, minutes])

  useEffect(() => () => {
    if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
  }, [])

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
        <p className="py-16 text-center text-sm text-dim-aa">
          Agent bulunamadı. <Link to="/agentlar" className="text-cyan-400 hover:underline">Agent listesine dön</Link>
        </p>
      </div>
    )
  }

  if (!loaded || !agent) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <p className="py-16 text-center text-sm text-dim-aa">Yükleniyor…</p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <Link to="/agentlar" className="text-xs text-dim-aa hover:text-cyan-400">
          ← Agent'lar
        </Link>
      </div>

      {dataStale && (
        <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-400">
          ⚠ Veriler güncellenemiyor — bağlantı sorunu olabilir, gösterilenler son başarılı polldan.
        </p>
      )}

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
              className="rounded-md border border-cyan-500/50 bg-slate-900 px-2 py-1 font-mono text-sm text-slate-100 outline-none"
            />
            <button
              type="button"
              onClick={() => void submitRename()}
              disabled={renameBusy}
              className="rounded border border-cyan-500/40 bg-cyan-500/10 px-2 py-0.5 text-xs text-cyan-300 transition hover:bg-cyan-500/20 disabled:opacity-50"
            >
              {renameBusy ? 'kaydediliyor…' : 'Kaydet'}
            </button>
            <button
              type="button"
              onClick={cancelRename}
              className="rounded border border-slate-700 px-2 py-0.5 text-xs text-slate-400 transition hover:text-slate-200"
            >
              Vazgeç
            </button>
          </>
        ) : (
          <>
            <h1 className="font-mono text-xl font-bold text-slate-100">{agent.name}</h1>
            {agent.site && <span className="rounded bg-slate-800 px-2 py-0.5 font-mono text-xs text-slate-400">{agent.site}</span>}
            <button
              type="button"
              onClick={startRename}
              className="rounded border border-slate-700 px-2 py-0.5 text-xs text-slate-400 transition hover:border-cyan-500/40 hover:text-cyan-300"
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
          className={`ml-auto rounded border px-2 py-0.5 text-xs transition disabled:opacity-50 ${
            confirmDelete
              ? 'border-rose-500 bg-rose-500/20 text-rose-200'
              : 'border-rose-900/50 text-rose-400 hover:border-rose-500/50 hover:bg-rose-500/10'
          }`}
        >
          {deleting ? 'siliniyor…' : confirmDelete ? 'emin misiniz?' : "Agent'ı Sil"}
        </button>
      </div>
      {renameError && (
        <p className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-400">⚠ Yeniden adlandırma başarısız: {renameError}</p>
      )}
      {deleteError && (
        <p className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-400">⚠ Silme başarısız: {deleteError}</p>
      )}
      {confirmDelete && (
        <p className="text-[11px] text-dim-aa">"{agent.name}" silinsin mi? Agent'ın tüm telemetrisi de silinir. Bu işlem geri alınamaz — onaylamak için "Agent'ı Sil" butonuna tekrar tıklayın.</p>
      )}

      {/* özet şeridi */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">IP Adresi</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200">{agent.remote_ip || '—'}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Sürüm</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200" title={`protokol v${agent.protocol_version}`}>
            {agent.version || '—'}
            <span className="ml-1.5 text-[11px] text-dim-aa">pv{agent.protocol_version}</span>
          </p>
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
      <Card title="Arayüzler" right={<span className="text-xs text-dim-aa">{agent.rates?.length ?? 0} arayüz</span>}>
        {!agent.rates || agent.rates.length === 0 ? (
          <p className="py-6 text-center text-sm text-dim-aa">Henüz arayüz verisi yok.</p>
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
                <p className="mt-1 font-mono text-[10.5px] text-dim-aa">
                  toplam {formatBytes(r.rx_bytes + r.tx_bytes)} · {formatNum(r.rx_packets + r.tx_packets)} pkt · {Math.round(r.pps)} pps
                </p>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* süreç trafiği */}
      <Card title="Süreç Trafiği" right={<span className="text-xs text-dim-aa">bu agent</span>}>
        <ProcessesCard agentId={agent.id} />
      </Card>

      {/* uygulama görünürlüğü (L7) */}
      <Card title="Uygulama Görünürlüğü" right={<span className="text-xs text-dim-aa">L7 · SNI + HTTP Host</span>}>
        <L7Card agentId={agent.id} />
      </Card>

      {/* DNS görünürlüğü */}
      <Card title="DNS Görünürlüğü" right={<span className="text-xs text-dim-aa">UDP/53 · süreç atıflı</span>}>
        <DnsCard agentId={agent.id} />
      </Card>

      {/* bağlantılar */}
      <Card
        title="Bağlantılar"
        right={
          <span className="text-xs text-dim-aa">
            son telemetri anı · {formatNum(filteredConns.length)}
            {connQuery && ` / ${formatNum(connections.length)}`} bağlantı
          </span>
        }
      >
        <input
          value={connQuery}
          onChange={(e) => setConnQuery(e.target.value)}
          placeholder="Filtrele: adres, süreç, durum…"
          aria-label="Bağlantı filtresi"
          className="mb-3 w-64 rounded-lg border border-slate-700/80 bg-slate-900 px-3 py-1.5 text-sm outline-none placeholder:text-dim-aa focus:border-cyan-500/60"
        />
        {filteredConns.length > 300 && (
          <p className="mb-2 text-[10px] text-amber-400">ilk 300 / {formatNum(filteredConns.length)} bağlantı gösteriliyor</p>
        )}
        {filteredConns.length === 0 ? (
          <p className="py-6 text-center text-sm text-dim-aa">Eşleşen bağlantı yok.</p>
        ) : (
          <div className="max-h-96 overflow-y-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
                <tr className="text-left text-[11px] uppercase tracking-wider text-dim-aa">
                  <th scope="col" className="px-3 py-2 font-medium">Protokol</th>
                  <th scope="col" className="px-3 py-2 font-medium">Yerel Adres</th>
                  <th scope="col" className="px-3 py-2 font-medium">Uzak Adres</th>
                  <th scope="col" className="px-3 py-2 font-medium">Durum</th>
                  <th scope="col" className="px-3 py-2 font-medium">Süreç</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {filteredConns.slice(0, 300).map((c, i) => (
                  <tr key={`${c.proto}-${c.local_addr}-${c.remote_addr ?? ''}-${i}`} className="hover:bg-slate-800/30">
                    <td className="px-3 py-1.5">
                      <span
                        className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-bold uppercase ${
                          c.proto === 'udp' ? 'bg-slate-800 text-slate-400' : 'bg-cyan-500/10 text-cyan-400'
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
                          {c.pid > 0 && <span className="ml-1 font-mono text-[10px] text-dim-aa">[{c.pid}]</span>}
                        </>
                      ) : (
                        <span className="text-dim-aa">—</span>
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
