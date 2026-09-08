import { useCallback, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useDialog } from '../lib/dialog'
import { useHotkeys } from '../lib/useHotkeys'
import { askAI } from '../lib/ai'
import { Markdown } from '../lib/markdown'
import { Panel } from '../components/Panel'
import type { Incident } from '../components/IncidentsPanel'

function relTime(unix: number): string {
  if (!unix) return '—'
  const s = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (s < 60) return `${s} sn`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} dk`
  const h = Math.floor(m / 60)
  return h < 48 ? `${h} sa` : `${Math.floor(h / 24)} g`
}

interface Evidence {
  kind: string // alert | event
  ref: string
  ts: number
  summary: string
}
interface Detail {
  incident: Incident & {
    correlation_key: string
    summary: string
    ack_by?: string
    resolved_ts?: number
    created_ts: number
    updated_ts: number
  }
  evidence: Evidence[]
}

const ACTIONS: { v: string; label: string; danger?: boolean }[] = [
  { v: 'ack', label: 'Kabul Et' },
  { v: 'investigate', label: 'İncele' },
  { v: 'resolve', label: 'Çöz', danger: true },
  { v: 'close', label: 'Kapat', danger: true },
]

function clock(ts: number) {
  return new Date(ts * 1000).toLocaleString('tr-TR')
}

export function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { confirm } = useDialog()
  const [data, setData] = useState<Detail | null>(null)
  const [state, setState] = useState<'loading' | 'ok' | 'notfound'>('loading')
  const [busy, setBusy] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    try {
      const res = await fetch(`/api/v1/incidents/${id}`)
      if (res.status === 404) {
        setState('notfound')
        return
      }
      if (!res.ok) return
      setData(await res.json())
      setState('ok')
    } catch {
      /* poll tekrar dener */
    }
  }, [id])

  useEffect(() => {
    void load()
    const t = window.setInterval(load, 15_000)
    return () => window.clearInterval(t)
  }, [load])

  useHotkeys([{ key: 'Escape', handler: () => navigate('/uyarilar'), allowInField: true }])

  // otomatik AI triyaj notu (Faz 26-E): scope=incident + source=triage konuşması
  const [triage, setTriage] = useState<{ text: string; ts: number } | null>(null)
  useEffect(() => {
    if (!id) return
    let stop = false
    ;(async () => {
      try {
        const r = await fetch(`/api/v1/ai/conversations?scope=incident&ref=${id}&source=triage`)
        if (!r.ok) return
        const convs = (await r.json()) as { id: number }[]
        if (!convs.length) return
        const d = await fetch(`/api/v1/ai/conversations/${convs[0].id}`).then((x) => x.json())
        const last = [...(d.messages ?? [])].reverse().find((m: { role: string }) => m.role === 'assistant')
        if (!stop && last?.content) setTriage({ text: last.content, ts: last.created_ts })
      } catch {
        /* AI kapalı olabilir — yoksay */
      }
    })()
    return () => {
      stop = true
    }
  }, [id])

  const doAction = async (action: string, label: string) => {
    if (!id) return
    if ((action === 'resolve' || action === 'close') && !(await confirm(`Bu olay "${label.toLowerCase()}" olarak işaretlensin mi?`, { danger: true }))) return
    setBusy(action)
    try {
      const res = await fetch(`/api/v1/incidents/${id}/${action}`, { method: 'POST' })
      if (res.ok) await load()
    } finally {
      setBusy('')
    }
  }

  if (state === 'loading') {
    return <p className="mx-auto max-w-[1400px] px-4 py-10 text-center font-mono text-[11px] text-tui-dim">Yükleniyor…</p>
  }
  if (state === 'notfound' || !data) {
    return (
      <p className="mx-auto max-w-[1400px] px-4 py-10 text-center font-mono text-[11px] text-tui-dim">
        Olay bulunamadı.{' '}
        <Link to="/uyarilar" className="text-rx hover:underline">
          Uyarılara dön
        </Link>
      </p>
    )
  }

  const i = data.incident
  const sevTone = i.severity === 'crit' ? 'text-rose-400' : i.severity === 'warn' ? 'text-amber-400' : 'text-sky-400'

  return (
    <div className="mx-auto max-w-[1400px] space-y-3 px-4 py-3 font-mono">
      <div className="flex flex-wrap items-center gap-2 text-[11px]">
        <Link to="/uyarilar" className="text-tui-dim hover:text-rx">
          ← Uyarılar
        </Link>
        <span className="text-rule-hi">/</span>
        <span className="text-ink-hi">OLAY #{i.id}</span>
        <span className={`uppercase ${sevTone}`}>{i.severity}</span>
        <span className="border border-rule px-1.5 py-0.5 uppercase text-tui-dim">{i.status}</span>
        <span className="ml-auto flex gap-1.5">
          <button
            type="button"
            onClick={() => void askAI(navigate, 'incident', id ?? '', 'incident_triage')}
            className="border border-rx/40 px-2 py-0.5 text-[10px] uppercase text-rx transition hover:bg-rx/10"
          >
            AI Triyaj
          </button>
          {ACTIONS.map((a) => (
            <button
              key={a.v}
              type="button"
              disabled={!!busy}
              onClick={() => void doAction(a.v, a.label)}
              className={`border px-2 py-0.5 text-[10px] uppercase transition disabled:opacity-40 ${
                a.danger ? 'border-rose-500/40 text-rose-400 hover:bg-rose-500/10' : 'border-rule-hi text-tui-dim hover:border-rx/40 hover:text-rx'
              }`}
            >
              {busy === a.v ? '…' : a.label}
            </button>
          ))}
        </span>
      </div>

      <h1 className="text-[13px] font-bold uppercase tracking-[0.04em] text-ink-hi">{i.title}</h1>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1.5fr]">
        <Panel title="Özet">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-[11px]">
            {(
              [
                ['Risk', String(i.risk_score) + ' / 100'],
                ['Agent', i.agent_id ? <Link key="a" to={`/agentlar/${i.agent_id}`} className="text-rx hover:underline">#{i.agent_id}</Link> : '—'],
                ['Saha', i.site || '—'],
                ['Kabul eden', i.ack_by || '—'],
                ['Açıldı', relTime(i.created_ts)],
                ['Son etkinlik', relTime(i.last_seen)],
              ] as [string, ReactNode][]
            ).map(([k, v]) => (
              <div key={k} className="min-w-0">
                <dt className="text-[10px] uppercase tracking-[0.04em] text-tui-dim">{k}</dt>
                <dd className="truncate text-ink-hi">{v}</dd>
              </div>
            ))}
          </dl>
        </Panel>

        <Panel title="Korelasyon">
          <p className="text-[11px] text-ink-hi">{i.correlation_reason}</p>
          {i.summary && <p className="mt-2 text-[11px] text-tui-dim">{i.summary}</p>}
          <p className="mt-2 text-[10px] text-tui-dim">
            dedup anahtarı: <span className="text-ink">{i.correlation_key}</span>
          </p>
        </Panel>
      </div>

      <Panel title="Kanıt Zaman Çizelgesi" right={<span className="text-[10px] text-tui-dim">{data.evidence.length} kanıt · kronolojik</span>}>
        {data.evidence.length === 0 ? (
          <p className="py-4 text-center text-[11px] text-tui-dim">Kanıt yok.</p>
        ) : (
          <ol className="space-y-1 text-[11px]">
            {data.evidence.map((e, idx) => (
              <li key={`${e.kind}-${e.ref}-${idx}`} className="flex gap-2">
                <span className="w-40 shrink-0 text-tui-dim">{clock(e.ts)}</span>
                <span className={`w-14 shrink-0 uppercase ${e.kind === 'alert' ? 'text-amber-400' : 'text-sky-400'}`}>{e.kind}</span>
                <span className="min-w-0 flex-1 text-ink">{e.summary}</span>
              </li>
            ))}
          </ol>
        )}
      </Panel>

      {triage && (
        <Panel
          title="AI Triyaj"
          right={<span className="text-[10px] text-tui-dim">otomatik · {clock(triage.ts)}</span>}
        >
          <Markdown text={triage.text} />
          <p className="mt-2 text-[10px] text-tui-dim">
            AI danışmandır — deterministik korelasyon + risk skoru yetkilidir.{' '}
            <Link to={`/ai?c=`} className="text-rx hover:underline" onClick={(e) => { e.preventDefault(); void askAI(navigate, 'incident', id ?? '') }}>
              Sohbete devam et →
            </Link>
          </p>
        </Panel>
      )}

      <Panel title="İlişkili">
        <div className="flex flex-wrap gap-2 text-[11px]">
          {i.agent_id > 0 && (
            <Link to={`/agentlar/${i.agent_id}`} className="border border-rule px-2 py-0.5 text-tui-dim hover:border-rx/40 hover:text-rx">
              Agent #{i.agent_id}
            </Link>
          )}
          <Link to="/uyarilar" className="border border-rule px-2 py-0.5 text-tui-dim hover:border-rx/40 hover:text-rx">
            Alarmlar
          </Link>
          <Link to="/akis" className="border border-rule px-2 py-0.5 text-tui-dim hover:border-rx/40 hover:text-rx">
            Canlı Akış
          </Link>
        </div>
      </Panel>
    </div>
  )
}
