import { useCallback, useEffect, useRef, useState } from 'react'
import { errText } from '../lib/api'
import { OS_OPTIONS } from '../lib/agentInstall'
import { CopyButton } from './CopyButton'

interface EnrollToken {
  id: number
  name: string
  site: string
  created_at: number
  expires_at: number
  last_used: number
  revoked: boolean
}

const REVOKE_CONFIRM_MS = 4000
const inputCls =
  'w-full border border-rule-hi bg-ground px-2 py-1 font-mono text-[11px] text-ink-hi outline-none placeholder:text-tui-dim focus:border-rx/60'

// lockedSite dolu ise (site-admin) enroll token o sahaya sabit. multiSite ise
// (çoklu-saha modu) site zorunlu — statik/site'siz token agent kaydında reddedilir.
export function EnrollWizard({ lockedSite = '', multiSite = false }: { lockedSite?: string; multiSite?: boolean }) {
  const hubUrl = window.location.origin

  const [tokens, setTokens] = useState<EnrollToken[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')

  // sihirbaz
  const [form, setForm] = useState({ name: '', site: lockedSite, expires_in_days: 0 })
  const [generating, setGenerating] = useState(false)
  const [generated, setGenerated] = useState<{ token: string; site: string } | null>(null)
  const [osId, setOsId] = useState(OS_OPTIONS[0].id)

  const [confirmRevokeId, setConfirmRevokeId] = useState<number | null>(null)
  const confirmTimer = useRef<number | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/enroll-tokens')
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

  const generate = async () => {
    setError('')
    if (!form.name) {
      setError('token adı zorunlu')
      return
    }
    setGenerating(true)
    try {
      const res = await fetch('/api/v1/enroll-tokens', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        setError(await errText(res))
        return
      }
      const data = await res.json()
      setGenerated({ token: data.token, site: form.site })
      setForm({ name: '', site: lockedSite, expires_in_days: 0 })
      await load()
    } finally {
      setGenerating(false)
    }
  }

  const revoke = async (t: EnrollToken) => {
    setError('')
    const res = await fetch(`/api/v1/enroll-tokens/${t.id}`, { method: 'DELETE' })
    if (!res.ok) {
      setError(await errText(res))
      return
    }
    await load()
  }

  const handleRevokeClick = (t: EnrollToken) => {
    if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    if (confirmRevokeId === t.id) {
      setConfirmRevokeId(null)
      void revoke(t)
      return
    }
    setConfirmRevokeId(t.id)
    confirmTimer.current = window.setTimeout(() => setConfirmRevokeId(null), REVOKE_CONFIRM_MS)
  }

  const os = OS_OPTIONS.find((o) => o.id === osId) ?? OS_OPTIONS[0]
  const command = generated
    ? os.command({ hubUrl, token: generated.token, site: generated.site })
    : null

  return (
    <div className="space-y-5">
      <p className="border border-rule bg-panel-2/40 px-3 py-2 text-[11px] text-tui-dim">
        Hub’ın <code className="text-ink">-enroll-token</code> bayrağındaki statik sır yalnızca ilk kurulum
        içindir — sızarsa hub yeniden başlatılmadan iptal edilemez. Buradan ürettiğiniz token’lar isimli, süreli ve
        tek tıkla iptal edilebilir.
      </p>

      {/* --- adım 1: token üret --- */}
      <div className="space-y-2">
        <p className="text-[10px] font-medium uppercase tracking-[0.06em] text-rx">1 · Enrollment token’ı üret</p>
        <div className="grid grid-cols-1 gap-2 border border-rule bg-panel-2/40 p-3 sm:grid-cols-[1fr_1fr_auto_auto]">
          <input
            className={inputCls}
            placeholder="ad (ör. ofis-linux, k8s-daemonset)"
            autoComplete="off"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          />
          <input
            className={inputCls}
            placeholder={
              lockedSite
                ? 'site'
                : multiSite
                  ? 'site (zorunlu)'
                  : 'site (boş = agent kendi beyan eder)'
            }
            title={lockedSite ? 'sahanıza sabit' : undefined}
            readOnly={!!lockedSite}
            value={form.site}
            onChange={(e) => setForm((f) => ({ ...f, site: e.target.value }))}
          />
          <label className="flex items-center gap-1.5 text-xs text-tui-dim">
            geçerlilik
            <input
              type="number"
              min={0}
              value={form.expires_in_days}
              onChange={(e) => setForm((f) => ({ ...f, expires_in_days: Math.max(0, +e.target.value || 0) }))}
              className={inputCls + ' !w-16'}
            />
            gün (0=süresiz)
          </label>
          <button
            onClick={generate}
            disabled={generating}
            className="border border-rx bg-rx px-3 py-1 font-mono text-[11px] font-bold uppercase tracking-[0.04em] text-ground transition enabled:hover:opacity-90 disabled:opacity-40"
          >
            {generating ? 'Üretiliyor…' : 'Token Üret'}
          </button>
        </div>
        {error && <p className="text-xs text-rose-400">⚠ {error}</p>}
      </div>

      {/* --- adım 2: kurulum komutu --- */}
      {generated ? (
        <div className="space-y-2">
          <p className="text-[10px] font-medium uppercase tracking-[0.06em] text-rx">2 · Hedef makinede çalıştır</p>
          <div className="flex flex-wrap gap-1.5">
            {OS_OPTIONS.map((o) => (
              <button
                key={o.id}
                onClick={() => setOsId(o.id)}
                className={`border-r border-rule px-3 py-1 font-mono text-[11px] uppercase tracking-[0.04em] transition ${
                  o.id === osId ? 'bg-rx text-ground' : 'text-tui-dim hover:bg-panel-2 hover:text-ink-hi'
                }`}
              >
                {o.label}
              </button>
            ))}
          </div>
          <p className="text-[11px] text-tui-dim">{os.note}</p>
          <div className="border border-rule-hi bg-ground">
            <div className="flex items-center justify-between border-b border-rule px-3 py-1.5">
              <span className="font-mono text-[10px] uppercase tracking-wider text-tui-dim">{os.label}</span>
              <CopyButton text={command ?? ''} />
            </div>
            <pre className="overflow-x-auto p-3 font-mono text-[11px] leading-relaxed text-ink-hi">{command}</pre>
          </div>
          <p className="text-[11px] text-tui-dim">
            Hub adresi <code className="text-tui-dim">{hubUrl}</code> · token yalnızca bu ekranda görünür ·
            enroll olan agent <a href="/agentlar" className="text-rx hover:underline">Agent’lar</a> sayfasında listelenir.
          </p>
        </div>
      ) : (
        <p className="border border-dashed border-rule-hi bg-panel-2/40 px-4 py-6 text-center text-sm text-tui-dim">
          Önce bir enrollment token’ı üretin — kurulum komutu burada görünecek.
        </p>
      )}

      {/* --- token listesi (yönetim) --- */}
      <div className="space-y-2">
        <p className="text-[10px] font-medium uppercase tracking-[0.06em] text-rx">Enrollment token’ları</p>
        {!loaded && !error ? (
          <p className="py-4 text-center text-sm text-tui-dim">Yükleniyor…</p>
        ) : tokens.length === 0 ? (
          <p className="py-4 text-center text-sm text-tui-dim">Henüz DB enrollment token’ı yok.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] text-sm">
              <thead>
                <tr className="bg-rx text-left text-[10px] uppercase tracking-[0.04em] text-ground">
                  <th className="py-2 pr-3 font-medium">Ad</th>
                  <th className="py-2 pr-3 font-medium">Site</th>
                  <th className="py-2 pr-3 font-medium">Geçerlilik</th>
                  <th className="py-2 pr-3 font-medium">Son Kullanım</th>
                  <th className="py-2 pr-3 font-medium">Durum</th>
                  <th className="py-2 font-medium">İşlem</th>
                </tr>
              </thead>
              <tbody>
                {tokens.map((t) => (
                  <tr key={t.id} className="border-b border-rule last:border-0">
                    <td className="py-2 pr-3 font-mono text-ink-hi">{t.name}</td>
                    <td className="py-2 pr-3 text-tui-dim">{t.site || <span className="text-tui-dim">—</span>}</td>
                    <td className="py-2 pr-3 text-tui-dim">
                      {t.expires_at > 0 ? new Date(t.expires_at * 1000).toLocaleDateString('tr-TR') : 'süresiz'}
                    </td>
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
      </div>
    </div>
  )
}
