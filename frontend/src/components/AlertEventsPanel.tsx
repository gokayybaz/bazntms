import { useCallback, useEffect, useMemo, useState } from 'react'
import { Panel } from './Panel'
import { TuiTable, type TuiColumn } from './TuiTable'
import { useDialog } from '../lib/dialog'
import { KIND_LABELS } from '../lib/alertKinds'
import { formatNum } from '../lib/format'

// Yaşam döngüsü alanlı uyarı olayı (S22.6+ · GET /api/v1/alerts/events). Yerel
// tip — CLAUDE.md konvansiyonu.
interface LifecycleEvent {
  id: number
  ts: number
  kind: string
  key: string
  message: string
  severity: 'info' | 'warn' | 'crit'
  state: 'firing' | 'ack' | 'resolved' | 'silenced'
  site: string
  count: number
  first_ts: number
  last_ts: number
  ack_by?: string
  note?: string
  group_id?: string
  ext_ref?: string
}

interface Silence {
  id: number
  match_kind: string
  match_site: string
  match_key: string
  starts_ts: number
  ends_ts: number
  reason: string
  created_by: string
}

const SEV_CLS: Record<string, string> = {
  crit: 'border-rose-500/50 text-rose-400',
  warn: 'border-amber-500/50 text-amber-400',
  info: 'border-sky-500/40 text-sky-300',
}
const STATE_CLS: Record<string, string> = {
  firing: 'text-rose-400',
  ack: 'text-amber-400',
  resolved: 'text-emerald-400',
  silenced: 'text-tui-dim',
}
const STATE_LABEL: Record<string, string> = { firing: 'açık', ack: 'kabul', resolved: 'çözüldü', silenced: 'susturuldu' }

function rel(ts: number): string {
  const s = Math.max(0, Math.floor(Date.now() / 1000 - ts))
  if (s < 60) return `${s}sn`
  if (s < 3600) return `${Math.floor(s / 60)}d`
  if (s < 86400) return `${Math.floor(s / 3600)}sa`
  return `${Math.floor(s / 86400)}g`
}

const HOURS = [
  { v: 6, l: '6sa' },
  { v: 24, l: '24sa' },
  { v: 24 * 7, l: '7g' },
  { v: 0, l: 'tümü' },
]

export function AlertEventsPanel() {
  const { form, confirm } = useDialog()
  const [severity, setSeverity] = useState('')
  const [state, setState] = useState('firing')
  const [kind, setKind] = useState('')
  const [hours, setHours] = useState(24)
  const [events, setEvents] = useState<LifecycleEvent[]>([])
  const [nextCursor, setNextCursor] = useState(0)
  const [silences, setSilences] = useState<Silence[]>([])
  const [err, setErr] = useState(false)

  const query = useMemo(() => {
    const p = new URLSearchParams()
    if (severity) p.set('severity', severity)
    if (state) p.set('state', state)
    if (kind) p.set('kind', kind)
    if (hours > 0) p.set('since', String(Math.floor(Date.now() / 1000 - hours * 3600)))
    p.set('limit', '100')
    return p.toString()
  }, [severity, state, kind, hours])

  const load = useCallback(async () => {
    try {
      const r = await fetch(`/api/v1/alerts/events?${query}`)
      if (!r.ok) throw new Error()
      const j = await r.json()
      setEvents(Array.isArray(j.events) ? j.events : [])
      setNextCursor(j.next_cursor ?? 0)
      setErr(false)
    } catch {
      setErr(true)
    }
  }, [query])

  const loadSilences = useCallback(async () => {
    try {
      const r = await fetch('/api/v1/alerts/silences')
      const j = await r.json()
      setSilences(Array.isArray(j.silences) ? j.silences : [])
    } catch {
      /* yoksay */
    }
  }, [])

  useEffect(() => {
    load()
    loadSilences()
    const id = window.setInterval(() => {
      load()
      loadSilences()
    }, 15_000)
    return () => window.clearInterval(id)
  }, [load, loadSilences])

  const loadMore = async () => {
    if (!nextCursor) return
    try {
      const r = await fetch(`/api/v1/alerts/events?${query}&cursor=${nextCursor}`)
      const j = await r.json()
      setEvents((prev) => [...prev, ...(j.events ?? [])])
      setNextCursor(j.next_cursor ?? 0)
    } catch {
      /* yoksay */
    }
  }

  const act = async (e: LifecycleEvent) => {
    const res = await form({
      title: `Uyarı #${e.id} — ${KIND_LABELS[e.kind] ?? e.kind}`,
      fields: [
        { key: 'action', label: 'İşlem', type: 'select', options: ['kabul et', 'çöz', 'not ekle', 'bakım penceresi'], defaultValue: 'kabul et' },
        { key: 'note', label: 'Not / gerekçe', type: 'textarea', defaultValue: e.note ?? '' },
        { key: 'minutes', label: 'Bakım süresi (dk)', type: 'number', defaultValue: '60' },
      ],
      confirmLabel: 'Uygula',
    })
    if (!res) return
    const note = res.note ?? ''
    try {
      if (res.action === 'kabul et') {
        await fetch(`/api/v1/alerts/events/${e.id}/ack`, mkBody({ note }))
      } else if (res.action === 'çöz') {
        await fetch(`/api/v1/alerts/events/${e.id}/resolve`, { method: 'POST' })
        if (note) await fetch(`/api/v1/alerts/events/${e.id}/note`, mkBody({ note }))
      } else if (res.action === 'not ekle') {
        await fetch(`/api/v1/alerts/events/${e.id}/note`, mkBody({ note }))
      } else if (res.action === 'bakım penceresi') {
        await fetch('/api/v1/alerts/silences', mkBody({
          match_kind: e.kind, match_site: e.site, match_key: e.key,
          reason: note || `#${e.id} için bakım`, duration_min: Number(res.minutes) || 60,
        }))
      }
      await load()
      await loadSilences()
    } catch {
      setErr(true)
    }
  }

  const dropSilence = async (s: Silence) => {
    if (!(await confirm(`Bakım penceresi silinsin mi? (${s.reason || s.match_kind || 'tüm'})`))) return
    await fetch(`/api/v1/alerts/silences/${s.id}`, { method: 'DELETE' })
    await loadSilences()
  }

  const columns: TuiColumn<LifecycleEvent>[] = [
    {
      key: 'severity',
      header: 'Önem',
      width: '5.5rem',
      render: (e) => <span className={`border px-1.5 py-0.5 text-[9px] uppercase ${SEV_CLS[e.severity] ?? 'border-rule text-tui-dim'}`}>{e.severity}</span>,
    },
    {
      key: 'state',
      header: 'Durum',
      width: '6rem',
      render: (e) => <span className={`text-[10px] uppercase ${STATE_CLS[e.state] ?? ''}`}>{STATE_LABEL[e.state] ?? e.state}</span>,
    },
    { key: 'kind', header: 'Tür', width: '9rem', render: (e) => KIND_LABELS[e.kind] ?? e.kind },
    { key: 'site', header: 'Kapsam', width: '8rem', render: (e) => e.site || <span className="text-tui-dim">—</span> },
    { key: 'message', header: 'Mesaj', render: (e) => <span className="text-ink">{e.message}</span> },
    {
      key: 'count',
      header: '×',
      align: 'right',
      width: '4rem',
      sortValue: (e) => e.count,
      render: (e) => (e.count > 1 ? formatNum(e.count) : ''),
    },
    { key: 'last_ts', header: 'Son', align: 'right', width: '4.5rem', sortValue: (e) => e.last_ts, render: (e) => rel(e.last_ts) },
    { key: 'group_id', header: 'Grup', width: '6rem', render: (e) => e.group_id || '' },
  ]

  const kinds = useMemo(() => [...new Set(events.map((e) => e.kind))].sort(), [events])

  return (
    <div className="space-y-3">
      <Panel
        title="Uyarı Olayları"
        right={
          <div className="flex items-center gap-1 overflow-x-auto text-[10px]">
            {['', 'crit', 'warn', 'info'].map((s) => (
              <FilterBtn key={s || 'a'} on={severity === s} onClick={() => setSeverity(s)}>{s || 'tüm önem'}</FilterBtn>
            ))}
            <span className="mx-1 text-rule-hi">│</span>
            {['', 'firing', 'ack', 'resolved', 'silenced'].map((s) => (
              <FilterBtn key={s || 'a'} on={state === s} onClick={() => setState(s)}>{s ? STATE_LABEL[s] : 'tüm durum'}</FilterBtn>
            ))}
            <span className="mx-1 text-rule-hi">│</span>
            {HOURS.map((h) => (
              <FilterBtn key={h.v} on={hours === h.v} onClick={() => setHours(h.v)}>{h.l}</FilterBtn>
            ))}
          </div>
        }
      >
        {err && <p className="mb-2 text-[11px] text-rose-400">⚠ olaylar alınamadı — otomatik yeniden denenecek</p>}
        {kinds.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-1">
            <FilterBtn on={kind === ''} onClick={() => setKind('')}>tüm tür</FilterBtn>
            {kinds.map((k) => (
              <FilterBtn key={k} on={kind === k} onClick={() => setKind(k)}>{KIND_LABELS[k] ?? k}</FilterBtn>
            ))}
          </div>
        )}
        <TuiTable
          columns={columns}
          rows={events}
          getKey={(e) => String(e.id)}
          onActivate={act}
          initialSort={{ key: 'last_ts', dir: 'desc' }}
          filterText={(e) => `${e.message} ${e.kind} ${e.site} ${e.key}`}
          filterLabel="mesaj / tür / kapsam"
          empty={<span className="text-tui-dim">Bu filtrede olay yok.</span>}
        />
        {nextCursor > 0 && (
          <button onClick={loadMore} className="mt-2 border border-rule px-3 py-1 text-[11px] uppercase tracking-[0.04em] text-tui-dim hover:text-ink-hi">
            daha fazla
          </button>
        )}
        <p className="mt-2 text-[10px] text-tui-dim">Enter / çift tık → kabul et · çöz · not · bakım penceresi</p>
      </Panel>

      {silences.length > 0 && (
        <Panel title="Bakım Pencereleri" right={<span className="text-[10px] text-tui-dim">{silences.length} aktif</span>}>
          <div className="space-y-1 text-[11px]">
            {silences.map((s) => (
              <div key={s.id} className="flex items-baseline gap-2 border-b border-rule py-1 last:border-0">
                <span className="text-tui-dim">{s.match_kind || 'her tür'}</span>
                <span className="text-tui-dim">{s.match_site || 'her saha'}</span>
                {s.match_key && <span className="text-tui-dim">key~{s.match_key}</span>}
                <span className="min-w-0 flex-1 truncate text-ink">{s.reason || '—'}</span>
                <span className="shrink-0 text-tui-dim">→ {new Date(s.ends_ts * 1000).toLocaleString('tr-TR')}</span>
                <button onClick={() => dropSilence(s)} className="shrink-0 text-rose-400 hover:underline">sil</button>
              </div>
            ))}
          </div>
        </Panel>
      )}
    </div>
  )
}

function FilterBtn({ on, onClick, children }: { on: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`shrink-0 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] transition ${
        on ? 'border-rx bg-rx/10 text-rx' : 'border-rule text-tui-dim hover:text-ink-hi'
      }`}
    >
      {children}
    </button>
  )
}

function mkBody(obj: Record<string, unknown>): RequestInit {
  return { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(obj) }
}
