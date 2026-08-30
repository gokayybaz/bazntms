import { useCallback, useEffect, useState } from 'react'
import { Card } from '../components/Card'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { ask, ismsPut, pill, statusTone, type SoaItem } from '../lib/isms'

export function SoaPage() {
  const [soa, setSoa] = useState<SoaItem[]>([])
  const [filter, setFilter] = useState<'hepsi' | 'planned' | 'uygulanan' | 'haric'>('hepsi')
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/isms/soa')
      if (!res.ok) throw new Error('SoA alınamadı')
      setSoa(await res.json())
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const updateSoa = async (c: SoaItem) => {
    const applicable = confirm(`${c.control_id} ${c.title}\n\nUygulanacak mı? (İptal = hariç, gerekçe istenir)`)
    const status = ask('Durum (planned/implemented/verified):', c.status) || c.status
    const justification = applicable ? c.justification : ask('Hariç gerekçesi:', c.justification)
    const evidence = ask('Kanıt notu:', c.evidence)
    const owner = ask('Sahip:', c.owner)
    await ismsPut(`/api/v1/isms/soa/${c.control_id}`, { applicable, status, justification, evidence, owner })
    load()
  }

  const filtered = soa.filter((c) =>
    filter === 'hepsi'
      ? true
      : filter === 'planned'
        ? c.applicable && c.status === 'planned'
        : filter === 'uygulanan'
          ? c.applicable && (c.status === 'implemented' || c.status === 'verified')
          : !c.applicable,
  )

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Card title="Statement of Applicability" right={<span className="text-xs text-slate-500">{soa.length} kontrol</span>}>
        <div className="space-y-1">
          <div className="mb-1.5 flex items-center gap-2">
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value as typeof filter)}
              className="rounded border border-slate-700 bg-slate-900 px-1.5 py-0.5 text-[10px] text-slate-300"
            >
              <option value="hepsi">hepsi</option>
              <option value="planned">planlanan</option>
              <option value="uygulanan">uygulanan</option>
              <option value="haric">hariç</option>
            </select>
          </div>
          {error ? (
            <p className="text-xs text-rose-400">{error}</p>
          ) : (
            <div className="max-h-[calc(100vh-320px)] space-y-0.5 overflow-y-auto pr-1">
              {filtered.map((c) => (
                <button
                  key={c.control_id}
                  onClick={() => updateSoa(c)}
                  className="flex w-full flex-wrap items-center gap-2 rounded border border-slate-800/70 bg-slate-900/40 px-2.5 py-1 text-left transition hover:border-slate-600"
                >
                  <span className="font-mono text-[10px] text-cyan-300">{c.control_id}</span>
                  <span className="text-[11px] text-slate-300">{c.title}</span>
                  {!c.applicable && pill('hariç', 'bad')}
                  {pill(c.status, statusTone(c.status))}
                  {c.evidence && (
                    <span className="ml-auto max-w-[45%] truncate text-[10px] text-slate-600" title={c.evidence}>
                      {c.evidence}
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}
          <p className="text-[10px] text-slate-600">satıra tıklayarak uygula/hariç kararı, gerekçe, kanıt ve sahip güncellenir</p>
        </div>
      </Card>
    </div>
  )
}
