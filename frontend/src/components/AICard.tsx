import { useCallback, useEffect, useState } from 'react'
import type { AIStatus, Insight, ModelInfo } from '../types'
import { formatNum } from '../lib/format'

const PERIODS = [
  { label: '15 dk', minutes: 15 },
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
  { label: '24 saat', minutes: 1440 },
]

export function AICard({ onAnalyzed }: { onAnalyzed: () => void }) {
  const [status, setStatus] = useState<AIStatus | null>(null)
  const [models, setModels] = useState<ModelInfo[]>([])
  const [model, setModel] = useState('')
  const [minutes, setMinutes] = useState(60)
  const [chunked, setChunked] = useState(true)
  const [summary, setSummary] = useState('')
  const [insights, setInsights] = useState<Insight[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const loadInsights = useCallback(async () => {
    try {
      setInsights(await fetch('/api/ai/insights').then((r) => r.json()))
    } catch {
      /* yoksay */
    }
  }, [])

  useEffect(() => {
    fetch('/api/ai/status')
      .then((r) => r.json())
      .then((s: AIStatus) => {
        setStatus(s)
        if (s.enabled) {
          setModel(s.model)
          fetch('/api/ai/models')
            .then((r) => r.json())
            .then((d) => {
              if (d.ok && Array.isArray(d.models)) setModels(d.models)
            })
            .catch(() => {})
        }
      })
      .catch(() => setStatus({ enabled: false, model: '-' }))
    loadInsights()
  }, [loadInsights])

  const analyze = useCallback(async () => {
    setBusy(true)
    setError('')
    setSummary('')
    try {
      const res = await fetch('/api/ai/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ minutes, model: model || undefined, chunked }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) throw new Error(data.error ?? `HTTP ${res.status}`)
      setSummary(data.summary)
      onAnalyzed()
      loadInsights()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }, [minutes, model, chunked, loadInsights, onAnalyzed])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={`inline-flex items-center gap-1.5 rounded px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
            status?.enabled
              ? 'bg-emerald-500/10 text-emerald-400 ring-1 ring-emerald-500/30'
              : 'bg-slate-500/10 text-slate-400 ring-1 ring-slate-500/30'
          }`}
        >
          <span className={`size-1.5 rounded-full ${status?.enabled ? 'bg-emerald-400' : 'bg-slate-500'}`} />
          {status?.enabled ? `AI hazır · ${status.model}` : 'AI pasif'}
        </span>
        <div className="flex rounded-lg border border-slate-700/80 p-0.5">
          {PERIODS.map((p) => (
            <button
              key={p.minutes}
              onClick={() => setMinutes(p.minutes)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition ${
                minutes === p.minutes ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
        {models.length > 0 && (
          <select
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className="rounded-lg border border-slate-700/80 bg-slate-900 px-2.5 py-1.5 text-xs text-slate-200 outline-none focus:border-fuchsia-500/60"
            title="kullanılacak model"
          >
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.id}
              </option>
            ))}
          </select>
        )}
        <label
          className="flex cursor-pointer items-center gap-1.5 text-xs text-slate-400 select-none"
          title="Veri 3 parçaya bölünüp ayrı isteklerle gönderilir; küçük modellerde context dolmaz"
        >
          <input
            type="checkbox"
            checked={chunked}
            onChange={(e) => setChunked(e.target.checked)}
            className="size-3.5 accent-fuchsia-500"
          />
          parça parça gönder
        </label>
        <button
          onClick={analyze}
          disabled={busy || !status?.enabled}
          className="ml-auto rounded-lg bg-violet-600 px-4 py-1.5 text-sm font-semibold text-white transition enabled:hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {busy ? 'Analiz ediliyor…' : 'AI ile Analiz Et'}
        </button>
      </div>

      {!status?.enabled && (
        <p className="rounded-lg bg-slate-800/50 px-3 py-2 text-xs text-slate-400">
          Yerel modeller için sunucuyu şöyle başlatın:{' '}
          <code className="text-slate-300">./bazntms -llm-base-url http://localhost:11434/v1</code>{' '}
          (Ollama). LM Studio: <code className="text-slate-300">http://localhost:1234/v1</code>. Bulut servisleri
          için: <code className="text-slate-300">LLM_API_KEY</code> + opsiyonel{' '}
          <code className="text-slate-300">LLM_BASE_URL</code>/<code className="text-slate-300">LLM_MODEL</code>.
        </p>
      )}

      {error && (
        <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-400 ring-1 ring-rose-500/30">{error}</p>
      )}

      {busy && (
        <div className="flex items-center gap-3 rounded-lg bg-slate-800/50 px-4 py-6">
          <span className="size-2 animate-ping rounded-full bg-fuchsia-400" />
          <span className="text-sm text-slate-400">Veriler hazırlanıyor ve modele gönderiliyor…</span>
        </div>
      )}

      {summary && (
        <div className="rounded-lg border border-fuchsia-500/20 bg-fuchsia-500/5 px-4 py-3">
          <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-fuchsia-400">AI Analizi</p>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-200">{summary}</p>
        </div>
      )}

      {insights.length > 0 && (
        <div>
          <p className="mb-2 text-[11px] font-medium uppercase tracking-wider text-slate-500">
            Önceki Analizler ({formatNum(insights.length)})
          </p>
          <ul className="space-y-1.5">
            {insights.slice(0, 5).map((i) => (
              <li key={i.id}>
                <button
                  onClick={() => setSummary(i.summary)}
                  className="w-full rounded-lg border border-slate-800 px-3 py-2 text-left text-xs text-slate-400 transition hover:border-slate-600 hover:text-slate-200"
                >
                  <span className="font-mono text-slate-500">
                    {new Date(i.ts * 1000).toLocaleString('tr-TR')}
                  </span>{' '}
                  · {i.period_minutes} dk · {i.model}
                  <span className="ml-2 line-clamp-1 inline">{i.summary.slice(0, 90)}…</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
