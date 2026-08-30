import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceCard } from '../components/ComplianceCard'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ismsPost, pill, type Asset, type Summary } from '../lib/isms'

const btn = 'rounded border border-slate-700 px-2 py-0.5 text-[10px] text-slate-400 transition hover:border-slate-500 hover:text-slate-200'

export function ComplianceOverviewPage({ refreshKey }: { refreshKey: number }) {
  const [summary, setSummary] = useState<Summary | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const [s, a] = await Promise.all([
        fetch('/api/v1/isms/summary').then((x) => x.json()),
        fetch('/api/v1/isms/assets').then((x) => x.json()),
      ])
      setSummary(s)
      setAssets(a)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load, refreshKey])

  const syncAssets = async () => {
    await ismsPost('/api/v1/isms/assets/sync', {})
    load()
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      {/* ISMS olgunluk şeridi */}
      {error ? (
        <p className="text-xs text-rose-400">{error}</p>
      ) : !summary ? (
        <p className="text-xs text-slate-500">yükleniyor…</p>
      ) : (
        <Card title="ISMS Olgunluk Özeti" right={<span className="text-xs text-slate-500">Faz 10</span>}>
          <div className="flex flex-wrap items-center gap-2">
            {pill(`soa ${summary.soa.implemented}/${summary.soa.applicable}`, summary.soa.verified > 0 ? 'ok' : 'muted')}
            {pill(`yüksek risk ${summary.risks.high}`, summary.risks.high > 0 ? 'bad' : 'ok')}
            {pill(`açık bulgu ${summary.open_findings}`, summary.open_findings > 0 ? 'warn' : 'ok')}
            {pill(`yayın ${summary.policies_published}`, 'muted')}
            {pill(`varlık ${summary.assets}`, 'muted')}
            {summary.suppliers_due > 0 && pill(`tedarikçi vadesi ${summary.suppliers_due}`, 'warn')}
            <a
              href="/api/v1/isms/auditor-package?format=html"
              className="ml-auto rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20"
            >
              denetçi paketi ↓html
            </a>
            <a href="/api/v1/isms/auditor-package" className={btn}>
              ↓json
            </a>
            <button onClick={syncAssets} className={btn}>
              + filo senkronu
            </button>
          </div>

          <details className="mt-3 rounded border border-slate-800 bg-slate-900/40 px-2.5 py-1.5">
            <summary className="cursor-pointer text-[10px] uppercase tracking-wider text-slate-500">
              varlık envanteri ({assets.length})
            </summary>
            <div className="mt-1.5 flex flex-wrap gap-1">
              {assets.map((a) => (
                <span key={a.id} className="rounded border border-slate-800 bg-slate-900 px-1.5 py-0.5 font-mono text-[10px] text-slate-400">
                  {a.kind}:{a.name}
                  {a.criticality !== 'orta' && <b className="text-slate-300"> · {a.criticality}</b>}
                </span>
              ))}
              {assets.length === 0 && <span className="text-[11px] text-slate-600">boş — filo senkronu ile doldurun</span>}
            </div>
          </details>
        </Card>
      )}

      <Card
        title="5651 Log İmzalama"
        right={<span className="text-xs text-slate-500">imzalı loglar · delil paketi · inceleme</span>}
      >
        <ComplianceCard refreshKey={refreshKey} />
      </Card>
    </div>
  )
}
