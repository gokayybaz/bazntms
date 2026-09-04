import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DeviceDetailPage } from './DeviceDetailPage'

// Faz 4.5 smoke test (bkz. GeoMapCard.test.tsx üstteki not).
// AgentDetailPage.test.tsx'teki Router+useParams kalıbını taklit eder.

const DEVICE = {
  id: 1,
  name: 'core-switch',
  host: '10.0.0.1',
  kind: 'switch',
  vendor: 'snmp',
  snmp_version: 2,
  poll_seconds: 60,
  enabled: true,
  sys_name: 'core-switch.local',
  sys_descr: '',
  api_url: '',
  api_verify_tls: true,
  vdom: '',
  added_at: Math.floor(Date.now() / 1000),
  last_poll: Math.floor(Date.now() / 1000),
  last_error: '',
}

function mockFetch(devices: unknown[] = [DEVICE]) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/devices') return Promise.resolve({ status: 200, json: async () => devices } as Response)
      if (url.endsWith('/interfaces')) return Promise.resolve({ status: 200, json: async () => [] } as Response)
      if (url.startsWith('/api/v1/flows')) return Promise.resolve({ status: 200, json: async () => [] } as Response)
      if (url.startsWith('/api/v1/syslog')) return Promise.resolve({ status: 200, json: async () => [] } as Response)
      return Promise.resolve({ status: 404, json: async () => ({}) } as Response)
    }),
  )
}

function renderPage(id = '1') {
  return render(
    <MemoryRouter initialEntries={[`/cihazlar/${id}`]}>
      <Routes>
        <Route path="/cihazlar/:id" element={<DeviceDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('DeviceDetailPage', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur ve cihaz bilgisini gösterir', async () => {
    mockFetch()
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'core-switch' })).toBeInTheDocument())
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
    expect(screen.getByText('sağlıklı')).toBeInTheDocument()
  })

  it('listede olmayan id için "cihaz bulunamadı" gösterir', async () => {
    mockFetch()
    renderPage('999')
    await waitFor(() => expect(screen.getByText(/Cihaz bulunamadı/)).toBeInTheDocument())
  })

  it('arayüz/akış/syslog boşken üç bölümde de boş-durum mesajı gösterir', async () => {
    mockFetch()
    renderPage()
    await waitFor(() => expect(screen.getByText(/Henüz arayüz verisi yok/)).toBeInTheDocument())
    expect(screen.getByText(/Bu cihazdan akış yok/)).toBeInTheDocument()
    expect(screen.getByText(/Bu cihazdan syslog olayı yok/)).toBeInTheDocument()
  })
})
