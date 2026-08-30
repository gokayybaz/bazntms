import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ask, btnCls, ismsDel, ismsPost, ismsPut, pill, riskTone, statusTone, type Risk } from '../lib/isms'

export function RiskRegisterPage() {
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
    const threat = ask('Tehdit:')
    if (!threat) return
    await ismsPost('/api/v1/isms/risks', {
      threat,
      vulnerability: ask('Zaafiyet:'),
      impact: Number(ask('Etki (1-5):', '3')),
      likelihood: Number(ask('Olasılık (1-5):', '3')),
      treatment: ask('Muamele (mitigate/accept/transfer/avoid):', 'mitigate'),
      plan: ask('Muamele planı:'),
      res_impact: Number(ask('Kalıntı etki (0=bilinmiyor):', '0')),
      res_likelihood: Number(ask('Kalıntı olasılık:', '0')),
      owner: ask('Sahip:'),
    })
    load()
  }

  const updateRisk = async (rk: Risk) => {
    await ismsPut(`/api/v1/isms/risks/${rk.id}`, { ...rk, status: ask('Durum (open/in_progress/closed):', rk.status) || rk.status })
    load()
  }

  const removeRisk = async (rk: Risk) => {
    if (!confirm(`${rk.threat} silinsin mi?`)) return
    await ismsDel(`/api/v1/isms/risks/${rk.id}`)
    load()
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Card title="Risk Defteri" right={<span className="text-xs text-slate-500">etki × olasılık</span>}>
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <button onClick={addRisk} className={btnCls}>
              + risk
            </button>
          </div>
          {error ? (
            <p className="text-xs text-rose-400">{error}</p>
          ) : risks.length === 0 ? (
            <p className="text-xs text-slate-600">risk kaydı yok</p>
          ) : (
            risks.map((rk) => (
              <div key={rk.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                {pill(String(rk.score), riskTone(rk.score))}
                <span className="text-xs text-slate-300">{rk.threat}</span>
                {rk.vulnerability && <span className="text-[10px] text-slate-500">{rk.vulnerability}</span>}
                <span className="text-[10px] text-slate-600">{rk.treatment}</span>
                {rk.res_score > 0 && pill(`kalıntı ${rk.res_score}`, riskTone(rk.res_score))}
                {pill(rk.status, statusTone(rk.status))}
                {rk.owner && <span className="font-mono text-[10px] text-slate-500">{rk.owner}</span>}
                <span className="ml-auto flex gap-1">
                  <button onClick={() => updateRisk(rk)} className={btnCls}>
                    güncelle
                  </button>
                  <button onClick={() => removeRisk(rk)} className={btnCls}>
                    sil
                  </button>
                </span>
                {rk.plan && (
                  <p className="w-full truncate text-[11px] text-slate-500" title={rk.plan}>
                    plan: {rk.plan}
                  </p>
                )}
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  )
}
