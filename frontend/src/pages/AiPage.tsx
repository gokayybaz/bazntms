import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Panel } from '../components/Panel'
import { PanelState } from '../components/PanelState'
import { Markdown } from '../lib/markdown'
import { useRegisterKeys } from '../lib/KeymapContext'
import {
  streamMessage,
  type AiConversation,
  type AiMessage,
  type AiPreset,
  type AiStatus,
} from '../lib/ai'

// GET /api/v1/ai/conversations/{id} yanıtı
interface ConvDetail {
  conversation: AiConversation
  messages: AiMessage[]
}

const SCOPE_LABEL: Record<string, string> = {
  fleet: 'Filo', agent: 'Agent', incident: 'Olay', anomaly: 'Anomali', device: 'Cihaz',
}

function relTime(unix: number): string {
  const s = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (s < 60) return `${s} sn`
  if (s < 3600) return `${Math.floor(s / 60)} dk`
  if (s < 86400) return `${Math.floor(s / 3600)} sa`
  return `${Math.floor(s / 86400)} g`
}

export function AiPage() {
  const [params, setParams] = useSearchParams()
  const activeID = params.get('c') ? Number(params.get('c')) : null
  const pendingPreset = params.get('preset')

  const [status, setStatus] = useState<AiStatus | null>(null)
  const [presets, setPresets] = useState<AiPreset[]>([])
  const [convs, setConvs] = useState<AiConversation[]>([])
  const [detail, setDetail] = useState<ConvDetail | null>(null)
  const [detailLoaded, setDetailLoaded] = useState(false)
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)

  const loadConvs = useCallback(async () => {
    try {
      const r = await fetch('/api/v1/ai/conversations')
      if (r.ok) setConvs(await r.json())
    } catch {
      /* yoksay */
    }
  }, [])

  const loadDetail = useCallback(async (id: number) => {
    setDetailLoaded(false)
    try {
      const r = await fetch(`/api/v1/ai/conversations/${id}`)
      if (r.ok) setDetail(await r.json())
      else setDetail(null)
    } catch {
      setDetail(null)
    } finally {
      setDetailLoaded(true)
    }
  }, [])

  useEffect(() => {
    fetch('/api/v1/ai/status')
      .then((r) => r.json())
      .then(setStatus)
      .catch(() => setStatus({ enabled: false }))
    fetch('/api/v1/ai/presets')
      .then((r) => r.json())
      .then((d) => setPresets(d.presets ?? []))
      .catch(() => {})
    loadConvs()
  }, [loadConvs])

  useEffect(() => {
    if (activeID) loadDetail(activeID)
    else {
      setDetail(null)
      setDetailLoaded(true)
    }
  }, [activeID, loadDetail])

  // akış / mesaj sonrası en alta kaydır
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [detail?.messages?.length, streaming])

  const send = useCallback(
    (opts: { content?: string; preset?: string }) => {
      if (busy || !activeID) return
      const content = opts.content ?? input
      if (!content.trim() && !opts.preset) return
      setBusy(true)
      setErr('')
      setStreaming('')
      setInput('')
      // iyimser: kullanıcı mesajını hemen göster
      setDetail((d) =>
        d
          ? {
              ...d,
              messages: [
                ...(d.messages ?? []),
                { id: -1, role: 'user', content: content || `[${opts.preset}]`, tokens_in: 0, tokens_out: 0, created_ts: Math.floor(Date.now() / 1000) },
              ],
            }
          : d,
      )
      abortRef.current = streamMessage(activeID, { content, preset: opts.preset }, {
        onDelta: (t) => setStreaming((s) => s + t),
        onError: (e) => {
          setErr(e)
          setBusy(false)
          setStreaming('')
          loadDetail(activeID)
        },
        onDone: () => {
          setBusy(false)
          setStreaming('')
          loadDetail(activeID)
          loadConvs()
        },
      })
    },
    [busy, activeID, input, loadDetail, loadConvs],
  )

  // sayfa-farkında derin bağlantı: ?c=&preset= → o preset'i bir kez gönder
  const firedPreset = useRef(false)
  useEffect(() => {
    if (activeID && pendingPreset && detailLoaded && detail && (detail.messages?.length ?? 0) === 0 && !firedPreset.current) {
      firedPreset.current = true
      send({ preset: pendingPreset })
      params.delete('preset')
      setParams(params, { replace: true })
    }
  }, [activeID, pendingPreset, detailLoaded, detail, send, params, setParams])

  const newConv = useCallback(async () => {
    const r = await fetch('/api/v1/ai/conversations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scope_kind: 'fleet' }),
    })
    if (r.ok) {
      const d = await r.json()
      await loadConvs()
      setParams({ c: String(d.id) })
    }
  }, [loadConvs, setParams])

  const archiveActive = useCallback(async () => {
    if (!activeID) return
    await fetch(`/api/v1/ai/conversations/${activeID}`, { method: 'DELETE' })
    await loadConvs()
    setParams({})
  }, [activeID, loadConvs, setParams])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    setBusy(false)
    setStreaming('')
    if (activeID) loadDetail(activeID)
  }, [activeID, loadDetail])

  useRegisterKeys(
    useMemo(
      () => [
        { key: 'F2', label: 'Yeni', handler: newConv },
        ...(activeID ? [{ key: 'F8', label: 'Arşivle', handler: archiveActive }] : []),
        ...(busy ? [{ key: 'F10', label: 'İptal', handler: cancel }] : []),
      ],
      [newConv, archiveActive, cancel, activeID, busy],
    ),
  )

  const scopePresets = useMemo(() => {
    const sc = detail?.conversation.scope_kind ?? 'fleet'
    return presets.filter((p) => !p.scopes || p.scopes.length === 0 || p.scopes.includes(sc))
  }, [presets, detail])

  if (status && !status.enabled) {
    return (
      <div className="mx-auto max-w-[900px] px-4 py-6 font-mono">
        <Panel title="AI Analiz">
          <PanelState
            kind="empty"
            message="AI analiz kapalı."
            hint="Hub'ı -ai bayrağıyla başlatın, sonra Yönetim > AI Sağlayıcı'dan bir model ekleyin (yerel: Ollama/LM Studio; bulut: OpenAI/Anthropic)."
          />
        </Panel>
      </div>
    )
  }

  return (
    <div className="mx-auto flex h-full max-w-[1600px] gap-3 px-4 py-3 font-mono">
      {/* konuşma listesi */}
      <aside className="hidden w-56 shrink-0 flex-col md:flex">
        <Panel
          title="Sohbetler"
          className="flex min-h-0 flex-1 flex-col"
          bodyClassName="min-h-0 flex-1 overflow-y-auto p-0"
          right={
            <button onClick={newConv} className="border border-rx px-1.5 py-0.5 text-[10px] text-rx hover:bg-rx/10">
              + F2
            </button>
          }
        >
          {convs.length === 0 ? (
            <PanelState kind="empty" message="Henüz sohbet yok." />
          ) : (
            <ul className="divide-y divide-rule">
              {convs.map((c) => (
                <li key={c.id}>
                  <button
                    onClick={() => setParams({ c: String(c.id) })}
                    className={`block w-full px-2.5 py-1.5 text-left text-[11px] transition ${
                      c.id === activeID ? 'bg-rx text-ground' : 'text-tui-dim hover:bg-panel-2 hover:text-ink-hi'
                    }`}
                  >
                    <div className="truncate">{c.title || 'Yeni sohbet'}</div>
                    <div className={`flex gap-1.5 text-[10px] uppercase ${c.id === activeID ? 'text-ground/70' : 'text-tui-dim'}`}>
                      <span>{SCOPE_LABEL[c.scope_kind] ?? c.scope_kind}</span>
                      {c.source !== 'user' && <span>· {c.source}</span>}
                      <span>· {relTime(c.updated_ts)}</span>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Panel>
      </aside>

      {/* sohbet */}
      <section className="flex min-w-0 flex-1 flex-col">
        <Panel
          title={detail ? detail.conversation.title || `${SCOPE_LABEL[detail.conversation.scope_kind]} sohbeti` : 'AI Analiz'}
          className="flex min-h-0 flex-1 flex-col"
          bodyClassName="flex min-h-0 flex-1 flex-col p-0"
          right={
            <div className="flex items-center gap-1.5 text-[10px] text-tui-dim">
              {status?.default_model && <span className="hidden sm:inline">{status.default_model}</span>}
              {activeID && (
                <button onClick={archiveActive} className="border border-rule px-1.5 py-0.5 hover:text-ink-hi">
                  Arşivle
                </button>
              )}
            </div>
          }
        >
          {!activeID ? (
            <PanelState
              kind="empty"
              message="Bir sohbet seçin ya da yeni başlatın."
              hint="Detay sayfalarındaki 'AI'ya Sor' düğmeleri de buraya getirir."
            />
          ) : !detailLoaded ? (
            <PanelState kind="loading" />
          ) : (
            <>
              {/* preset şeridi */}
              {scopePresets.length > 0 && (
                <div className="flex flex-wrap gap-1 border-b border-rule px-3 py-1.5">
                  {scopePresets.map((p) => (
                    <button
                      key={p.id}
                      disabled={busy}
                      onClick={() => send({ preset: p.id })}
                      className="border border-rule px-2 py-0.5 text-[10px] uppercase tracking-[0.03em] text-tui-dim transition hover:border-rx hover:text-rx disabled:opacity-40"
                    >
                      {p.label}
                    </button>
                  ))}
                </div>
              )}

              {/* transcript */}
              <div ref={scrollRef} className="min-h-0 flex-1 space-y-3 overflow-y-auto px-3 py-3">
                {(detail?.messages ?? [])
                  .filter((m) => m.role !== 'system')
                  .map((m) => (
                    <div key={m.id} className={m.role === 'user' ? 'flex justify-end' : ''}>
                      <div
                        className={
                          m.role === 'user'
                            ? 'max-w-[85%] border border-rule-hi bg-panel-2 px-2.5 py-1.5 text-[13px] text-ink'
                            : 'w-full border-l-2 border-rx/50 pl-3'
                        }
                      >
                        {m.role === 'user' ? (
                          <span className="whitespace-pre-wrap">{m.content}</span>
                        ) : (
                          <Markdown text={m.content} />
                        )}
                        {m.error && <p className="mt-1 text-[11px] text-rose-400">⚠ {m.error}</p>}
                      </div>
                    </div>
                  ))}
                {busy && (
                  <div className="w-full border-l-2 border-rx/50 pl-3">
                    {streaming ? (
                      <Markdown text={streaming} />
                    ) : (
                      <span className="text-[11px] text-tui-dim">
                        <span className="animate-pulse">●</span> düşünüyor…
                      </span>
                    )}
                  </div>
                )}
                {err && (
                  <p role="alert" className="border border-rose-400/40 bg-rose-400/5 px-2 py-1 text-[11px] text-rose-400">
                    {err}
                  </p>
                )}
              </div>

              {/* composer */}
              <form
                className="flex items-end gap-2 border-t border-rule px-3 py-2"
                onSubmit={(e) => {
                  e.preventDefault()
                  send({})
                }}
              >
                <textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                      e.preventDefault()
                      send({})
                    }
                  }}
                  rows={2}
                  placeholder={busy ? 'yanıt bekleniyor…' : 'Bir şey sor (Ctrl+Enter gönderir)'}
                  disabled={busy}
                  className="min-h-[2.4rem] flex-1 resize-y border border-rule-hi bg-ground px-2 py-1 text-[13px] text-ink outline-none placeholder:text-tui-dim focus:border-rx/60 disabled:opacity-50"
                />
                {busy ? (
                  <button
                    type="button"
                    onClick={cancel}
                    className="border border-rose-400 px-3 py-1.5 text-[11px] uppercase text-rose-400 hover:bg-rose-400/10"
                  >
                    İptal
                  </button>
                ) : (
                  <button
                    type="submit"
                    disabled={!input.trim()}
                    className="border border-rx bg-rx px-3 py-1.5 text-[11px] uppercase tracking-[0.04em] text-ground transition hover:opacity-90 disabled:opacity-40"
                  >
                    Gönder
                  </button>
                )}
              </form>
            </>
          )}
        </Panel>
      </section>
    </div>
  )
}
