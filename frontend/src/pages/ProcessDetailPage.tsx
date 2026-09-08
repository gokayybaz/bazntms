import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { useHotkeys } from '../lib/useHotkeys'
import { Panel } from '../components/Panel'
import { RangeTabs } from '../components/RangeTabs'
import { TuiTable } from '../components/TuiTable'
import type { TuiColumn } from '../components/TuiTable'

interface ProcessSummary {
  process: string
  pids: number[]
  first_seen: number
  last_seen: number
  bytes_in: number
  bytes_out: number
  total: number
  rx_bps: number
  tx_bps: number
}
interface ProcessRemote {
  remote_ip: string
  port: number
  proto: string
  bytes_in: number
  bytes_out: number
  first_seen: number
  last_seen: number
  conns: number
  country?: string
  asn?: string
}
interface ProcessConn {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
  pid: number
  process?: string
}
interface AppObservation {
  type: string // dns | tls | http
  host: string
  observations: number
  bytes: number
  first_seen: number
  last_seen: number
}
interface TimelineEntry {
  ts: number
  event: string
  target: string
  detail?: string
}
interface ProcessDetail {
  process: ProcessSummary
  remotes: ProcessRemote[]
  connections: ProcessConn[]
  app_visibility: AppObservation[]
  timeline: TimelineEntry[]
}

const RANGES = [
  { label: '15 dk', value: 15 },
  { label: '1 saat', value: 60 },
  { label: '6 saat', value: 360 },
  { label: '24 saat', value: 1440 },
] as const

function relTime(unix: number): string {
  if (!unix) return '—'
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h} sa önce`
  return `${Math.floor(h / 24)} gün önce`
}
function clockTime(unix: number): string {
  if (!unix) return '—'
  return new Date(unix * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const EVENT_LABELS: Record<string, string> = {
  'process.first_seen': 'süreç ilk görüldü',
  'dns.query': 'DNS sorgusu',
  'tls.sni': 'TLS SNI',
  'http.host': 'HTTP Host',
  'traffic.spike': 'trafik sıçraması',
  alert: 'uyarı',
}
const EVENT_TONE: Record<string, string> = {
  'process.first_seen': 'text-ink-hi',
  'dns.query': 'text-tui-dim',
  'tls.sni': 'text-emerald-400',
  'http.host': 'text-amber-400',
  'traffic.spike': 'text-tx',
  alert: 'text-rose-400',
}

export function ProcessDetailPage() {
  const { id, ad } = useParams<{ id: string; ad: string }>()
  const navigate = useNavigate()
  const [minutes, setMinutes] = useState<15 | 60 | 360 | 1440>(60)
  const backTo = `/agentlar/${id ?? ''}`

  useHotkeys([{ key: 'Escape', handler: () => navigate(backTo), allowInField: true }])

  const [data, setData] = useState<ProcessDetail | null>(null)
  const [state, setState] = useState<'loading' | 'ok' | 'notfound'>('loading')

  useEffect(() => {
    if (!id || !ad) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/agents/${id}/processes/${encodeURIComponent(ad)}?minutes=${minutes}`)
        if (res.status === 401) return
        if (res.status === 404) {
          if (!stop) setState('notfound')
          return
        }
        if (!res.ok) return // geçici hata — bir sonraki poll düzeltir
        const json: ProcessDetail = await res.json()
        if (!stop) {
          setData(json)
          setState('ok')
        }
      } catch {
        /* yoksay — poll tekrar dener */
      }
    }
    load()
    const t = window.setInterval(load, 10_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id, ad, minutes])

  const remoteCols: TuiColumn<ProcessRemote>[] = useMemo(
    () => [
      {
        key: 'remote_ip',
        header: 'Hedef',
        sortable: true,
        render: (r) => (
          <span className="flex flex-wrap items-center gap-1.5">
            <span className="text-ink-hi">{r.remote_ip || '—'}</span>
            {r.country && <span className="border border-rule px-1 text-[10px] text-tui-dim">{r.country}</span>}
            {r.asn && <span className="text-[10px] text-tui-dim">{r.asn}</span>}
          </span>
        ),
      },
      { key: 'port', header: 'Port', width: '4.5rem', align: 'right', sortable: true, render: (r) => <span className="text-tui-dim">{r.port || '—'}</span> },
      { key: 'proto', header: 'Proto', width: '4rem', sortable: true, render: (r) => <span className="uppercase text-tui-dim">{r.proto || '—'}</span> },
      { key: 'bytes_in', header: 'İndir', align: 'right', sortable: true, sortValue: (r) => r.bytes_in, render: (r) => <span className="text-rx">{formatBytes(r.bytes_in)}</span> },
      { key: 'bytes_out', header: 'Gönder', align: 'right', sortable: true, sortValue: (r) => r.bytes_out, render: (r) => <span className="text-tx">{formatBytes(r.bytes_out)}</span> },
      { key: 'conns', header: 'Bağ.', width: '4rem', align: 'right', sortable: true, sortValue: (r) => r.conns, render: (r) => <span className="text-tui-dim">{r.conns || '—'}</span> },
      { key: 'last_seen', header: 'Son', width: '7rem', align: 'right', sortable: true, sortValue: (r) => r.last_seen, render: (r) => <span className="text-tui-dim">{relTime(r.last_seen)}</span> },
    ],
    [],
  )

  const connCols: TuiColumn<ProcessConn>[] = useMemo(
    () => [
      { key: 'proto', header: 'Proto', width: '4rem', sortable: true, render: (c) => <span className="uppercase text-tui-dim">{c.proto}</span> },
      { key: 'local_addr', header: 'Yerel Adres', sortable: true, render: (c) => <span className="text-ink">{c.local_addr}</span> },
      { key: 'remote_addr', header: 'Uzak Adres', sortable: true, render: (c) => <span className="text-tui-dim">{c.remote_addr || '—'}</span> },
      { key: 'status', header: 'Durum', width: '9rem', sortable: true, render: (c) => <span className="text-tui-dim">{c.status || '—'}</span> },
      // yaş türetilemiyor — agent_conn_latest'te timestamp yok (bilinen sınır)
      { key: 'age', header: 'Yaş', width: '4rem', align: 'right', render: () => <span className="text-tui-dim">—</span> },
    ],
    [],
  )

  const appCols: TuiColumn<AppObservation>[] = useMemo(
    () => [
      {
        key: 'type',
        header: 'Tür',
        width: '4rem',
        sortable: true,
        render: (o) => (
          <span className={`uppercase ${o.type === 'dns' ? 'text-tui-dim' : o.type === 'tls' ? 'text-emerald-400' : 'text-amber-400'}`}>{o.type}</span>
        ),
      },
      { key: 'host', header: 'Alan Adı', sortable: true, render: (o) => <span className="text-ink-hi">{o.host}</span> },
      { key: 'observations', header: 'Gözlem', align: 'right', sortable: true, sortValue: (o) => o.observations, render: (o) => <span className="text-tui-dim">{formatNum(o.observations)}</span> },
      { key: 'bytes', header: 'Veri', align: 'right', sortable: true, sortValue: (o) => o.bytes, render: (o) => <span className="text-tui-dim">{o.bytes ? formatBytes(o.bytes) : '—'}</span> },
      { key: 'last_seen', header: 'Son', width: '7rem', align: 'right', sortable: true, sortValue: (o) => o.last_seen, render: (o) => <span className="text-tui-dim">{relTime(o.last_seen)}</span> },
    ],
    [],
  )

  const p = data?.process

  if (!id || !ad) return null

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex flex-wrap items-center gap-2 text-[11px]">
        <Link to={backTo} className="text-tui-dim hover:text-rx">
          ← Agent
        </Link>
        <span className="text-rule-hi">/</span>
        <span className="text-ink-hi">Süreç: {decodeURIComponent(ad)}</span>
        <span className="ml-auto">
          <RangeTabs ranges={RANGES} value={minutes} onChange={setMinutes} />
        </span>
      </div>

      {state === 'loading' ? (
        <p className="py-10 text-center text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : state === 'notfound' || !data || !p ? (
        <p className="py-10 text-center text-[11px] text-tui-dim">
          Bu süreç seçili pencerede görülmedi.{' '}
          <Link to={backTo} className="text-rx hover:underline">
            Agent'a dön
          </Link>{' '}
          — ya da agent'ta <code className="text-tui-dim">-pcap</code> açık mı kontrol edin.
        </p>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1.4fr]">
            <Panel title="Özet">
              <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-[11px]">
                {(
                  [
                    ['Süreç', p.process || 'bilinmeyen'],
                    ['PID', p.pids.length ? p.pids.join(', ') : '—'],
                    ['İlk Görülme', relTime(p.first_seen)],
                    ['Son Görülme', relTime(p.last_seen)],
                    ['Hedef', formatNum(data.remotes.length)],
                    ['Canlı Bağlantı', formatNum(data.connections.length)],
                  ] as [string, string][]
                ).map(([k, v]) => (
                  <div key={k} className="min-w-0">
                    <dt className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">{k}</dt>
                    <dd className="truncate text-ink-hi">{v}</dd>
                  </div>
                ))}
              </dl>
              <div className="mt-3 space-y-1 border-t border-rule pt-2">
                <div className="text-[11px]">
                  <span className="text-rx">↓ {formatBytes(p.bytes_in)}</span>
                  <span className="mx-2 text-rule-hi">|</span>
                  <span className="text-tx">↑ {formatBytes(p.bytes_out)}</span>
                  <span className="mx-2 text-rule-hi">|</span>
                  <span className="text-ink-hi">Σ {formatBytes(p.total)}</span>
                </div>
                <div className="text-[11px]">
                  <span className="text-tui-dim">anlık: </span>
                  <span className="text-rx">↓ {formatBits(p.rx_bps * 8)}</span>
                  <span className="mx-2 text-rule-hi">|</span>
                  <span className="text-tx">↑ {formatBits(p.tx_bps * 8)}</span>
                </div>
              </div>
            </Panel>

            <Panel title="Zaman Çizelgesi" right={<span className="text-[10px] text-tui-dim">{data.timeline.length} olay</span>}>
              {data.timeline.length === 0 ? (
                <p className="py-6 text-center text-[11px] text-tui-dim">Bu pencerede olay yok.</p>
              ) : (
                <ol className="max-h-72 space-y-1 overflow-y-auto text-[11px]">
                  {data.timeline.map((e, i) => (
                    <li key={`${e.ts}-${e.event}-${i}`} className="flex gap-2">
                      <span className="w-16 shrink-0 text-tui-dim">{clockTime(e.ts)}</span>
                      <span className={`w-28 shrink-0 ${EVENT_TONE[e.event] ?? 'text-tui-dim'}`}>{EVENT_LABELS[e.event] ?? e.event}</span>
                      <span className="min-w-0 flex-1 truncate text-ink" title={e.detail || e.target}>
                        {e.target}
                        {e.detail && <span className="ml-1.5 text-tui-dim">· {e.detail}</span>}
                      </span>
                    </li>
                  ))}
                </ol>
              )}
            </Panel>
          </div>

          <Panel title="Uzak Hedefler" right={<span className="text-[10px] text-tui-dim">ip:port / proto bazında</span>}>
            {data.remotes.length === 0 ? (
              <p className="py-6 text-center text-[11px] text-tui-dim">Uzak hedef yok.</p>
            ) : (
              <TuiTable
                columns={remoteCols}
                rows={data.remotes}
                getKey={(r) => `${r.remote_ip}-${r.port}-${r.proto}`}
                filterText={(r) => `${r.remote_ip} ${r.proto} ${r.country ?? ''} ${r.asn ?? ''}`}
                filterLabel="Hedef filtrele…"
                initialSort={{ key: 'bytes_in', dir: 'desc' }}
                scrollClass="max-h-96"
                className="border-0"
              />
            )}
          </Panel>

          <Panel title="Uygulama Görünürlüğü" right={<span className="text-[10px] text-tui-dim">DNS + TLS SNI + HTTP Host</span>}>
            {data.app_visibility.length === 0 ? (
              <p className="py-6 text-center text-[11px] text-tui-dim">
                Bu sürece atfedilen alan adı yok — DNS/L7 görünürlüğü pcap gerektirir.
              </p>
            ) : (
              <TuiTable
                columns={appCols}
                rows={data.app_visibility}
                getKey={(o) => `${o.type}-${o.host}`}
                filterText={(o) => `${o.host} ${o.type}`}
                filterLabel="Alan adı filtrele…"
                initialSort={{ key: 'observations', dir: 'desc' }}
                scrollClass="max-h-80"
                className="border-0"
              />
            )}
          </Panel>

          <Panel title="Bağlantılar" right={<span className="text-[10px] text-tui-dim">son telemetri anı · {formatNum(data.connections.length)} bağlantı</span>}>
            {data.connections.length === 0 ? (
              <p className="py-6 text-center text-[11px] text-tui-dim">Bu sürece ait açık bağlantı yok.</p>
            ) : (
              <TuiTable
                columns={connCols}
                rows={data.connections}
                getKey={(c) => `${c.proto}-${c.local_addr}-${c.remote_addr ?? ''}`}
                filterText={(c) => `${c.local_addr} ${c.remote_addr ?? ''} ${c.status ?? ''}`}
                filterLabel="Bağlantı filtrele…"
                scrollClass="max-h-96"
                className="border-0"
              />
            )}
          </Panel>
        </>
      )}
    </div>
  )
}
