import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ask, btnCls, ismsDel, ismsPost, pill, type ContinuityTest, type MgmtReview, type Supplier } from '../lib/isms'

export function GovernancePage() {
  const [reviews, setReviews] = useState<MgmtReview[]>([])
  const [suppliers, setSuppliers] = useState<Supplier[]>([])
  const [continuity, setContinuity] = useState<ContinuityTest[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const [rv, sp, ct] = await Promise.all([
        fetch('/api/v1/isms/mgmt-reviews?limit=8').then((x) => x.json()),
        fetch('/api/v1/isms/suppliers').then((x) => x.json()),
        fetch('/api/v1/isms/continuity?limit=8').then((x) => x.json()),
      ])
      setReviews(rv)
      setSuppliers(sp)
      setContinuity(ct)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const addReview = async () => {
    const period = ask('Dönem (ör. 2026-Q3):')
    if (!period) return
    await ismsPost('/api/v1/isms/mgmt-reviews', {
      period,
      attendees: ask('Katılımcılar:'),
      inputs: ask('Girdiler (metrik/rapor özeti):'),
      decisions: ask('Kararlar:'),
      actions: ask('Aksiyonlar:'),
    })
    load()
  }

  const addSupplier = async () => {
    const name = ask('Tedarikçi adı:')
    if (!name) return
    await ismsPost('/api/v1/isms/suppliers', {
      name,
      service: ask('Hizmet:'),
      criticality: ask('Kritiklik (dusuk/orta/yuksek/kritik):', 'orta'),
      data_access: ask('Veri erişimi:'),
      contract_ref: ask('Sözleşme referansı:'),
      risk: ask('Risk notu:'),
    })
    load()
  }

  const removeSupplier = async (sp: Supplier) => {
    if (!confirm(`${sp.name} silinsin mi?`)) return
    await ismsDel(`/api/v1/isms/suppliers/${sp.id}`)
    load()
  }

  const addContinuity = async () => {
    const title = ask('Test başlığı:')
    if (!title) return
    await ismsPost('/api/v1/isms/continuity', {
      title,
      kind: ask('Tür (restore/failover/backup_check/tabletop):', 'restore'),
      result: ask('Sonuç (basarili/kismen/basarisiz):', 'basarili'),
      evidence: ask('Kanıt (runbook/backup log referansı):'),
      notes: ask('Notlar:'),
    })
    load()
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      {error && <p className="text-xs text-rose-400">{error}</p>}

      <Card title="Yönetim İncelemesi" right={<button onClick={addReview} className={btnCls}>+ inceleme</button>}>
        {reviews.length === 0 ? (
          <p className="text-xs text-slate-600">inceleme kaydı yok</p>
        ) : (
          <div className="space-y-1">
            {reviews.map((r) => (
              <div key={r.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                <span className="text-xs text-slate-300">{r.period}</span>
                <span className="text-[10px] text-slate-500">{r.attendees}</span>
                <span className="ml-auto font-mono text-[10px] text-slate-600">{new Date(r.ts * 1000).toLocaleDateString('tr-TR')}</span>
                {r.decisions && (
                  <p className="w-full truncate text-[11px] text-slate-500" title={r.decisions}>
                    karar: {r.decisions}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card title="Tedarikçiler" right={<span className="flex items-center gap-2 text-xs text-slate-500">A.5.19-22<button onClick={addSupplier} className={btnCls}>+ tedarikçi</button></span>}>
        {suppliers.length === 0 ? (
          <p className="text-xs text-slate-600">tedarikçi kaydı yok</p>
        ) : (
          <div className="space-y-1">
            {suppliers.map((sp) => (
              <div key={sp.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                <span className="text-xs text-slate-300">{sp.name}</span>
                <span className="text-[10px] text-slate-500">{sp.service}</span>
                {pill(sp.criticality, sp.criticality === 'kritik' || sp.criticality === 'yuksek' ? 'warn' : 'muted')}
                {sp.next_review > 0 && sp.next_review <= Date.now() / 1000 && pill('vadesi geçti', 'bad')}
                <button onClick={() => removeSupplier(sp)} className={`${btnCls} ml-auto`}>
                  sil
                </button>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card title="Süreklilik Testleri (BCDR)" right={<button onClick={addContinuity} className={btnCls}>+ test</button>}>
        {continuity.length === 0 ? (
          <p className="text-xs text-slate-600">test kaydı yok — DR runbook ve backup scriptleri kanıt olarak bağlanır</p>
        ) : (
          <div className="space-y-1">
            {continuity.map((t) => (
              <div key={t.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1">
                <span className="text-xs text-slate-300">{t.title}</span>
                {pill(t.kind, 'muted')}
                {pill(t.result, t.result === 'basarili' ? 'ok' : t.result === 'basarisiz' ? 'bad' : 'warn')}
                {t.evidence && <span className="text-[10px] text-slate-600">{t.evidence}</span>}
                <span className="ml-auto font-mono text-[10px] text-slate-600">{new Date(t.performed_at * 1000).toLocaleDateString('tr-TR')}</span>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
