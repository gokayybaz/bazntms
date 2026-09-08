import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { formatNum } from '../lib/format'
import { usePolledJson } from '../lib/usePolledJson'
import { PanelState } from './PanelState'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

// Faz 24-B/C: korele uyarı kümeleri (incident) listesi.

export interface Incident {
  id: number
  title: string
  severity: 'info' | 'warn' | 'crit'
  status: 'open' | 'investigating' | 'resolved' | 'closed'
  site: string
  agent_id: number
  correlation_reason: string
  risk_score: number
  first_seen: number
  last_seen: number
}

const SEV_TONE: Record<string, string> = { crit: 'text-rose-400', warn: 'text-amber-400', info: 'text-sky-400' }
const STATUS_LABEL: Record<string, string> = { open: 'açık', investigating: 'inceleniyor', resolved: 'çözüldü', closed: 'kapandı' }
const STATUS_TONE: Record<string, string> = { open: 'text-rose-400', investigating: 'text-amber-400', resolved: 'text-emerald-400', closed: 'text-tui-dim' }

const FILTERS = [
  { v: 'open', label: 'Açık' },
  { v: 'resolved', label: 'Çözüldü' },
  { v: '', label: 'Tümü' },
] as const

function relTime(unix: number): string {
  if (!unix) return '—'
  const s = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (s < 60) return `${s} sn`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} dk`
  const h = Math.floor(m / 60)
  return h < 48 ? `${h} sa` : `${Math.floor(h / 24)} g`
}

export function IncidentsPanel() {
  const navigate = useNavigate()
  const [filter, setFilter] = useState<string>('open')
  const { data, loaded } = usePolledJson<{ incidents: Incident[] }>(
    `/api/v1/incidents?limit=100${filter ? `&status=${filter}` : ''}`,
    15_000,
  )
  const rows = useMemo(() => data?.incidents ?? [], [data])

  const cols: TuiColumn<Incident>[] = [
    { key: 'severity', header: 'Sev', width: '3.5rem', sortable: true, render: (i) => <span className={`uppercase ${SEV_TONE[i.severity]}`}>{i.severity}</span> },
    { key: 'status', header: 'Durum', width: '7rem', sortable: true, render: (i) => <span className={`uppercase ${STATUS_TONE[i.status]}`}>{STATUS_LABEL[i.status] ?? i.status}</span> },
    { key: 'title', header: 'Başlık', sortable: true, render: (i) => <span className="text-ink-hi">{i.title}</span> },
    { key: 'agent_id', header: 'Agent', width: '5rem', align: 'right', sortable: true, render: (i) => <span className="text-tui-dim">{i.agent_id ? `#${i.agent_id}` : '—'}</span> },
    { key: 'risk_score', header: 'Risk', width: '4rem', align: 'right', sortable: true, sortValue: (i) => i.risk_score, render: (i) => <span className={i.risk_score >= 70 ? 'text-rose-400' : i.risk_score >= 40 ? 'text-amber-400' : 'text-tui-dim'}>{i.risk_score}</span> },
    { key: 'first_seen', header: 'İlk', width: '5rem', align: 'right', sortable: true, sortValue: (i) => i.first_seen, render: (i) => <span className="text-tui-dim">{relTime(i.first_seen)}</span> },
    { key: 'last_seen', header: 'Son', width: '5rem', align: 'right', sortable: true, sortValue: (i) => i.last_seen, render: (i) => <span className="text-tui-dim">{relTime(i.last_seen)}</span> },
  ]

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-1.5 font-mono text-[10px]">
        {FILTERS.map((f) => (
          <button
            key={f.v}
            type="button"
            onClick={() => setFilter(f.v)}
            aria-pressed={filter === f.v}
            className={`border px-2 py-0.5 uppercase ${filter === f.v ? 'border-rx bg-rx text-ground' : 'border-rule text-tui-dim hover:text-ink-hi'}`}
          >
            {f.label}
          </button>
        ))}
        <span className="ml-auto text-tui-dim">{formatNum(rows.length)} olay · deterministik korelasyon (5 kural, AI yok) · Enter → detay</span>
      </div>

      {!loaded ? (
        <PanelState kind="loading" />
      ) : rows.length === 0 ? (
        <PanelState
          kind="empty"
          message={filter === 'open' ? 'Açık olay yok.' : 'Bu filtrede olay yok.'}
          hint={filter === 'open' ? 'ilişkili uyarılar kısa aralıkta gelince motor bir olay açar' : undefined}
        />
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(i) => String(i.id)}
          filterText={(i) => `${i.title} ${i.correlation_reason} ${i.severity} ${i.status}`}
          filterLabel="Olay filtrele…"
          initialSort={{ key: 'last_seen', dir: 'desc' }}
          scrollClass="max-h-[32rem]"
          className="border-0"
          onActivate={(i) => navigate(`/uyarilar/olay/${i.id}`)}
        />
      )}
    </div>
  )
}
