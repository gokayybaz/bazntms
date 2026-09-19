import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { IncidentDetailPage } from './IncidentDetailPage'
import { DialogProvider } from '../lib/dialog'

const DETAIL = {
  incident: {
    id: 42,
    title: 'Şüpheli çıkış aktivitesi (IOC)',
    severity: 'crit',
    status: 'open',
    site: '',
    agent_id: 3,
    correlation_key: 'r1|agent3',
    correlation_reason: 'yeni süreç + tehdit istihbaratı eşleşmesi',
    summary: 'proc → ioc',
    risk_score: 85,
    first_seen: Math.floor(Date.now() / 1000) - 200,
    last_seen: Math.floor(Date.now() / 1000) - 30,
    created_ts: Math.floor(Date.now() / 1000) - 200,
    updated_ts: Math.floor(Date.now() / 1000) - 30,
  },
  evidence: [
    { kind: 'alert', ref: '1', ts: Math.floor(Date.now() / 1000) - 200, summary: '[proc/info] yeni süreç' },
    { kind: 'alert', ref: '2', ts: Math.floor(Date.now() / 1000) - 30, summary: '[ioc/crit] tehdit eşleşmesi' },
  ],
}

function mockFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (url.match(/\/incidents\/42$/) && (!init || !init.method)) {
        return Promise.resolve({ ok: true, status: 200, json: async () => DETAIL } as Response)
      }
      if (url.includes('/incidents/42/') && init?.method === 'POST') {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ ok: true, status: 'investigating' }) } as Response)
      }
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

function renderAt(path = '/uyarilar/olay/42') {
  return render(
    <DialogProvider>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/uyarilar/olay/:id" element={<IncidentDetailPage />} />
          <Route path="/uyarilar" element={<div>uyarılar sayfası</div>} />
          <Route path="/agentlar/:id" element={<div>agent</div>} />
        </Routes>
      </MemoryRouter>
    </DialogProvider>,
  )
}

describe('IncidentDetailPage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('özet + korelasyon + kanıt zaman çizelgesini render eder', async () => {
    mockFetch()
    renderAt()
    expect(await screen.findByText('Şüpheli çıkış aktivitesi (IOC)')).toBeInTheDocument()
    expect(screen.getByText('85 / 100')).toBeInTheDocument()
    expect(screen.getByText(/yeni süreç \+ tehdit istihbaratı/)).toBeInTheDocument()
    expect(screen.getByText('[ioc/crit] tehdit eşleşmesi')).toBeInTheDocument()
    // r1|agent3 dedup anahtarı
    expect(screen.getByText('r1|agent3')).toBeInTheDocument()
  })

  it('İncele eylemi POST atar', async () => {
    mockFetch()
    renderAt()
    await screen.findByText('Şüpheli çıkış aktivitesi (IOC)')
    await userEvent.click(screen.getByRole('button', { name: 'İncele' }))
    await waitFor(() =>
      expect((globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls.some((c) => String(c[0]).includes('/incidents/42/investigate'))).toBe(true),
    )
  })

  it('bulunamayan olay için mesaj gösterir', async () => {
    mockFetch()
    renderAt('/uyarilar/olay/999')
    await waitFor(() => expect(screen.getByText(/Olay bulunamadı/)).toBeInTheDocument())
  })
})
