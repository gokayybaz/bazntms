import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { useDialog } from '../lib/dialog'
import { btnCls, ismsDel, ismsPost, pill, type ContinuityTest, type MgmtReview, type Supplier } from '../lib/isms'

export function GovernancePage() {
  const { form, confirm } = useDialog()
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
    const v = await form({
      title: 'Yönetim İncelemesi',
      fields: [
        { key: 'period', label: 'Dönem (ör. 2026-Q3)' },
        { key: 'attendees', label: 'Katılımcılar' },
        { key: 'inputs', label: 'Girdiler (metrik/rapor özeti)', type: 'textarea' },
        { key: 'decisions', label: 'Kararlar', type: 'textarea' },
        { key: 'actions', label: 'Aksiyonlar', type: 'textarea' },
      ],
    })
    if (!v || !v.period.trim()) return
    await ismsPost('/api/v1/isms/mgmt-reviews', v)
    load()
  }

  const addSupplier = async () => {
    const v = await form({
      title: 'Yeni Tedarikçi',
      fields: [
        { key: 'name', label: 'Tedarikçi adı' },
        { key: 'service', label: 'Hizmet' },
        { key: 'criticality', label: 'Kritiklik', type: 'select', options: ['dusuk', 'orta', 'yuksek', 'kritik'], defaultValue: 'orta' },
        { key: 'data_access', label: 'Veri erişimi' },
        { key: 'contract_ref', label: 'Sözleşme referansı' },
        { key: 'risk', label: 'Risk notu', type: 'textarea' },
      ],
    })
    if (!v || !v.name.trim()) return
    await ismsPost('/api/v1/isms/suppliers', v)
    load()
  }

  const removeSupplier = async (sp: Supplier) => {
    if (!(await confirm(`"${sp.name}" tedarikçisi silinsin mi?`, { danger: true, confirmLabel: 'Sil' }))) return
    await ismsDel(`/api/v1/isms/suppliers/${sp.id}`)
    load()
  }

  const addContinuity = async () => {
    const v = await form({
      title: 'Süreklilik Testi',
      fields: [
        { key: 'title', label: 'Test başlığı' },
        { key: 'kind', label: 'Tür', type: 'select', options: ['restore', 'failover', 'backup_check', 'tabletop'], defaultValue: 'restore' },
        { key: 'result', label: 'Sonuç', type: 'select', options: ['basarili', 'kismen', 'basarisiz'], defaultValue: 'basarili' },
        { key: 'evidence', label: 'Kanıt (runbook/backup log referansı)' },
        { key: 'notes', label: 'Notlar', type: 'textarea' },
      ],
    })
    if (!v || !v.title.trim()) return
    await ismsPost('/api/v1/isms/continuity', v)
    load()
  }

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="text-[10px] text-tui-dim">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      {error && <p className="text-[11px] text-rose-400">{error}</p>}

      <Panel title="Yönetim İncelemesi" right={<button onClick={addReview} className={btnCls}>+ inceleme</button>}>
        {reviews.length === 0 ? (
          <p className="text-[11px] text-tui-dim">inceleme kaydı yok</p>
        ) : (
          <div className="space-y-1 text-[11px]">
            {reviews.map((r) => (
              <div key={r.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1">
                <span className="text-ink">{r.period}</span>
                <span className="text-[10px] text-tui-dim">{r.attendees}</span>
                <span className="ml-auto text-[10px] text-tui-dim">{new Date(r.ts * 1000).toLocaleDateString('tr-TR')}</span>
                {r.decisions && (
                  <p className="w-full truncate text-[11px] text-tui-dim" title={r.decisions}>
                    karar: {r.decisions}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </Panel>

      <Panel
        title="Tedarikçiler"
        right={
          <span className="flex items-center gap-2 text-[10px] text-tui-dim">
            A.5.19-22
            <button onClick={addSupplier} className={btnCls}>
              + tedarikçi
            </button>
          </span>
        }
      >
        {suppliers.length === 0 ? (
          <p className="text-[11px] text-tui-dim">tedarikçi kaydı yok</p>
        ) : (
          <div className="space-y-1 text-[11px]">
            {suppliers.map((sp) => (
              <div key={sp.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1">
                <span className="text-ink">{sp.name}</span>
                <span className="text-[10px] text-tui-dim">{sp.service}</span>
                {pill(sp.criticality, sp.criticality === 'kritik' || sp.criticality === 'yuksek' ? 'warn' : 'muted')}
                {sp.next_review > 0 && sp.next_review <= Date.now() / 1000 && pill('vadesi geçti', 'bad')}
                <button onClick={() => removeSupplier(sp)} className={`${btnCls} ml-auto`}>
                  sil
                </button>
              </div>
            ))}
          </div>
        )}
      </Panel>

      <Panel title="Süreklilik Testleri (BCDR)" right={<button onClick={addContinuity} className={btnCls}>+ test</button>}>
        {continuity.length === 0 ? (
          <p className="text-[11px] text-tui-dim">test kaydı yok — DR runbook ve backup scriptleri kanıt olarak bağlanır</p>
        ) : (
          <div className="space-y-1 text-[11px]">
            {continuity.map((t) => (
              <div key={t.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1">
                <span className="text-ink">{t.title}</span>
                {pill(t.kind, 'muted')}
                {pill(t.result, t.result === 'basarili' ? 'ok' : t.result === 'basarisiz' ? 'bad' : 'warn')}
                {t.evidence && <span className="text-[10px] text-tui-dim">{t.evidence}</span>}
                <span className="ml-auto text-[10px] text-tui-dim">{new Date(t.performed_at * 1000).toLocaleDateString('tr-TR')}</span>
              </div>
            ))}
          </div>
        )}
      </Panel>
    </div>
  )
}
