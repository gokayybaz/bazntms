import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceCard } from '../components/ComplianceCard'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { btnCls, ismsPost, pill, type Asset, type Summary } from '../lib/isms'

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
    <div className="mx-auto max-w-[1600px] space-y-3 px-3 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="text-[10px] text-tui-dim">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      {error ? (
        <p className="text-[11px] text-rose-400">{error}</p>
      ) : !summary ? (
        <p className="text-[11px] text-tui-dim">yükleniyor…</p>
      ) : (
        <Panel title="ISMS Olgunluk Özeti">
          <div className="flex flex-wrap items-center gap-1.5">
            {pill(`soa ${summary.soa.implemented}/${summary.soa.applicable}`, summary.soa.verified > 0 ? 'ok' : 'muted')}
            {pill(`yüksek risk ${summary.risks.high}`, summary.risks.high > 0 ? 'bad' : 'ok')}
            {pill(`açık bulgu ${summary.open_findings}`, summary.open_findings > 0 ? 'warn' : 'ok')}
            {pill(`yayın ${summary.policies_published}`, 'muted')}
            {pill(`varlık ${summary.assets}`, 'muted')}
            {summary.suppliers_due > 0 && pill(`tedarikçi vadesi ${summary.suppliers_due}`, 'warn')}
            <a
              href="/api/v1/isms/auditor-package?format=html"
              className="ml-auto border border-rx/40 bg-rx/10 px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20"
            >
              denetçi paketi ↓html
            </a>
            <a href="/api/v1/isms/auditor-package" className={btnCls}>
              ↓json
            </a>
            <button onClick={syncAssets} className={btnCls}>
              + filo senkronu
            </button>
          </div>

          <details className="mt-3 border border-rule bg-panel-2/40 px-2.5 py-1.5">
            <summary className="cursor-pointer text-[10px] uppercase tracking-[0.04em] text-tui-dim">
              varlık envanteri ({assets.length})
            </summary>
            <div className="mt-1.5 flex flex-wrap gap-1">
              {assets.map((a) => (
                <span key={a.id} className="border border-rule px-1.5 py-0.5 font-mono text-[10px] text-tui-dim">
                  {a.kind}:{a.name}
                  {a.criticality !== 'orta' && <b className="text-ink"> · {a.criticality}</b>}
                </span>
              ))}
              {assets.length === 0 && <span className="text-[11px] text-tui-dim">boş — filo senkronu ile doldurun</span>}
            </div>
          </details>
        </Panel>
      )}

      <Panel title="5651 Log İmzalama" right={<span className="text-[10px] text-tui-dim">imzalı loglar · delil paketi · inceleme</span>}>
        <ComplianceCard refreshKey={refreshKey} />
      </Panel>
    </div>
  )
}
