// ComplianceCard — 5651 uyum paneli (Faz 9.9): imza motoru durumu, delil
// paketi indirici ve inceleme tutanakları (ISO A.8.15 / A.8.2).

import { useCallback, useEffect, useState } from 'react'

interface ComplianceConfig {
  enabled: boolean
  tsa_url: string
  sign_key: boolean
  worm_dir: string
  mask_pii: boolean
  retention_days: number
}
interface ComplianceStatus {
  config: ComplianceConfig
  records: number
  last_record_ts: number
  last_hourly?: { bucket_start: number; root: string; record_count: number }
  last_daily?: {
    day: string
    root: string
    tsa_status: string
    signed: boolean
    signed_at: number
    record_count: number
  }
}
interface Review {
  id: number
  ts: number
  username: string
  kind: string
  period: string
  notes: string
  finding: string
}

function badge(ok: boolean, on: string, off: string) {
  return (
    <span
      className={`rounded px-1.5 py-0.5 font-mono text-[9px] uppercase ${
        ok ? 'border border-emerald-500/40 bg-emerald-500/10 text-emerald-300' : 'border border-slate-700 text-dim-aa'
      }`}
    >
      {ok ? on : off}
    </span>
  )
}

export function ComplianceCard({ refreshKey }: { refreshKey: number }) {
  const [status, setStatus] = useState<ComplianceStatus | null>(null)
  const [reviews, setReviews] = useState<Review[]>([])
  const [error, setError] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [mask, setMask] = useState(true)
  const [reviewError, setReviewError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/compliance/status')
      if (!res.ok) throw new Error('durum alınamadı')
      setStatus(await res.json())
      const r = await fetch('/api/v1/compliance/reviews?limit=10')
      if (r.ok) setReviews(await r.json())
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load, refreshKey])

  const evidenceUrl = () => {
    const p = new URLSearchParams()
    if (from) p.set('from', from)
    if (to) p.set('to', to)
    p.set('mask', String(mask))
    return `/api/v1/compliance/evidence?${p.toString()}`
  }

  const addReview = async (kind: 'log' | 'access') => {
    const notes = prompt(
      kind === 'log' ? 'Log inceleme notları:' : 'Erişim incelemesi notları:',
      '',
    )
    if (notes === null) return
    // ikinci prompt'ta da iptal (null) tam vazgeçme sayılır — önceden `?? ''`
    // ile sessizce boş bulguyla devam ediyordu, kullanıcı "vazgeçtim"
    // sanırken gerçek bir tutanak kaydı oluşuyordu
    const finding = prompt('Bulgu (yoksa boş bırakın):', '')
    if (finding === null) return
    setReviewError('')
    try {
      const res = await fetch('/api/v1/compliance/reviews', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          kind,
          period: new Date().toISOString().slice(0, 7),
          notes,
          finding,
        }),
      })
      if (!res.ok) throw new Error('tutanak kaydedilemedi')
    } catch (e) {
      setReviewError(e instanceof Error ? e.message : String(e))
      return
    }
    load()
  }

  if (error) return <p className="text-xs text-rose-400">{error}</p>
  if (!status) return <p className="text-xs text-dim-aa">yükleniyor…</p>

  const cfg = status.config

  return (
    <div className="space-y-3">
      {/* motor durumu */}
      <div className="flex flex-wrap items-center gap-2">
        {badge(cfg.enabled, 'motor aktif', 'motor kapalı')}
        {badge(!!cfg.tsa_url, 'tsa yapılandırıldı', 'tsa yok')}
        {badge(cfg.sign_key, 'imza anahtarı', 'imza yok')}
        {badge(!!cfg.worm_dir, 'worm dizini', 'worm yok')}
        {badge(cfg.mask_pii, 'pii maskeleme', 'maskeleme kapalı')}
        <span className="ml-auto font-mono text-[10px] text-dim-aa">
          saklama: {cfg.retention_days} gün
        </span>
      </div>

      <div className="grid gap-2 sm:grid-cols-3">
        <div className="rounded border border-slate-800 bg-slate-900/60 px-3 py-2">
          <span className="text-[10px] uppercase tracking-wider text-dim-aa">imzalı kayıt</span>
          <div className="font-mono text-sm text-slate-200">{status.records.toLocaleString('tr-TR')}</div>
        </div>
        <div className="rounded border border-slate-800 bg-slate-900/60 px-3 py-2">
          <span className="text-[10px] uppercase tracking-wider text-dim-aa">son saatlik checkpoint</span>
          <div className="truncate font-mono text-xs text-slate-300" title={status.last_hourly?.root}>
            {status.last_hourly
              ? `${new Date(status.last_hourly.bucket_start * 1000).toLocaleString('tr-TR')} · ${status.last_hourly.root.slice(0, 12)}…`
              : '—'}
          </div>
        </div>
        <div className="rounded border border-slate-800 bg-slate-900/60 px-3 py-2">
          <span className="text-[10px] uppercase tracking-wider text-dim-aa">son günlük mühür</span>
          <div className="truncate font-mono text-xs text-slate-300">
            {status.last_daily ? (
              <>
                {status.last_daily.day} ·{' '}
                <span className={status.last_daily.tsa_status === 'ok' ? 'text-emerald-400' : 'text-amber-400'}>
                  tsa:{status.last_daily.tsa_status}
                </span>{' '}
                {status.last_daily.signed && '· imzalı'}
              </>
            ) : (
              '—'
            )}
          </div>
        </div>
      </div>

      {/* delil paketi */}
      <div className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 p-2.5">
        <span className="text-[11px] font-medium text-slate-400">delil paketi (A.5.28):</span>
        <input
          type="date"
          value={from}
          onChange={(e) => setFrom(e.target.value)}
          aria-label="Başlangıç tarihi"
          className="rounded border border-slate-700 bg-slate-900 px-2 py-1 font-mono text-xs text-slate-300"
        />
        <input
          type="date"
          value={to}
          onChange={(e) => setTo(e.target.value)}
          aria-label="Bitiş tarihi"
          className="rounded border border-slate-700 bg-slate-900 px-2 py-1 font-mono text-xs text-slate-300"
        />
        <label className="flex items-center gap-1.5 text-[11px] text-dim-aa">
          <input type="checkbox" checked={mask} onChange={(e) => setMask(e.target.checked)} className="accent-cyan-500" />
          PII maskele
        </label>
        <a
          href={evidenceUrl()}
          aria-label={`Kanıt paketini indir (${from || 'başlangıç belirtilmedi'} – ${to || 'bitiş belirtilmedi'}, PII ${mask ? 'maskeli' : 'maskesiz'})`}
          className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          indir ↓
        </a>
        <span className="text-[10px] text-dim-aa">
          doğrulama: <code className="font-mono">bazntmsctl verify -bundle &lt;dosya&gt;</code>
        </span>
      </div>

      {/* inceleme tutanakları */}
      <div>
        <div className="mb-1.5 flex items-center gap-2">
          <span className="text-[10px] uppercase tracking-wider text-dim-aa">inceleme tutanakları</span>
          <button
            onClick={() => addReview('log')}
            className="rounded border border-slate-700 px-2 py-0.5 text-[10px] text-slate-400 transition hover:border-slate-500 hover:text-slate-200"
          >
            + log inceleme (A.8.15)
          </button>
          <button
            onClick={() => addReview('access')}
            className="rounded border border-slate-700 px-2 py-0.5 text-[10px] text-slate-400 transition hover:border-slate-500 hover:text-slate-200"
          >
            + erişim incelemesi (A.8.2)
          </button>
        </div>
        <p className="mb-1.5 text-[10px] text-dim-aa">tutanaklar oluşturulduktan sonra değiştirilemez (WORM)</p>
        {reviewError && <p className="mb-1.5 text-xs text-rose-400">⚠ {reviewError}</p>}
        {reviews.length === 0 ? (
          <p className="text-xs text-dim-aa">tutanak yok — periyodik incelemeler burada imzalı olarak listelenir</p>
        ) : (
          <div className="space-y-1">
            {reviews.map((r) => (
              <div key={r.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                <span
                  className={`rounded px-1.5 py-0.5 font-mono text-[9px] uppercase ${
                    // 'access' önceden tek başına violet kullanıyordu — DESIGN.md'de
                    // violet her zaman cyan ile eşleşmesi gereken tx-trafik rengi,
                    // burada bir kategori etiketi için sözleşme dışı kullanılıyordu;
                    // nötr slate'e taşındı (log/access ayrımı zaten metinle sağlanıyor)
                    r.kind === 'log' ? 'bg-cyan-500/10 text-cyan-300' : 'border border-slate-600 bg-slate-800/60 text-slate-300'
                  }`}
                >
                  {r.kind}
                </span>
                <span className="text-xs text-slate-300">{r.period}</span>
                <span className="font-mono text-[10px] text-dim-aa">{r.username}</span>
                {r.finding && (
                  <span className="rounded bg-amber-500/10 px-1.5 py-0.5 font-mono text-[9px] text-amber-300">bulgu</span>
                )}
                <span className="ml-auto text-[10px] text-dim-aa">
                  {new Date(r.ts * 1000).toLocaleString('tr-TR')}
                </span>
                {r.notes && <p className="w-full truncate text-[11px] text-dim-aa" title={r.notes}>{r.notes}</p>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
