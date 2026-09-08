// AI analiz — paylaşılan tipler + yardımcılar (Faz 26). Sayfalar kendi API
// tiplerini yerelde tanımlar (CLAUDE.md); burada yalnız gerçekten çok yerde
// kullanılanlar + askAI navigasyon yardımcısı.

export type AiScope = 'fleet' | 'agent' | 'incident' | 'anomaly' | 'device'

export interface AiConversation {
  id: number
  title: string
  created_by: string
  site: string
  scope_kind: AiScope
  scope_ref: string
  provider_id: number
  model: string
  source: 'user' | 'nightly' | 'triage'
  created_ts: number
  updated_ts: number
  archived: boolean
}

export interface AiMessage {
  id: number
  role: 'system' | 'user' | 'assistant'
  content: string
  context_json?: string
  tokens_in: number
  tokens_out: number
  error?: string
  created_ts: number
}

export interface AiStatus {
  enabled: boolean
  providers?: { id: number; name: string; kind: string; enabled: boolean; default_model: string }[]
  default_provider?: number
  default_model?: string
  has_ready_provider?: boolean
}

export interface AiPreset {
  id: string
  label: string
  scopes: string[] | null
  task: string
}

// createConversation, yeni bir sohbet oturumu açar ve id döndürür.
export async function createConversation(scope: AiScope, ref = '', providerID = 0): Promise<number> {
  const res = await fetch('/api/v1/ai/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ scope_kind: scope, scope_ref: ref, provider_id: providerID }),
  })
  if (!res.ok) throw new Error(`konuşma oluşturulamadı (${res.status})`)
  const d = await res.json()
  return d.id as number
}

// askAI, bir varlık bağlamında yeni sohbet açıp /ai sayfasına gider. Detay
// sayfalarındaki "AI'ya Sor" düğmeleri bunu çağırır.
export async function askAI(
  navigate: (to: string) => void,
  scope: AiScope,
  ref = '',
  preset?: string,
): Promise<void> {
  const id = await createConversation(scope, ref)
  const q = new URLSearchParams({ c: String(id) })
  if (preset) q.set('preset', preset)
  navigate(`/ai?${q}`)
}

// streamMessage, POST .../messages SSE akışını okur. onDelta her parçada,
// onDone bitişte, onError hatada çağrılır. AbortController ile iptal edilir.
export function streamMessage(
  convID: number,
  body: { content?: string; preset?: string; refresh_context?: boolean },
  cb: {
    onDelta: (t: string) => void
    onDone: (u: { tokens_in?: number; tokens_out?: number }) => void
    onError: (e: string) => void
  },
): AbortController {
  const ac = new AbortController()
  ;(async () => {
    try {
      const res = await fetch(`/api/v1/ai/conversations/${convID}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: ac.signal,
      })
      if (!res.ok || !res.body) {
        const txt = await res.text().catch(() => '')
        cb.onError(txt || `HTTP ${res.status}`)
        return
      }
      const reader = res.body.getReader()
      const dec = new TextDecoder()
      let buf = ''
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const parts = buf.split('\n\n')
        buf = parts.pop() ?? ''
        for (const p of parts) {
          const line = p.trim()
          if (!line.startsWith('data:')) continue
          let ev: Record<string, unknown>
          try {
            ev = JSON.parse(line.slice(5).trim())
          } catch {
            continue
          }
          if (typeof ev.delta === 'string') cb.onDelta(ev.delta)
          else if (ev.error) cb.onError(String(ev.error))
          else if (ev.done) cb.onDone({ tokens_in: Number(ev.tokens_in), tokens_out: Number(ev.tokens_out) })
        }
      }
    } catch (e) {
      if (!ac.signal.aborted) cb.onError(e instanceof Error ? e.message : String(e))
    }
  })()
  return ac
}
