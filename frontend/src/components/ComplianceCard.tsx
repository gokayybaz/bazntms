// ComplianceCard — 5651 uyum paneli (Faz 9.9): imza motoru durumu, delil
// paketi indirici ve inceleme tutanakları (ISO A.8.15 / A.8.2).

import { useCallback, useEffect, useState } from 'react'
import { useDialog } from '../lib/dialog'
import { btnCls } from '../lib/isms'

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
      className={`border px-1 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] ${
        ok ? 'border-emerald-500/40 text-emerald-400' : 'border-rule-hi text-tui-dim'
      }`}
    >
      {ok ? on : off}
    </span>
  )
}

export function ComplianceCard({ refreshKey }: { refreshKey: number }) {
  const { prompt } = useDialog()
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
    const notes = await prompt(kind === 'log' ? 'Log inceleme notları:' : 'Erişim incelemesi notları:', { title: 'İnceleme tutanağı' })
    if (notes === null) return
    // ikinci prompt iptali de tam vazgeçme sayılır (sessizce boş bulgu kaydı olmasın)
    const finding = await prompt('Bulgu (yoksa boş bırakın):', { title: 'Bulgu' })
    if (finding === null) return
    setReviewError('')
    try {
      const res = await fetch('/api/v1/compliance/reviews', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ kind, period: new Date().toISOString().slice(0, 7), notes, finding }),
      })
      if (!res.ok) throw new Error('tutanak kaydedilemedi')
    } catch (e) {
      setReviewError(e instanceof Error ? e.message : String(e))
      return
    }
    load()
  }

  if (error) return <p className="font-mono text-[11px] text-rose-400">{error}</p>
  if (!status) return <p className="font-mono text-[11px] text-tui-dim">yükleniyor…</p>

  const cfg = status.config

  return (
    <div className="space-y-3 font-mono text-[11px]">
      {/* motor durumu */}
      <div className="flex flex-wrap items-center gap-1.5">
        {badge(cfg.enabled, 'motor aktif', 'motor kapalı')}
        {badge(!!cfg.tsa_url, 'tsa yapılandırıldı', 'tsa yok')}
        {badge(cfg.sign_key, 'imza anahtarı', 'imza yok')}
        {badge(!!cfg.worm_dir, 'worm dizini', 'worm yok')}
        {badge(cfg.mask_pii, 'pii maskeleme', 'maskeleme kapalı')}
        <span className="ml-auto text-[10px] text-tui-dim">saklama: {cfg.retention_days} gün</span>
      </div>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
        <div className="border border-rule bg-panel-2/40 px-3 py-2">
          <span className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">imzalı kayıt</span>
          <div className="text-[13px] font-bold text-ink-hi">{status.records.toLocaleString('tr-TR')}</div>
        </div>
        <div className="border border-rule bg-panel-2/40 px-3 py-2">
          <span className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">son saatlik checkpoint</span>
          <div className="truncate text-ink" title={status.last_hourly?.root}>
            {status.last_hourly
              ? `${new Date(status.last_hourly.bucket_start * 1000).toLocaleString('tr-TR')} · ${status.last_hourly.root.slice(0, 12)}…`
              : '—'}
          </div>
        </div>
        <div className="border border-rule bg-panel-2/40 px-3 py-2">
          <span className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">son günlük mühür</span>
          <div className="truncate text-ink">
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
      <div className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 p-2.5">
        <span className="text-[11px] text-tui-dim">delil paketi (A.5.28):</span>
        <input
          type="date"
          value={from}
          onChange={(e) => setFrom(e.target.value)}
          aria-label="Başlangıç tarihi"
          className="border border-rule-hi bg-ground px-2 py-0.5 text-[11px] text-ink"
        />
        <input
          type="date"
          value={to}
          onChange={(e) => setTo(e.target.value)}
          aria-label="Bitiş tarihi"
          className="border border-rule-hi bg-ground px-2 py-0.5 text-[11px] text-ink"
        />
        <label className="flex items-center gap-1.5 text-[11px] text-tui-dim">
          <input type="checkbox" checked={mask} onChange={(e) => setMask(e.target.checked)} className="accent-cyan-500" />
          PII maskele
        </label>
        <a
          href={evidenceUrl()}
          aria-label={`Kanıt paketini indir (${from || 'başlangıç belirtilmedi'} – ${to || 'bitiş belirtilmedi'}, PII ${mask ? 'maskeli' : 'maskesiz'})`}
          className="border border-rx/40 bg-rx/10 px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20"
        >
          indir ↓
        </a>
        <span className="text-[10px] text-tui-dim">
          doğrulama: <code>bazntmsctl verify -bundle &lt;dosya&gt;</code>
        </span>
      </div>

      {/* inceleme tutanakları */}
      <div>
        <div className="mb-1.5 flex flex-wrap items-center gap-2">
          <span className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">inceleme tutanakları</span>
          <button onClick={() => addReview('log')} className={btnCls}>
            + log inceleme (A.8.15)
          </button>
          <button onClick={() => addReview('access')} className={btnCls}>
            + erişim incelemesi (A.8.2)
          </button>
        </div>
        <p className="mb-1.5 text-[10px] text-tui-dim">tutanaklar oluşturulduktan sonra değiştirilemez (WORM)</p>
        {reviewError && <p className="mb-1.5 text-[11px] text-rose-400">⚠ {reviewError}</p>}
        {reviews.length === 0 ? (
          <p className="text-[11px] text-tui-dim">tutanak yok — periyodik incelemeler burada imzalı olarak listelenir</p>
        ) : (
          <div className="space-y-1">
            {reviews.map((r) => (
              <div key={r.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1.5">
                <span
                  className={`px-1 font-mono text-[10px] uppercase ${r.kind === 'log' ? 'text-rx' : 'border border-rule-hi text-ink'}`}
                >
                  {r.kind}
                </span>
                <span className="text-ink">{r.period}</span>
                <span className="text-[10px] text-tui-dim">{r.username}</span>
                {r.finding && <span className="px-1 font-mono text-[10px] text-amber-400">bulgu</span>}
                <span className="ml-auto text-[10px] text-tui-dim">{new Date(r.ts * 1000).toLocaleString('tr-TR')}</span>
                {r.notes && (
                  <p className="w-full truncate text-[11px] text-tui-dim" title={r.notes}>
                    {r.notes}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
