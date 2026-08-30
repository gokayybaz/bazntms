import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ask, btnCls, ismsPost, pill, statusTone, type Policy } from '../lib/isms'

export function PoliciesPage() {
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
    const title = ask('Politika başlığı:')
    if (!title) return
    await ismsPost('/api/v1/isms/policies', { title, owner: ask('Sahip:'), content: ask('İçerik (boş geçilebilir):') })
    load()
  }

  const transition = async (p: Policy, status: string) => {
    await ismsPost(`/api/v1/isms/policies/${p.id}/transition`, { status })
    load()
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Card title="Politika Yaşam Döngüsü" right={<span className="text-xs text-slate-500">taslak → inceleme → onay → yayın</span>}>
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <button onClick={addPolicy} className={btnCls}>
              + politika
            </button>
          </div>
          {error ? (
            <p className="text-xs text-rose-400">{error}</p>
          ) : policies.length === 0 ? (
            <p className="text-xs text-slate-600">politika yok — akış: taslak → inceleme → onay → yayın</p>
          ) : (
            policies.map((p) => (
              <div key={p.id} className="flex flex-wrap items-center gap-2 rounded border border-slate-800 bg-slate-900/50 px-2.5 py-1.5">
                <span className="font-mono text-[10px] text-cyan-300">{p.ref}</span>
                <span className="text-xs text-slate-300">{p.title}</span>
                <span className="font-mono text-[10px] text-slate-600">v{p.version}</span>
                {pill(p.status, statusTone(p.status))}
                {p.approved_by && <span className="font-mono text-[10px] text-slate-500">{p.approved_by}</span>}
                {p.next_review > 0 && (
                  <span className="text-[10px] text-slate-600">inceleme: {new Date(p.next_review * 1000).toLocaleDateString('tr-TR')}</span>
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
      </Card>
    </div>
  )
}
