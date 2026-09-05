import { useCallback, useEffect, useState } from 'react'
import { auditTone } from '../lib/auditKinds'

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

const LIMITS = [50, 100, 250, 500]

export function AuditCard() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [verify, setVerify] = useState<VerifyResult | null>(null)
  const [limit, setLimit] = useState(100)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setError('')
    try {
      const [evRes, vRes] = await Promise.all([
        fetch(`/api/v1/audit?limit=${limit}`),
        fetch('/api/v1/audit/verify'),
      ])
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
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        {verify && (
          <span
            className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium ring-1 ${
              verify.ok
                ? 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/20'
                : 'bg-rose-500/10 text-rose-400 ring-rose-500/20'
            }`}
          >
            {verify.ok ? '✓ zincir sağlam' : `⚠ zincir bozuk — kayıt #${verify.broken_at}`}
            <span className="font-mono text-[10px] opacity-70">{verify.checked} kayıt</span>
          </span>
        )}
        <span className="flex items-center gap-1.5 text-xs text-slate-500">
          son
          <select
            value={limit}
            onChange={(e) => setLimit(+e.target.value)}
            className="rounded border border-slate-700/80 bg-slate-950 px-1.5 py-1 text-xs text-slate-300 focus:border-cyan-500/60"
          >
            {LIMITS.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
          kayıt
        </span>
        <button
          onClick={load}
          className="rounded-md border border-slate-700 px-2.5 py-1 text-xs text-slate-400 transition hover:border-slate-500 hover:text-slate-200"
        >
          Yenile
        </button>
        {error && <span className="text-xs text-rose-400">⚠ {error}</span>}
      </div>

      {!loaded && !error ? (
        <p className="py-6 text-center text-sm text-slate-500">Yükleniyor…</p>
      ) : events.length === 0 ? (
        <p className="py-6 text-center text-sm text-slate-500">Henüz denetim olayı yok.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-[11px] uppercase tracking-wider text-slate-500">
                <th className="py-2 pr-3 font-medium">Zaman</th>
                <th className="py-2 pr-3 font-medium">Kullanıcı</th>
                <th className="py-2 pr-3 font-medium">Eylem</th>
                <th className="py-2 pr-3 font-medium">Hedef</th>
                <th className="py-2 pr-3 font-medium">Detay</th>
                <th className="py-2 font-medium">IP</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id} className="border-b border-slate-800/60 last:border-0 align-top">
                  <td className="py-2 pr-3 whitespace-nowrap font-mono text-[11px] text-slate-500">
                    {new Date(e.ts * 1000).toLocaleString('tr-TR')}
                  </td>
                  <td className="py-2 pr-3 whitespace-nowrap">
                    <span className="font-mono text-slate-200">{e.username || '—'}</span>
                    {e.role && <span className="ml-1 text-[10px] text-slate-600">{e.role}</span>}
                  </td>
                  <td className="py-2 pr-3 whitespace-nowrap">
                    <span className={`rounded px-1.5 py-0.5 font-mono text-[10px] ring-1 ${auditTone(e.action)}`}>
                      {e.action}
                    </span>
                  </td>
                  <td className="py-2 pr-3 font-mono text-[11px] text-slate-400">{e.target || '—'}</td>
                  <td className="py-2 pr-3 text-[12px] text-slate-400">{e.detail || '—'}</td>
                  <td className="py-2 font-mono text-[11px] text-slate-500">{e.ip || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
