import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { useDialog } from '../lib/dialog'
import { btnCls, ismsDel, ismsPost, ismsPut, pill, riskTone, statusTone, type Risk } from '../lib/isms'

export function RiskRegisterPage() {
  const { form, prompt, confirm } = useDialog()
  const [risks, setRisks] = useState<Risk[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/isms/risks')
      if (!res.ok) throw new Error('risk defteri alınamadı')
      setRisks(await res.json())
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const addRisk = async () => {
    const v = await form({
      title: 'Yeni Risk',
      fields: [
        { key: 'threat', label: 'Tehdit' },
        { key: 'vulnerability', label: 'Zaafiyet' },
        { key: 'impact', label: 'Etki (1-5)', type: 'number', defaultValue: '3' },
        { key: 'likelihood', label: 'Olasılık (1-5)', type: 'number', defaultValue: '3' },
        { key: 'treatment', label: 'Muamele', type: 'select', options: ['mitigate', 'accept', 'transfer', 'avoid'] },
        { key: 'plan', label: 'Muamele planı', type: 'textarea' },
        { key: 'res_impact', label: 'Kalıntı etki (0=bilinmiyor)', type: 'number', defaultValue: '0' },
        { key: 'res_likelihood', label: 'Kalıntı olasılık', type: 'number', defaultValue: '0' },
        { key: 'owner', label: 'Sahip' },
      ],
    })
    if (!v || !v.threat.trim()) return
    await ismsPost('/api/v1/isms/risks', {
      ...v,
      impact: Number(v.impact),
      likelihood: Number(v.likelihood),
      res_impact: Number(v.res_impact),
      res_likelihood: Number(v.res_likelihood),
    })
    load()
  }

  const updateRisk = async (rk: Risk) => {
    const status = await prompt('Durum (open/in_progress/closed):', { title: `${rk.threat} — durum`, defaultValue: rk.status })
    if (status === null) return
    await ismsPut(`/api/v1/isms/risks/${rk.id}`, { ...rk, status: status || rk.status })
    load()
  }

  const removeRisk = async (rk: Risk) => {
    if (!(await confirm(`"${rk.threat}" riski silinsin mi?`, { danger: true, confirmLabel: 'Sil' }))) return
    await ismsDel(`/api/v1/isms/risks/${rk.id}`)
    load()
  }

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Panel title="Risk Defteri" right={<span className="text-[10px] text-tui-dim">etki × olasılık</span>}>
        <div className="space-y-1">
          <div className="mb-1.5">
            <button onClick={addRisk} className={btnCls}>
              + risk
            </button>
          </div>
          {error ? (
            <p className="text-[11px] text-rose-400">{error}</p>
          ) : risks.length === 0 ? (
            <p className="text-[11px] text-tui-dim">risk kaydı yok</p>
          ) : (
            risks.map((rk) => (
              <div key={rk.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1.5 text-[11px]">
                {pill(String(rk.score), riskTone(rk.score))}
                <span className="text-ink">{rk.threat}</span>
                {rk.vulnerability && <span className="text-[10px] text-tui-dim">{rk.vulnerability}</span>}
                <span className="hidden truncate text-[10px] text-tui-dim sm:inline">{rk.treatment}</span>
                {rk.res_score > 0 && pill(`kalıntı ${rk.res_score}`, riskTone(rk.res_score))}
                {pill(rk.status, statusTone(rk.status))}
                {rk.owner && <span className="text-[10px] text-tui-dim">{rk.owner}</span>}
                <span className="ml-auto flex gap-1">
                  <button onClick={() => updateRisk(rk)} className={btnCls}>
                    güncelle
                  </button>
                  <button onClick={() => removeRisk(rk)} className={btnCls}>
                    sil
                  </button>
                </span>
                {rk.plan && (
                  <p className="w-full truncate text-[11px] text-tui-dim" title={rk.plan}>
                    plan: {rk.plan}
                  </p>
                )}
              </div>
            ))
          )}
        </div>
      </Panel>
    </div>
  )
}
