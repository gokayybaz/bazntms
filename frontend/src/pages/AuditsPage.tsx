import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ask, btnCls, ismsPost, ismsPut, pill, statusTone, type Audit, type Finding } from '../lib/isms'

export function AuditsPage() {
  const [audits, setAudits] = useState<Audit[]>([])
  const [selAudit, setSelAudit] = useState<number | null>(null)
  const [findings, setFindings] = useState<Finding[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/isms/audits')
      if (!res.ok) throw new Error('denetimler alınamadı')
      setAudits(await res.json())
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const loadFindings = useCallback(async (auditId: number) => {
    setSelAudit((cur) => (cur === auditId ? null : auditId))
    const res = await fetch(`/api/v1/isms/audits/${auditId}/findings`)
    if (res.ok) setFindings(await res.json())
  }, [])

  const addAudit = async () => {
    const title = ask('Denetim başlığı:')
    if (!title) return
    await ismsPost('/api/v1/isms/audits', { title, scope: ask('Kapsam:'), planned_date: ask('Planlanan tarih (YYYY-MM-DD):') })
    load()
  }

  const addFinding = async (auditId: number) => {
    const description = ask('Bulgu açıklaması:')
    if (!description) return
    await ismsPost(`/api/v1/isms/audits/${auditId}/findings`, {
      description,
      severity: ask('Şiddet (dusuk/orta/yuksek):', 'orta'),
      control_id: ask('İlgili kontrol (ör. A.8.15):'),
      capa: ask('CAPA aksiyonu:'),
      capa_owner: ask('CAPA sorumlusu:'),
      capa_due: ask('Vade (YYYY-MM-DD):'),
    })
    if (selAudit === auditId) loadFindings(auditId)
  }

  const updateAudit = async (a: Audit) => {
    await ismsPut(`/api/v1/isms/audits/${a.id}`, { ...a, status: ask('Durum (planned/done/closed):', a.status) || a.status })
    load()
  }

  const updateFinding = async (f: Finding) => {
    await ismsPut(`/api/v1/isms/findings/${f.id}`, {
      ...f,
      status: ask('Durum (open/in_progress/verified/closed):', f.status) || f.status,
      capa: ask('CAPA:', f.capa),
    })
    if (selAudit) loadFindings(selAudit)
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Card title="İç Denetim Programı + CAPA" right={<span className="text-xs text-slate-500">{audits.length} denetim</span>}>
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <button onClick={addAudit} className={btnCls}>
              + denetim
            </button>
          </div>
          {error ? (
            <p className="text-xs text-rose-400">{error}</p>
          ) : audits.length === 0 ? (
            <p className="text-xs text-slate-600">denetim kaydı yok</p>
          ) : (
            audits.map((a) => (
              <div key={a.id} className="rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs text-slate-300">{a.title}</span>
                  {a.scope && <span className="text-[10px] text-slate-500">{a.scope}</span>}
                  {a.planned_date && <span className="font-mono text-[10px] text-slate-600">{a.planned_date}</span>}
                  {pill(a.status, statusTone(a.status))}
                  {a.auditor && <span className="font-mono text-[10px] text-slate-500">{a.auditor}</span>}
                  <span className="ml-auto flex gap-1">
                    <button onClick={() => loadFindings(a.id)} className={btnCls}>
                      bulgular
                    </button>
                    <button onClick={() => addFinding(a.id)} className={btnCls}>
                      + bulgu
                    </button>
                    <button onClick={() => updateAudit(a)} className={btnCls}>
                      güncelle
                    </button>
                  </span>
                </div>
                {selAudit === a.id && (
                  <div className="mt-1.5 space-y-0.5 border-t border-slate-800 pt-1.5">
                    {findings.length === 0 ? (
                      <p className="text-[11px] text-slate-600">bulgu yok</p>
                    ) : (
                      findings.map((f) => (
                        <div key={f.id} className="flex flex-wrap items-center gap-2 px-1 py-0.5">
                          <span className="font-mono text-[10px] text-slate-500">{f.ref}</span>
                          <span className="text-[11px] text-slate-300">{f.description}</span>
                          {pill(f.severity, f.severity === 'yuksek' ? 'bad' : f.severity === 'orta' ? 'warn' : 'muted')}
                          {f.control_id && <span className="font-mono text-[10px] text-cyan-300">{f.control_id}</span>}
                          {pill(f.status, statusTone(f.status))}
                          {f.capa_owner && (
                            <span className="font-mono text-[10px] text-slate-500">
                              {f.capa_owner}
                              {f.capa_due && ` · ${f.capa_due}`}
                            </span>
                          )}
                          <span className="ml-auto flex gap-1">
                            <button onClick={() => updateFinding(f)} className={btnCls}>
                              CAPA
                            </button>
                          </span>
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  )
}
