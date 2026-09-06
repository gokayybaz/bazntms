import { useCallback, useEffect, useState } from 'react'
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
        {e.role && <span className="ml-1 text-[10px] text-tui-dim">{e.role}</span>}
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
  { key: 'target', header: 'Hedef', sortable: true, render: (e) => <span className="text-tui-dim">{e.target || '—'}</span> },
  { key: 'detail', header: 'Detay', render: (e) => <span className="text-tui-dim">{e.detail || '—'}</span> },
  { key: 'ip', header: 'IP', width: '9rem', render: (e) => <span className="text-tui-dim">{e.ip || '—'}</span> },
]

export function AuditCard() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [verify, setVerify] = useState<VerifyResult | null>(null)
  const [limit, setLimit] = useState<50 | 100 | 250 | 500>(100)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setError('')
    try {
      const [evRes, vRes] = await Promise.all([fetch(`/api/v1/audit?limit=${limit}`), fetch('/api/v1/audit/verify')])
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
  }, [limit])

  useEffect(() => {
    load()
  }, [load])

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

      {!loaded && !error ? (
        <p className="py-6 text-center text-[11px] text-tui-dim">Yükleniyor…</p>
      ) : events.length === 0 ? (
        <p className="py-6 text-center text-[11px] text-tui-dim">Henüz denetim olayı yok.</p>
      ) : (
        <TuiTable
          columns={cols}
          rows={events}
          getKey={(e) => String(e.id)}
          filterText={(e) => `${e.username} ${e.action} ${e.target} ${e.detail} ${e.ip}`}
          filterLabel="Denetim filtrele (kullanıcı, eylem, hedef, ip)…"
          initialSort={{ key: 'ts', dir: 'desc' }}
          scrollClass="max-h-[32rem]"
          className="border-0"
        />
      )}
    </div>
  )
}
