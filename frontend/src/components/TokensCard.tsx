import { useCallback, useEffect, useRef, useState } from 'react'
import { errText } from '../lib/api'
import { Modal } from './Modal'
import { CopyButton } from './CopyButton'

interface APIToken {
  id: number
  name: string
  role: string
  site: string
  created_at: number
  last_used: number
  revoked: boolean
}

const ROLES = ['admin', 'site-admin', 'netops', 'analyst', 'viewer'] as const
const ROLE_LABEL: Record<string, string> = {
  admin: 'Yönetici (global)',
  'site-admin': 'Saha Yöneticisi',
  netops: 'Ağ Operatörü',
  analyst: 'Analist',
  viewer: 'İzleyici',
}
const REVOKE_CONFIRM_MS = 4000
const inputCls =
  'w-full border border-rule-hi bg-ground px-2 py-1 font-mono text-[11px] text-ink-hi outline-none placeholder:text-tui-dim focus:border-rx/60'

// lockedSite dolu ise (site-admin) token o sahaya sabitlenir, "admin" rolü gizli.
export function TokensCard({ lockedSite = '' }: { lockedSite?: string }) {
  const roles = lockedSite ? ROLES.filter((r) => r !== 'admin') : ROLES
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', role: 'viewer', site: lockedSite })
  const [creating, setCreating] = useState(false)
  const [reveal, setReveal] = useState<{ name: string; token: string } | null>(null)

  const [confirmRevokeId, setConfirmRevokeId] = useState<number | null>(null)
  const confirmTimer = useRef<number | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/tokens')
      if (res.status === 401 || res.status === 403) {
        setError('token listesi alınamadı (yetki)')
        return
      }
      setTokens(await res.json())
      setLoaded(true)
      setError('')
    } catch {
      setError('token listesi alınamadı')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])
  useEffect(
    () => () => {
      if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    },
    [],
  )

  const create = async () => {
    setError('')
    if (!form.name) {
      setError('token adı zorunlu')
      return
    }
    setCreating(true)
    try {
      const res = await fetch('/api/v1/tokens', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        setError(await errText(res))
        return
      }
      const data = await res.json()
      setReveal({ name: form.name, token: data.token })
      setForm({ name: '', role: 'viewer', site: lockedSite })
      setShowForm(false)
      await load()
    } finally {
      setCreating(false)
    }
  }

  const revoke = async (t: APIToken) => {
    setError('')
    const res = await fetch(`/api/v1/tokens/${t.id}`, { method: 'DELETE' })
    if (!res.ok) {
      setError(await errText(res))
      return
    }
    await load()
  }

  const handleRevokeClick = (t: APIToken) => {
    if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    if (confirmRevokeId === t.id) {
      setConfirmRevokeId(null)
      void revoke(t)
      return
    }
    setConfirmRevokeId(t.id)
    confirmTimer.current = window.setTimeout(() => setConfirmRevokeId(null), REVOKE_CONFIRM_MS)
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => setShowForm((v) => !v)}
          className="border border-rx/40 bg-rx/10 px-3 py-1 text-xs font-medium text-rx transition hover:bg-rx/20"
        >
          {showForm ? 'Vazgeç' : '+ Token Oluştur'}
        </button>
        {error && <span className="text-xs text-rose-400">⚠ {error}</span>}
      </div>

      {showForm && (
        <div className="grid grid-cols-1 gap-2 rounded-lg border border-rule bg-panel-2/40 p-3 sm:grid-cols-[1fr_auto_1fr_auto]">
          <input
            className={inputCls}
            placeholder="ad (ör. grafana, ci-pipeline)"
            autoComplete="off"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          />
          <select className={inputCls} value={form.role} onChange={(e) => setForm((f) => ({ ...f, role: e.target.value }))}>
            {roles.map((r) => (
              <option key={r} value={r}>
                {ROLE_LABEL[r]}
              </option>
            ))}
          </select>
          <input
            className={inputCls}
            placeholder="site (boş = tüm siteler)"
            title={lockedSite ? 'sahanıza sabit' : undefined}
            readOnly={!!lockedSite}
            value={form.site}
            onChange={(e) => setForm((f) => ({ ...f, site: e.target.value }))}
          />
          <button
            onClick={create}
            disabled={creating}
            className="border border-rx bg-rx px-3 py-1 font-mono text-[11px] font-bold uppercase tracking-[0.04em] text-ground transition enabled:hover:opacity-90 disabled:opacity-40"
          >
            {creating ? 'Oluşturuluyor…' : 'Oluştur'}
          </button>
        </div>
      )}

      {!loaded && !error ? (
        <p className="py-6 text-center text-sm text-tui-dim">Yükleniyor…</p>
      ) : tokens.length === 0 ? (
        <p className="py-6 text-center text-sm text-tui-dim">Henüz API token’ı yok.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[620px] text-sm">
            <thead>
              <tr className="bg-rx text-left text-[10px] uppercase tracking-[0.04em] text-ground">
                <th className="py-2 pr-3 font-medium">Ad</th>
                <th className="py-2 pr-3 font-medium">Rol</th>
                <th className="py-2 pr-3 font-medium">Site</th>
                <th className="py-2 pr-3 font-medium">Son Kullanım</th>
                <th className="py-2 pr-3 font-medium">Durum</th>
                <th className="py-2 font-medium">İşlem</th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.id} className="border-b border-rule last:border-0">
                  <td className="py-2 pr-3 font-mono text-ink-hi">{t.name}</td>
                  <td className="py-2 pr-3 text-tui-dim">{ROLE_LABEL[t.role] ?? t.role}</td>
                  <td className="py-2 pr-3 text-tui-dim">{t.site || <span className="text-tui-dim">tümü</span>}</td>
                  <td className="py-2 pr-3 font-mono text-[11px] text-tui-dim">
                    {t.last_used > 0 ? new Date(t.last_used * 1000).toLocaleString('tr-TR') : 'hiç'}
                  </td>
                  <td className="py-2 pr-3">
                    {t.revoked ? (
                      <span className="border border-rose-500/40 px-1 text-[10px] font-semibold uppercase text-rose-400">
                        iptal
                      </span>
                    ) : (
                      <span className="border border-emerald-500/40 px-1 text-[10px] font-semibold uppercase text-emerald-400">
                        etkin
                      </span>
                    )}
                  </td>
                  <td className="py-2">
                    {t.revoked ? (
                      <span className="text-[11px] text-tui-dim">—</span>
                    ) : (
                      <button
                        onClick={() => handleRevokeClick(t)}
                        className={`text-[11px] transition ${
                          confirmRevokeId === t.id ? 'font-semibold text-rose-400' : 'text-tui-dim hover:text-rose-400'
                        }`}
                      >
                        {confirmRevokeId === t.id ? 'emin misiniz?' : 'iptal et'}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {reveal && (
        <Modal title="Yeni API Token’ı" onClose={() => setReveal(null)}>
          <p className="mb-3 text-sm text-ink">
            <span className="font-mono text-rx">{reveal.name}</span> için token oluşturuldu. Bu değer{' '}
            <strong className="text-amber-400">yalnızca bir kez</strong> gösterilir — şimdi kopyalayın.
          </p>
          <div className="flex items-center gap-2 border border-rule-hi bg-ground p-2.5">
            <code className="min-w-0 flex-1 break-all font-mono text-xs text-ink-hi">{reveal.token}</code>
            <CopyButton text={reveal.token} />
          </div>
          <p className="mt-3 text-[11px] text-tui-dim">
            Kullanım: <code className="text-tui-dim">Authorization: Bearer {reveal.token.slice(0, 12)}…</code>
          </p>
          <div className="mt-4 flex justify-end">
            <button
              onClick={() => setReveal(null)}
              className="border border-rule-hi px-3 py-1 font-mono text-[11px] uppercase text-tui-dim transition hover:border-ink-hi hover:text-ink-hi"
            >
              Kapat
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
