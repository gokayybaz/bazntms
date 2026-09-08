import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProcessDetailPage } from './ProcessDetailPage'

// Faz 23-A smoke — AgentDetailPage.test.tsx'teki Router+useParams kalıbı.

const DETAIL = {
  process: {
    process: 'claude',
    pids: [77598],
    first_seen: Math.floor(Date.now() / 1000) - 600,
    last_seen: Math.floor(Date.now() / 1000),
    bytes_in: 4000,
    bytes_out: 600,
    total: 4600,
    rx_bps: 120,
    tx_bps: 20,
  },
  remotes: [
    {
      remote_ip: '160.79.104.10',
      port: 443,
      proto: 'tcp',
      bytes_in: 4000,
      bytes_out: 600,
      first_seen: 0,
      last_seen: Math.floor(Date.now() / 1000),
      conns: 2,
      country: 'US',
      asn: 'AS399358',
    },
  ],
  connections: [{ proto: 'tcp', local_addr: '10.0.0.9:51000', remote_addr: '160.79.104.10:443', status: 'ESTABLISHED', pid: 77598, process: 'claude' }],
  app_visibility: [{ type: 'tls', host: 'api.anthropic.com', observations: 12, bytes: 4096, first_seen: 0, last_seen: Math.floor(Date.now() / 1000) }],
  timeline: [
    { ts: Math.floor(Date.now() / 1000) - 600, event: 'process.first_seen', target: 'claude' },
    { ts: Math.floor(Date.now() / 1000) - 300, event: 'tls.sni', target: 'api.anthropic.com' },
  ],
}

function mockFetch(body: unknown = DETAIL, status = 200) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (typeof url === 'string' && url.includes('/processes/')) {
        return Promise.resolve({ ok: status === 200, status, json: async () => body } as Response)
      }
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

function renderAt(path = '/agentlar/1/surec/claude') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/agentlar/:id/surec/:ad" element={<ProcessDetailPage />} />
        <Route path="/agentlar/:id" element={<div>agent sayfası</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProcessDetailPage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('süreç detayını render eder — özet, hedef (ASN ile), uygulama görünürlüğü, zaman çizelgesi', async () => {
    mockFetch()
    renderAt()
    await waitFor(() => expect(screen.getAllByText(/claude/).length).toBeGreaterThan(0))
    expect((await screen.findAllByText('api.anthropic.com')).length).toBeGreaterThan(0)
    expect(screen.getByText('AS399358')).toBeInTheDocument()
    expect(screen.getByText(/süreç ilk görüldü/)).toBeInTheDocument()
  })

  it('boş yanıtta yönlendirme mesajı gösterir', async () => {
    mockFetch({}, 404)
    renderAt()
    await waitFor(() => expect(screen.getByText(/seçili pencerede görülmedi/)).toBeInTheDocument())
  })
})
