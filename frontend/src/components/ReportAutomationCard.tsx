import { useCallback, useEffect, useState } from 'react'
import { useDialog } from '../lib/dialog'

// GET /api/v1/reports/* yanıt tipleri (yerel — CLAUDE.md konvansiyonu).
interface Schedule {
  id: number
  spec: string
  enabled: boolean
  next_run_ts: number
  last_run_ts: number
  last_status: string
  payload: { type: string; days: number; site: string; format: string; email: string[] }
}
interface Archive {
  id: number
  kind: string
  site: string
  days: number
  format: string
  size: number
  generated_ts: number
  delivered_to: string
  status: string
}

function ts(v: number): string {
  return v ? new Date(v * 1000).toLocaleString('tr-TR') : '—'
}
function kb(n: number): string {
  return n < 1024 ? `${n} B` : `${(n / 1024).toFixed(0)} KB`
}

export function ReportAutomationCard() {
  const { form, confirm } = useDialog()
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [archive, setArchive] = useState<Archive[]>([])
  const [err, setErr] = useState('')

  const load = useCallback(async () => {
    try {
      const [s, a] = await Promise.all([
        fetch('/api/v1/reports/schedules').then((r) => r.json()),
        fetch('/api/v1/reports/archive').then((r) => r.json()),
      ])
      setSchedules(s.schedules ?? [])
      setArchive(a.archive ?? [])
      setErr('')
    } catch {
      setErr('liste alınamadı')
    }
  }, [])

  useEffect(() => {
    load()
    const id = window.setInterval(load, 30_000)
    return () => window.clearInterval(id)
  }, [load])

  const addSchedule = async () => {
    const res = await form({
      title: 'Rapor zamanlaması',
      fields: [
        { key: 'type', label: 'Rapor türü', type: 'select', options: ['enterprise', 'compliance', 'traffic'], defaultValue: 'enterprise' },
        { key: 'spec', label: 'Zamanlama', type: 'text', defaultValue: 'weekly:mon:07:00', placeholder: 'daily:08:00 · weekly:mon:07:00 · monthly:1:06:00 · interval:120' },
        { key: 'days', label: 'Pencere (gün)', type: 'number', defaultValue: '30' },
        { key: 'site', label: 'Saha (boş = filo)', type: 'text', defaultValue: '' },
        { key: 'format', label: 'Format', type: 'select', options: ['html', 'pdf'], defaultValue: 'pdf' },
        { key: 'email', label: 'E-posta (virgüllü)', type: 'text', defaultValue: '' },
      ],
      confirmLabel: 'Ekle',
    })
    if (!res) return
    const body = {
      spec: res.spec,
      payload: {
        type: res.type,
        days: Number(res.days) || 30,
        site: res.site || '',
        format: res.format,
        email: (res.email || '').split(',').map((s) => s.trim()).filter(Boolean),
      },
    }
    const r = await fetch('/api/v1/reports/schedules', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!r.ok) {
      setErr((await r.text()) || 'eklenemedi')
      return
    }
    await load()
  }

  const del = async (id: number) => {
    if (!(await confirm('Zamanlama silinsin mi?'))) return
    await fetch(`/api/v1/reports/schedules/${id}`, { method: 'DELETE' })
    await load()
  }

  const genNow = async (sc: Schedule) => {
    if (!(await confirm(`"${sc.payload.type}" raporu şimdi üretilsin mi?`))) return
    const r = await fetch('/api/v1/reports/generate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sc.payload) })
    if (!r.ok) setErr('üretilemedi')
    await load()
  }

  return (
    <div className="space-y-4 font-mono text-[11px]">
      {err && <p className="text-rose-400">⚠ {err}</p>}

      <div>
        <div className="mb-1.5 flex items-center gap-2">
          <span className="uppercase tracking-[0.06em] text-rx">Zamanlamalar</span>
          <button onClick={addSchedule} className="border border-rule px-2 py-0.5 text-[10px] uppercase text-tui-dim hover:text-ink-hi">
            + ekle
          </button>
        </div>
        {schedules.length === 0 ? (
          <p className="text-tui-dim">Zamanlama yok — rapor yalnızca elle üretilir.</p>
        ) : (
          <div className="space-y-1">
            {schedules.map((s) => (
              <div key={s.id} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 border-b border-rule py-1 last:border-0">
                <span className="text-ink-hi">{s.payload.type}</span>
                <span className="text-tui-dim">{s.spec}</span>
                <span className="text-tui-dim">{s.payload.site || 'filo'} · {s.payload.days}g · {s.payload.format}</span>
                {s.payload.email.length > 0 && <span className="text-tui-dim">→ {s.payload.email.join(', ')}</span>}
                <span className="text-tui-dim">sıradaki {ts(s.next_run_ts)}</span>
                {s.last_status && s.last_status !== 'ok' && <span className="text-amber-400">{s.last_status}</span>}
                <span className="ml-auto flex gap-2">
                  <button onClick={() => genNow(s)} className="text-rx hover:underline">şimdi üret</button>
                  <button onClick={() => del(s.id)} className="text-rose-400 hover:underline">sil</button>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div>
        <div className="mb-1.5 uppercase tracking-[0.06em] text-rx">Arşiv ({archive.length})</div>
        {archive.length === 0 ? (
          <p className="text-tui-dim">Henüz üretilmiş rapor yok.</p>
        ) : (
          <div className="max-h-72 space-y-0.5 overflow-y-auto">
            {archive.map((a) => (
              <div key={a.id} className="flex flex-wrap items-baseline gap-x-3 border-b border-rule py-0.5 last:border-0">
                <a href={`/api/v1/reports/archive/${a.id}`} target="_blank" rel="noreferrer" className="text-rx hover:underline">
                  {a.kind}{a.site ? ` · ${a.site}` : ''} · {a.format}
                </a>
                <span className="text-tui-dim">{ts(a.generated_ts)}</span>
                <span className="text-tui-dim">{kb(a.size)}</span>
                {a.delivered_to && <span className="text-emerald-400">→ {a.delivered_to}</span>}
                {a.status !== 'ok' && <span className="text-amber-400">{a.status}</span>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
