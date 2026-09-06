import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { useDialog } from '../lib/dialog'
import { btnCls, ismsPost, pill, statusTone, type Policy } from '../lib/isms'

export function PoliciesPage() {
  const { form } = useDialog()
  const [policies, setPolicies] = useState<Policy[]>([])
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/isms/policies')
      if (!res.ok) throw new Error('politikalar alınamadı')
      setPolicies(await res.json())
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const addPolicy = async () => {
    const v = await form({
      title: 'Yeni Politika',
      fields: [
        { key: 'title', label: 'Politika başlığı' },
        { key: 'owner', label: 'Sahip' },
        { key: 'content', label: 'İçerik (boş geçilebilir)', type: 'textarea' },
      ],
    })
    if (!v || !v.title.trim()) return
    await ismsPost('/api/v1/isms/policies', v)
    load()
  }

  const transition = async (p: Policy, status: string) => {
    await ismsPost(`/api/v1/isms/policies/${p.id}/transition`, { status })
    load()
  }

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Panel title="Politika Yaşam Döngüsü" right={<span className="text-[10px] text-tui-dim">taslak → inceleme → onay → yayın</span>}>
        <div className="space-y-1">
          <div className="mb-1.5">
            <button onClick={addPolicy} className={btnCls}>
              + politika
            </button>
          </div>
          {error ? (
            <p className="text-[11px] text-rose-400">{error}</p>
          ) : policies.length === 0 ? (
            <p className="text-[11px] text-tui-dim">politika yok — akış: taslak → inceleme → onay → yayın</p>
          ) : (
            policies.map((p) => (
              <div key={p.id} className="flex flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1.5 text-[11px]">
                <span className="text-[10px] text-rx">{p.ref}</span>
                <span className="text-ink">{p.title}</span>
                <span className="hidden truncate text-[10px] text-tui-dim sm:inline">v{p.version}</span>
                {pill(p.status, statusTone(p.status))}
                {p.approved_by && <span className="text-[10px] text-tui-dim">{p.approved_by}</span>}
                {p.next_review > 0 && (
                  <span className="hidden truncate text-[10px] text-tui-dim sm:inline">inceleme: {new Date(p.next_review * 1000).toLocaleDateString('tr-TR')}</span>
                )}
                <span className="ml-auto flex gap-1">
                  {p.status === 'draft' && (
                    <button onClick={() => transition(p, 'in_review')} className={btnCls}>
                      incelemeye al
                    </button>
                  )}
                  {p.status === 'in_review' && (
                    <button onClick={() => transition(p, 'approved')} className={btnCls}>
                      onayla
                    </button>
                  )}
                  {p.status === 'approved' && (
                    <button onClick={() => transition(p, 'published')} className={btnCls}>
                      yayınla
                    </button>
                  )}
                </span>
              </div>
            ))
          )}
        </div>
      </Panel>
    </div>
  )
}
