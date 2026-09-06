import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { useDialog } from '../lib/dialog'
import { btnCls, ismsPost, ismsPut, pill, statusTone, type Audit, type Finding } from '../lib/isms'

export function AuditsPage() {
  const { form, prompt } = useDialog()
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
    const v = await form({
      title: 'Yeni Denetim',
      fields: [
        { key: 'title', label: 'Denetim başlığı' },
        { key: 'scope', label: 'Kapsam' },
        { key: 'planned_date', label: 'Planlanan tarih (YYYY-MM-DD)' },
      ],
    })
    if (!v || !v.title.trim()) return
    await ismsPost('/api/v1/isms/audits', v)
    load()
  }

  const addFinding = async (auditId: number) => {
    const v = await form({
      title: 'Yeni Bulgu',
      fields: [
        { key: 'description', label: 'Bulgu açıklaması', type: 'textarea' },
        { key: 'severity', label: 'Şiddet', type: 'select', options: ['dusuk', 'orta', 'yuksek'], defaultValue: 'orta' },
        { key: 'control_id', label: 'İlgili kontrol (ör. A.8.15)' },
        { key: 'capa', label: 'CAPA aksiyonu', type: 'textarea' },
        { key: 'capa_owner', label: 'CAPA sorumlusu' },
        { key: 'capa_due', label: 'Vade (YYYY-MM-DD)' },
      ],
    })
    if (!v || !v.description.trim()) return
    await ismsPost(`/api/v1/isms/audits/${auditId}/findings`, v)
    if (selAudit === auditId) loadFindings(auditId)
  }

  const updateAudit = async (a: Audit) => {
    const status = await prompt('Durum (planned/done/closed):', { title: a.title, defaultValue: a.status })
    if (status === null) return
    await ismsPut(`/api/v1/isms/audits/${a.id}`, { ...a, status: status || a.status })
    load()
  }

  const updateFinding = async (f: Finding) => {
    const v = await form({
      title: `${f.ref} — CAPA`,
      fields: [
        { key: 'status', label: 'Durum', type: 'select', options: ['open', 'in_progress', 'verified', 'closed'], defaultValue: f.status },
        { key: 'capa', label: 'CAPA', type: 'textarea', defaultValue: f.capa },
      ],
    })
    if (!v) return
    await ismsPut(`/api/v1/isms/findings/${f.id}`, { ...f, status: v.status || f.status, capa: v.capa })
    if (selAudit) loadFindings(selAudit)
  }

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Panel title="İç Denetim Programı + CAPA" right={<span className="text-[10px] text-tui-dim">{audits.length} denetim</span>}>
        <div className="space-y-1">
          <div className="mb-1.5">
            <button onClick={addAudit} className={btnCls}>
              + denetim
            </button>
          </div>
          {error ? (
            <p className="text-[11px] text-rose-400">{error}</p>
          ) : audits.length === 0 ? (
            <p className="text-[11px] text-tui-dim">denetim kaydı yok</p>
          ) : (
            audits.map((a) => (
              <div key={a.id} className="border border-rule bg-panel-2/40 px-2.5 py-1.5 text-[11px]">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-ink">{a.title}</span>
                  {a.scope && <span className="text-[10px] text-tui-dim">{a.scope}</span>}
                  {a.planned_date && <span className="text-[10px] text-tui-dim">{a.planned_date}</span>}
                  {pill(a.status, statusTone(a.status))}
                  {a.auditor && <span className="text-[10px] text-tui-dim">{a.auditor}</span>}
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
                  <div className="mt-1.5 space-y-0.5 border-t border-rule pt-1.5">
                    {findings.length === 0 ? (
                      <p className="text-[11px] text-tui-dim">bulgu yok</p>
                    ) : (
                      findings.map((f) => (
                        <div key={f.id} className="flex flex-wrap items-center gap-2 px-1 py-0.5">
                          <span className="hidden truncate text-[10px] text-tui-dim sm:inline">{f.ref}</span>
                          <span className="text-[11px] text-ink">{f.description}</span>
                          {pill(f.severity, f.severity === 'yuksek' ? 'bad' : f.severity === 'orta' ? 'warn' : 'muted')}
                          {f.control_id && <span className="text-[10px] text-rx">{f.control_id}</span>}
                          {pill(f.status, statusTone(f.status))}
                          {f.capa_owner && (
                            <span className="text-[10px] text-tui-dim">
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
      </Panel>
    </div>
  )
}
