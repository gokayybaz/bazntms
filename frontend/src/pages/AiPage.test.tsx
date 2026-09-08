import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AiPage } from './AiPage'
import { KeymapProvider } from '../lib/KeymapContext'

function renderAt(path: string) {
  return render(
    <KeymapProvider>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/ai" element={<AiPage />} />
        </Routes>
      </MemoryRouter>
    </KeymapProvider>,
  )
}

const CONVS = [
  { id: 1, title: 'Filo özeti', created_by: 'a', site: '', scope_kind: 'fleet', scope_ref: '', provider_id: 1, model: 'm', source: 'user', created_ts: 1000, updated_ts: 2000, archived: false },
]

function sseStream(chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream({
    start(ctrl) {
      for (const c of chunks) ctrl.enqueue(enc.encode(`data: ${c}\n\n`))
      ctrl.close()
    },
  })
}

afterEach(() => vi.unstubAllGlobals())

describe('AiPage', () => {
  it('AI kapalıyken bilgilendirir', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      if (url === '/api/v1/ai/status') return Promise.resolve({ ok: true, json: async () => ({ enabled: false }) } as Response)
      return Promise.resolve({ ok: true, json: async () => ({ presets: [] }) } as Response)
    }))
    renderAt('/ai')
    await waitFor(() => expect(screen.getByText(/AI analiz kapalı/i)).toBeInTheDocument())
  })

  it('mesajsız (yeni) konuşmada çökmez — messages: null', async () => {
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      if (url === '/api/v1/ai/status')
        return Promise.resolve({ ok: true, json: async () => ({ enabled: true, has_ready_provider: true }) } as Response)
      if (url === '/api/v1/ai/presets')
        return Promise.resolve({ ok: true, json: async () => ({ presets: [] }) } as Response)
      if (url === '/api/v1/ai/conversations')
        return Promise.resolve({ ok: true, json: async () => CONVS } as Response)
      if (url === '/api/v1/ai/conversations/1')
        // backend eski davranışı: mesajsız konuşmada messages null dönebilir
        return Promise.resolve({ ok: true, json: async () => ({ conversation: CONVS[0], messages: null }) } as Response)
      return Promise.resolve({ ok: false, status: 404, text: async () => '', json: async () => ({}) } as Response)
    }))
    renderAt('/ai?c=1')
    // composer görünür → sayfa çökmedi
    await waitFor(() => expect(screen.getByPlaceholderText(/Bir şey sor/i)).toBeInTheDocument())
  })

  it('sohbet listesini gösterir ve mesaj akışını render eder', async () => {
    let sent = false
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      if (url === '/api/v1/ai/status')
        return Promise.resolve({ ok: true, json: async () => ({ enabled: true, default_model: 'test-model', has_ready_provider: true }) } as Response)
      if (url === '/api/v1/ai/presets')
        return Promise.resolve({ ok: true, json: async () => ({ presets: [{ id: 'fleet_summary', label: 'Filoyu özetle', scopes: ['fleet'], task: 't' }] }) } as Response)
      if (url === '/api/v1/ai/conversations')
        return Promise.resolve({ ok: true, json: async () => CONVS } as Response)
      if (url === '/api/v1/ai/conversations/1/messages') {
        sent = true
        return Promise.resolve({
          ok: true,
          body: sseStream([
            '{"delta":"Filo "}',
            '{"delta":"**sağlıklı**."}',
            '{"done":true,"tokens_in":10,"tokens_out":3}',
          ]),
        } as Response)
      }
      if (url === '/api/v1/ai/conversations/1')
        return Promise.resolve({
          ok: true,
          json: async () => ({
            conversation: CONVS[0],
            messages: sent
              ? [
                  { id: 1, role: 'user', content: 't', tokens_in: 0, tokens_out: 0, created_ts: 1 },
                  { id: 2, role: 'assistant', content: 'Filo **sağlıklı**.', tokens_in: 10, tokens_out: 3, created_ts: 2 },
                ]
              : [],
          }),
        } as Response)
      return Promise.resolve({ ok: false, status: 404, text: async () => '', json: async () => ({}) } as Response)
    }))

    renderAt('/ai?c=1')
    // konuşma listesi
    await waitFor(() => expect(screen.getAllByText('Filo özeti').length).toBeGreaterThan(0))
    // preset butonu
    const preset = await screen.findByRole('button', { name: 'Filoyu özetle' })
    await userEvent.click(preset)
    // akış markdown olarak render edilir (kalın)
    await waitFor(() => expect(screen.getByText('sağlıklı')).toBeInTheDocument())
  })
})
