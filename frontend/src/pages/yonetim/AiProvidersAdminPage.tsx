import { useCallback, useEffect, useState } from 'react'
import { AdminPageShell } from '../../components/AdminPageShell'
import { Panel } from '../../components/Panel'
import { PanelState } from '../../components/PanelState'
import { useDialog } from '../../lib/dialog'

// GET /api/v1/ai/providers yanıtı (yerel tip — CLAUDE.md).
interface Provider {
  id: number
  name: string
  kind: string
  base_url: string
  default_model: string
  enabled: boolean
  has_key: boolean
  is_local: boolean
  opts_json?: string
}

const KINDS = ['ollama', 'lmstudio', 'openai', 'anthropic', 'openai-compat'] as const
const KIND_LABEL: Record<string, string> = {
  ollama: 'Ollama (yerel)',
  lmstudio: 'LM Studio (yerel)',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  'openai-compat': 'OpenAI-uyumlu (vLLM/OpenRouter/…)',
}

export function AiProvidersAdminPage() {
  const { form, confirm } = useDialog()
  const [provs, setProvs] = useState<Provider[]>([])
  const [loaded, setLoaded] = useState(false)
  const [err, setErr] = useState('')
  const [testResult, setTestResult] = useState<Record<number, string>>({})

  const load = useCallback(async () => {
    try {
      const r = await fetch('/api/v1/ai/providers')
      if (r.status === 403) {
        setErr('AI sağlayıcı yönetimi yalnız global yöneticide')
        setLoaded(true)
        return
      }
      if (r.status === 503) {
        setErr('AI analiz kapalı — hub’ı -ai bayrağıyla başlatın')
        setLoaded(true)
        return
      }
      if (r.ok) {
        setProvs(await r.json())
        setErr('')
      }
    } catch {
      setErr('sağlayıcı listesi alınamadı')
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const edit = async (p?: Provider) => {
    let opts: { no_think?: boolean; max_tokens?: number; temperature?: number } = {}
    try {
      opts = p?.opts_json ? JSON.parse(p.opts_json) : {}
    } catch {
      /* yoksay */
    }
    const res = await form({
      title: p ? `Sağlayıcı — ${p.name}` : 'AI sağlayıcı ekle',
      fields: [
        { key: 'name', label: 'Ad', type: 'text', defaultValue: p?.name ?? '' },
        { key: 'kind', label: 'Tür', type: 'select', options: [...KINDS], defaultValue: p?.kind ?? 'ollama' },
        {
          key: 'base_url',
          label: 'Taban adres (boş = tür varsayılanı)',
          type: 'text',
          defaultValue: p?.base_url ?? '',
          placeholder: 'http://localhost:11434/v1',
        },
        {
          key: 'api_key',
          label: p?.has_key ? 'API anahtarı (boş = değiştirme)' : 'API anahtarı (yerel modelde gerekmez)',
          type: 'password',
          defaultValue: '',
        },
        { key: 'default_model', label: 'Varsayılan model', type: 'text', defaultValue: p?.default_model ?? '' },
        { key: 'enabled', label: 'Etkin (1/0)', type: 'text', defaultValue: p ? (p.enabled ? '1' : '0') : '1' },
        { key: 'no_think', label: 'no_think — Qwen3 düşünmeyi kapat (1/0)', type: 'text', defaultValue: opts.no_think ? '1' : '0' },
        { key: 'max_tokens', label: 'max_tokens (0 = varsayılan)', type: 'number', defaultValue: String(opts.max_tokens ?? 0) },
      ],
      confirmLabel: 'Kaydet',
    })
    if (!res) return
    const body = {
      name: res.name,
      kind: res.kind,
      base_url: res.base_url,
      api_key: res.api_key,
      default_model: res.default_model,
      enabled: res.enabled === '1',
      no_think: res.no_think === '1',
      max_tokens: Number(res.max_tokens) || 0,
    }
    const r = await fetch(p ? `/api/v1/ai/providers/${p.id}` : '/api/v1/ai/providers', {
      method: p ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!r.ok) {
      setErr((await r.text()) || 'kaydedilemedi')
      return
    }
    setErr('')
    await load()
  }

  const del = async (p: Provider) => {
    if (!(await confirm(`“${p.name}” sağlayıcısı silinsin mi?`, { danger: true }))) return
    await fetch(`/api/v1/ai/providers/${p.id}`, { method: 'DELETE' })
    await load()
  }

  const test = async (p: Provider) => {
    setTestResult((s) => ({ ...s, [p.id]: 'test ediliyor…' }))
    try {
      const r = await fetch(`/api/v1/ai/providers/${p.id}/test`, { method: 'POST' })
      const d = await r.json()
      if (d.ok) {
        const models = Array.isArray(d.models) && d.models.length ? ` · ${d.models.length} model` : ''
        setTestResult((s) => ({ ...s, [p.id]: `✓ ${d.latency_ms} ms${models}` }))
      } else {
        setTestResult((s) => ({ ...s, [p.id]: `✗ ${d.error ?? 'başarısız'}` }))
      }
    } catch {
      setTestResult((s) => ({ ...s, [p.id]: '✗ ulaşılamadı' }))
    }
  }

  return (
    <AdminPageShell
      title="AI Sağlayıcı"
      hint="Yerel (Ollama/LM Studio) veya bulut (OpenAI/Anthropic/OpenAI-uyumlu) model sağlayıcıları. API anahtarları vault ile şifreli saklanır, panelde bir daha gösterilmez. Egress kilidi açıksa (-ai-allow-cloud=false) yalnız yerel adresler kabul edilir."
    >
      <Panel
        title="Sağlayıcılar"
        right={
          <button
            onClick={() => edit()}
            className="border border-rx px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-rx hover:bg-rx/10"
          >
            + Ekle
          </button>
        }
      >
        {!loaded ? (
          <PanelState kind="loading" />
        ) : err ? (
          <PanelState kind="error" message={err} onRetry={load} />
        ) : provs.length === 0 ? (
          <PanelState kind="empty" message="Henüz sağlayıcı yok." hint="Yerel bir model için Ollama başlatın, + Ekle deyin." />
        ) : (
          <div className="space-y-1 font-mono text-[11px]">
            {provs.map((p) => (
              <div key={p.id} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-b border-rule py-1.5 last:border-0">
                <span className="min-w-[8rem] text-ink-hi">{p.name}</span>
                <span className="text-tui-dim">{KIND_LABEL[p.kind] ?? p.kind}</span>
                <span className="text-tui-dim">{p.default_model || '—'}</span>
                <span className={p.is_local ? 'text-emerald-400' : 'text-amber-400'}>
                  {p.is_local ? 'yerel' : 'bulut'}
                </span>
                <span className={p.has_key ? 'text-emerald-400' : 'text-tui-dim'}>{p.has_key ? 'anahtar ✓' : 'anahtarsız'}</span>
                <span className={p.enabled ? 'text-emerald-400' : 'text-rose-400'}>{p.enabled ? 'etkin' : 'kapalı'}</span>
                {testResult[p.id] && <span className="text-rx">{testResult[p.id]}</span>}
                <span className="ml-auto flex gap-2">
                  <button onClick={() => test(p)} className="text-tui-dim hover:text-rx">
                    Test Et
                  </button>
                  <button onClick={() => edit(p)} className="text-tui-dim hover:text-ink-hi">
                    Düzenle
                  </button>
                  <button onClick={() => del(p)} className="text-tui-dim hover:text-rose-400">
                    Sil
                  </button>
                </span>
              </div>
            ))}
          </div>
        )}
      </Panel>
    </AdminPageShell>
  )
}
