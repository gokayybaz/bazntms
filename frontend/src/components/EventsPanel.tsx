import { useCallback, useEffect, useMemo, useState } from 'react'
import { formatNum } from '../lib/format'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

// Faz 24-A: normalleştirilmiş ham olay akışı (uyarılardan ayrı — ADR 0010).
// Salt-okunur, filtrelenebilir, imleçli "daha fazla".

interface Event {
  type: string
  source: string
  ts: number
  agent_id?: number
  device?: string
  process?: string
  pid?: number
  src_ip?: string
  src_port?: number
  dst_ip?: string
  dst_port?: number
  proto?: string
  domain?: string
  severity?: string
  count?: number
}

const TYPES = [
  { v: 'dns.query', label: 'DNS' },
  { v: 'tls.sni_observed', label: 'TLS SNI' },
  { v: 'http.host_observed', label: 'HTTP Host' },
  { v: 'netflow.flow', label: 'NetFlow' },
  { v: 'syslog.received', label: 'Syslog' },
  { v: 'connection.seen', label: 'Bağlantı' },
] as const

const TYPE_TONE: Record<string, string> = {
  'dns.query': 'text-tui-dim',
  'tls.sni_observed': 'text-emerald-400',
  'http.host_observed': 'text-amber-400',
  'netflow.flow': 'text-sky-400',
  'syslog.received': 'text-tui-dim',
  'connection.seen': 'text-ink',
}

function clock(ts: number) {
  return new Date(ts * 1000).toLocaleTimeString('tr-TR')
}

export function EventsPanel({ agentId }: { agentId?: number } = {}) {
  const [sel, setSel] = useState<Set<string>>(new Set())
  const [rows, setRows] = useState<Event[]>([])
  const [next, setNext] = useState(0)
  const [loading, setLoading] = useState(true)

  const typeParam = useMemo(() => (sel.size ? `&type=${[...sel].join(',')}` : ''), [sel])
  const baseUrl = `/api/v1/events?since_min=180&limit=100${agentId ? `&agent_id=${agentId}` : ''}${typeParam}`

  const load = useCallback(
    async (before?: number) => {
      try {
        const res = await fetch(before ? `${baseUrl}&before=${before}` : baseUrl)
        if (!res.ok) return
        const data: { events: Event[]; next: number } = await res.json()
        setRows((cur) => (before ? [...cur, ...(data.events ?? [])] : (data.events ?? [])))
        setNext(data.next ?? 0)
      } finally {
        setLoading(false)
      }
    },
    [baseUrl],
  )

  useEffect(() => {
    setLoading(true)
    void load()
    const t = window.setInterval(() => void load(), 20_000)
    return () => window.clearInterval(t)
  }, [load])

  const toggle = (v: string) =>
    setSel((cur) => {
      const n = new Set(cur)
      n.has(v) ? n.delete(v) : n.add(v)
      return n
    })

  const cols: TuiColumn<Event>[] = [
    { key: 'ts', header: 'Saat', width: '6rem', sortable: true, sortValue: (e) => e.ts, render: (e) => <span className="text-tui-dim">{clock(e.ts)}</span> },
    {
      key: 'type',
      header: 'Tür',
      width: '9rem',
      sortable: true,
      render: (e) => <span className={`uppercase ${TYPE_TONE[e.type] ?? 'text-tui-dim'}`}>{e.type}</span>,
    },
    {
      key: 'origin',
      header: 'Kaynak',
      sortable: true,
      sortValue: (e) => e.agent_id ?? e.device ?? '',
      render: (e) => <span className="text-tui-dim">{e.agent_id ? `agent#${e.agent_id}` : e.device || e.source}</span>,
    },
    { key: 'process', header: 'Süreç', sortable: true, render: (e) => <span className="text-ink">{e.process || '—'}</span> },
    {
      key: 'target',
      header: 'Hedef',
      render: (e) => (
        <span className="text-ink-hi">
          {e.domain || (e.dst_ip ? `${e.dst_ip}${e.dst_port ? `:${e.dst_port}` : ''}` : '—')}
          {e.proto && <span className="ml-1 text-[10px] uppercase text-tui-dim">{e.proto}</span>}
        </span>
      ),
    },
    { key: 'count', header: 'N', width: '3.5rem', align: 'right', sortable: true, sortValue: (e) => e.count ?? 0, render: (e) => <span className="text-tui-dim">{e.count ? formatNum(e.count) : ''}</span> },
  ]

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-1.5 font-mono text-[10px]">
        {TYPES.map((t) => (
          <button
            key={t.v}
            type="button"
            onClick={() => toggle(t.v)}
            aria-pressed={sel.has(t.v)}
            className={`border px-2 py-0.5 uppercase ${sel.has(t.v) ? 'border-rx bg-rx text-ground' : 'border-rule text-tui-dim hover:text-ink-hi'}`}
          >
            {t.label}
          </button>
        ))}
        <span className="ml-auto text-tui-dim">son 3 saat · uyarılardan ayrı ham gözlem akışı (ADR 0010)</span>
      </div>

      {loading && rows.length === 0 ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Bu pencerede olay yok.</p>
      ) : (
        <>
          <TuiTable
            columns={cols}
            rows={rows}
            getKey={(e) =>
              `${e.type}|${e.ts}|${e.agent_id ?? e.device ?? ''}|${e.process ?? ''}|${e.domain ?? e.dst_ip ?? ''}|${e.src_port ?? ''}|${e.dst_port ?? ''}`
            }
            filterText={(e) => `${e.type} ${e.process ?? ''} ${e.domain ?? ''} ${e.dst_ip ?? ''} ${e.device ?? ''}`}
            filterLabel="Olay filtrele…"
            initialSort={{ key: 'ts', dir: 'desc' }}
            scrollClass="max-h-[32rem]"
            className="border-0"
          />
          {next > 0 && (
            <button
              type="button"
              onClick={() => void load(next)}
              className="mt-2 w-full border border-rule py-1 font-mono text-[10px] uppercase text-tui-dim hover:text-ink-hi"
            >
              daha fazla
            </button>
          )}
        </>
      )}
    </div>
  )
}
