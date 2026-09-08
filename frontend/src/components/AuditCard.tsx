import { useCallback, useEffect, useMemo, useState } from 'react'
import { auditTone } from '../lib/auditKinds'
import { RangeTabs } from './RangeTabs'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface AuditEvent {
  id: number
  ts: number
  username: string
  role: string
  action: string
  target: string
  detail: string
  ip: string
  hash: string
  // v2 (Faz 25-C)
  actor_type?: string
  request_id?: string
  user_agent?: string
  result?: string
  before_json?: string
  after_json?: string
}

interface VerifyResult {
  ok: boolean
  broken_at: number
  checked: number
}

const LIMITS = [
  { label: '50', value: 50 },
  { label: '100', value: 100 },
  { label: '250', value: 250 },
  { label: '500', value: 500 },
] as const

interface Filters {
  actor: string
  action: string
  resource: string
  ip: string
  result: string
  from: string
  to: string
}

const EMPTY: Filters = { actor: '', action: '', resource: '', ip: '', result: '', from: '', to: '' }

function resultTone(r?: string): string {
  if (r === 'denied') return 'border-rose-500/40 text-rose-400'
  if (r === 'error') return 'border-amber-500/40 text-amber-400'
  if (r === 'ok') return 'border-emerald-500/40 text-emerald-400'
  return 'border-rule-hi text-tui-dim'
}

// pretty, JSON metnini okunur biçime getirir (başarısızsa ham döner).
function pretty(s?: string): string {
  if (!s) return ''
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

const cols: TuiColumn<AuditEvent>[] = [
  {
    key: 'ts',
    header: 'Zaman',
    width: '11rem',
    sortable: true,
    sortValue: (e) => e.ts,
    render: (e) => <span className="text-tui-dim">{new Date(e.ts * 1000).toLocaleString('tr-TR')}</span>,
  },
  {
    key: 'username',
    header: 'Kullanıcı',
    sortable: true,
    render: (e) => (
      <span>
        <span className="text-ink-hi">{e.username || '—'}</span>
        {e.actor_type && e.actor_type !== '-' && (
          <span className="ml-1 text-[10px] text-tui-dim">{e.actor_type}</span>
        )}
      </span>
    ),
  },
  {
    key: 'action',
    header: 'Eylem',
    width: '9rem',
    sortable: true,
    render: (e) => <span className={`border px-1 font-mono text-[10px] ${auditTone(e.action)}`}>{e.action}</span>,
  },
  {
    key: 'result',
    header: 'Sonuç',
    width: '5rem',
    sortable: true,
    render: (e) =>
      e.result ? (
        <span className={`border px-1 font-mono text-[10px] ${resultTone(e.result)}`}>{e.result}</span>
      ) : (
        <span className="text-tui-dim">—</span>
      ),
  },
  { key: 'target', header: 'Hedef', sortable: true, render: (e) => <span className="text-tui-dim">{e.target || '—'}</span> },
  {
    key: 'detail',
    header: 'Detay',
    render: (e) => (
      <span className="text-tui-dim">
        {e.detail || '—'}
        {(e.before_json || e.after_json) && <span className="ml-1 text-[10px] text-rx">±diff</span>}
      </span>
    ),
  },
  { key: 'ip', header: 'IP', width: '9rem', render: (e) => <span className="text-tui-dim">{e.ip || '—'}</span> },
]

function fieldInput(
  label: string,
  value: string,
  onChange: (v: string) => void,
  opts: { type?: string; placeholder?: string } = {},
) {
  return (
    <input
      type={opts.type ?? 'text'}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={opts.placeholder}
      aria-label={label}
      className="w-28 border border-rule-hi bg-ground px-2 py-0.5 text-[11px] text-ink placeholder:text-tui-dim"
    />
  )
}

export function AuditCard() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [verify, setVerify] = useState<VerifyResult | null>(null)
  const [limit, setLimit] = useState<50 | 100 | 250 | 500>(100)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [filters, setFilters] = useState<Filters>(EMPTY)
  const [selected, setSelected] = useState<AuditEvent | null>(null)

  const query = useMemo(() => {
    const p = new URLSearchParams({ limit: String(limit) })
    if (filters.actor) p.set('actor', filters.actor)
    if (filters.action) p.set('action', filters.action)
    if (filters.resource) p.set('resource', filters.resource)
    if (filters.ip) p.set('ip', filters.ip)
    if (filters.result) p.set('result', filters.result)
    if (filters.from) p.set('since', String(Math.floor(new Date(filters.from).getTime() / 1000)))
    if (filters.to) p.set('until', String(Math.floor(new Date(filters.to).getTime() / 1000) + 86_399))
    return p.toString()
  }, [limit, filters])

  const load = useCallback(async () => {
    setError('')
    try {
      const [evRes, vRes] = await Promise.all([fetch(`/api/v1/audit?${query}`), fetch('/api/v1/audit/verify')])
      if (evRes.status === 401 || evRes.status === 403) {
        setError('denetim kaydı alınamadı (yetki)')
        return
      }
      setEvents(await evRes.json())
      setVerify(vRes.ok ? await vRes.json() : null)
      setLoaded(true)
    } catch {
      setError('denetim kaydı alınamadı')
    }
  }, [query])

  useEffect(() => {
    const t = window.setTimeout(load, 250)
    return () => window.clearTimeout(t)
  }, [load])

  const set = (k: keyof Filters) => (v: string) => setFilters((f) => ({ ...f, [k]: v }))
  const active = Object.values(filters).some(Boolean)

  return (
    <div className="space-y-2 font-mono">
      <div className="flex flex-wrap items-center gap-3 text-[11px]">
        {verify && (
          <span
            className={`inline-flex items-center gap-1.5 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] ${
              verify.ok ? 'border-emerald-500/40 text-emerald-400' : 'border-rose-500/40 text-rose-400'
            }`}
          >
            {verify.ok ? '✓ zincir sağlam' : `⚠ zincir bozuk — kayıt #${verify.broken_at}`}
            <span className="opacity-70">{verify.checked} kayıt</span>
          </span>
        )}
        <span className="flex items-center gap-1.5 text-tui-dim">
          son
          <RangeTabs ranges={LIMITS} value={limit} onChange={setLimit} />
          kayıt
        </span>
        <button
          onClick={load}
          className="border border-rule-hi px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-tui-dim transition hover:border-ink-hi hover:text-ink-hi"
        >
          Yenile
        </button>
        {error && <span className="text-rose-400">⚠ {error}</span>}
      </div>

      {/* sunucu-taraflı süzgeç barı (Faz 25-C) */}
      <div className="flex flex-wrap items-center gap-1.5 border border-rule bg-panel-2/40 p-2 text-[11px]">
        <span className="text-tui-dim">süzgeç:</span>
        {fieldInput('Kullanıcı', filters.actor, set('actor'), { placeholder: 'kullanıcı' })}
        {fieldInput('Eylem', filters.action, set('action'), { placeholder: 'eylem (user.*)' })}
        {fieldInput('Hedef', filters.resource, set('resource'), { placeholder: 'hedef' })}
        {fieldInput('IP', filters.ip, set('ip'), { placeholder: 'ip' })}
        <select
          value={filters.result}
          onChange={(e) => set('result')(e.target.value)}
          aria-label="Sonuç"
          className="border border-rule-hi bg-ground px-1 py-0.5 text-[11px] text-ink"
        >
          <option value="">sonuç: tümü</option>
          <option value="ok">ok</option>
          <option value="error">error</option>
          <option value="denied">denied</option>
        </select>
        {fieldInput('Başlangıç', filters.from, set('from'), { type: 'date' })}
        {fieldInput('Bitiş', filters.to, set('to'), { type: 'date' })}
        {active && (
          <button
            onClick={() => setFilters(EMPTY)}
            className="border border-rule-hi px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-tui-dim transition hover:border-ink-hi hover:text-ink-hi"
          >
            Temizle
          </button>
        )}
      </div>

      {!loaded && !error ? (
        <p className="py-6 text-center text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : events.length === 0 ? (
        <p className="py-6 text-center text-[11px] text-tui-dim">
          {active ? 'Süzgece uyan denetim olayı yok.' : 'Henüz denetim olayı yok.'}
        </p>
      ) : (
        <TuiTable
          columns={cols}
          rows={events}
          getKey={(e) => String(e.id)}
          onActivate={(e) => setSelected((cur) => (cur?.id === e.id ? null : e))}
          filterText={(e) => `${e.username} ${e.action} ${e.target} ${e.detail} ${e.ip}`}
          filterLabel="Denetim filtrele (yerel)…"
          initialSort={{ key: 'ts', dir: 'desc' }}
          scrollClass="max-h-[32rem]"
          className="border-0"
        />
      )}

      {selected && (
        <div className="border border-rule-hi bg-panel-2/40 p-3 text-[11px]">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-ink-hi">
              #{selected.id} · {selected.action}
            </span>
            <button onClick={() => setSelected(null)} className="text-tui-dim hover:text-ink-hi">
              ✕ kapat
            </button>
          </div>
          <dl className="grid grid-cols-[7rem_1fr] gap-x-3 gap-y-0.5 text-tui-dim">
            <dt>aktör</dt>
            <dd className="text-ink">
              {selected.username} ({selected.actor_type || '—'} · {selected.role || '—'})
            </dd>
            <dt>sonuç</dt>
            <dd className="text-ink">{selected.result || '—'}</dd>
            <dt>request-id</dt>
            <dd className="text-ink">{selected.request_id || '—'}</dd>
            <dt>user-agent</dt>
            <dd className="break-all text-ink">{selected.user_agent || '—'}</dd>
            <dt>IP</dt>
            <dd className="text-ink">{selected.ip || '—'}</dd>
          </dl>
          {(selected.before_json || selected.after_json) && (
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              <div>
                <div className="mb-1 text-[10px] uppercase tracking-[0.04em] text-tui-dim">öncesi</div>
                <pre className="overflow-x-auto border border-rule bg-ground p-2 text-[10px] text-rose-300">
                  {pretty(selected.before_json) || '—'}
                </pre>
              </div>
              <div>
                <div className="mb-1 text-[10px] uppercase tracking-[0.04em] text-tui-dim">sonrası</div>
                <pre className="overflow-x-auto border border-rule bg-ground p-2 text-[10px] text-emerald-300">
                  {pretty(selected.after_json) || '—'}
                </pre>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
