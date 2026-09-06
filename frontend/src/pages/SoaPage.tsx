import { useCallback, useEffect, useState } from 'react'
import { Panel } from '../components/Panel'
import { ComplianceSubNav } from '../components/ComplianceSubNav'
import { useDialog } from '../lib/dialog'
import { ismsPut, pill, statusTone, type SoaItem } from '../lib/isms'

export function SoaPage() {
  const { form } = useDialog()
  const [soa, setSoa] = useState<SoaItem[]>([])
  const [filter, setFilter] = useState<'hepsi' | 'planned' | 'uygulanan' | 'haric'>('hepsi')
  const [q, setQ] = useState('')
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
    const v = await form({
      title: `${c.control_id} — ${c.title}`,
      fields: [
        { key: 'applicable', label: 'Uygulanabilir mi', type: 'select', options: ['evet', 'hayır'], defaultValue: c.applicable ? 'evet' : 'hayır' },
        { key: 'status', label: 'Durum', type: 'select', options: ['planned', 'implemented', 'verified'], defaultValue: c.status },
        { key: 'justification', label: 'Gerekçe (hariçse zorunlu)', type: 'textarea', defaultValue: c.justification },
        { key: 'evidence', label: 'Kanıt notu', defaultValue: c.evidence },
        { key: 'owner', label: 'Sahip', defaultValue: c.owner },
      ],
    })
    if (!v) return
    await ismsPut(`/api/v1/isms/soa/${c.control_id}`, {
      applicable: v.applicable === 'evet',
      status: v.status,
      justification: v.justification,
      evidence: v.evidence,
      owner: v.owner,
    })
    load()
  }

  const filtered = soa
    .filter((c) =>
      filter === 'hepsi'
        ? true
        : filter === 'planned'
          ? c.applicable && c.status === 'planned'
          : filter === 'uygulanan'
            ? c.applicable && (c.status === 'implemented' || c.status === 'verified')
            : !c.applicable,
    )
    .filter((c) => {
      const n = q.trim().toLocaleLowerCase('tr')
      return !n || `${c.control_id} ${c.title} ${c.owner}`.toLocaleLowerCase('tr').includes(n)
    })

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyumluluk</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">5651 + ISO 27001</span>
      </div>

      <ComplianceSubNav />

      <Panel title="Statement of Applicability" right={<span className="text-[10px] text-tui-dim">{filtered.length} / {soa.length} kontrol</span>}>
        <div className="space-y-1">
          <div className="mb-1.5 flex flex-wrap items-center gap-2">
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value as typeof filter)}
              className="border border-rule-hi bg-ground px-1.5 py-0.5 text-[10px] text-ink"
            >
              <option value="hepsi">hepsi</option>
              <option value="planned">planlanan</option>
              <option value="uygulanan">uygulanan</option>
              <option value="haric">hariç</option>
            </select>
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="kontrol / başlık / sahip filtrele…"
              aria-label="SoA filtresi"
              className="min-w-0 flex-1 border border-rule-hi bg-ground px-2 py-0.5 text-[11px] text-ink outline-none placeholder:text-tui-dim focus:border-rx/60"
            />
          </div>
          {error ? (
            <p className="text-[11px] text-rose-400">{error}</p>
          ) : (
            <div className="max-h-[calc(100vh-320px)] space-y-0.5 overflow-y-auto">
              {filtered.map((c) => (
                <button
                  key={c.control_id}
                  onClick={() => updateSoa(c)}
                  className="flex w-full flex-wrap items-center gap-2 border border-rule bg-panel-2/40 px-2.5 py-1 text-left transition hover:border-rule-hi"
                >
                  <span className="text-[10px] text-rx">{c.control_id}</span>
                  <span className="text-[11px] text-ink">{c.title}</span>
                  {!c.applicable && pill('hariç', 'bad')}
                  {pill(c.status, statusTone(c.status))}
                  {c.evidence && (
                    <span className="ml-auto max-w-[45%] truncate text-[10px] text-tui-dim" title={c.evidence}>
                      {c.evidence}
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}
          <p className="text-[10px] text-tui-dim">satıra tıklayarak uygula/hariç kararı, gerekçe, kanıt ve sahip güncellenir</p>
        </div>
      </Panel>
    </div>
  )
}
