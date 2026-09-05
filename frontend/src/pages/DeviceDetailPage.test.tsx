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

  it('hiç poll edilmemiş cihaz "sorunlu" değil "ilk poll bekleniyor" gösterir', async () => {
    mockFetch([{ ...DEVICE, last_poll: 0 }])
    renderPage()
    await waitFor(() => expect(screen.getByText('ilk poll bekleniyor')).toBeInTheDocument())
    expect(screen.queryByText('sorunlu')).not.toBeInTheDocument()
  })

  it('devre dışı bırakılmış cihaz "sorunlu" gösterir', async () => {
    mockFetch([{ ...DEVICE, enabled: false }])
    renderPage()
    await waitFor(() => expect(screen.getByText('sorunlu')).toBeInTheDocument())
  })

  it('401 gelince oturum bildirimi gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 401, json: async () => ({}) } as Response)))
    renderPage()
    await waitFor(() => expect(screen.getByText(/Oturum sona ermiş olabilir/)).toBeInTheDocument())
  })

  it('syslog önem-seviyesi rozeti severity\'e göre renklendiriliyor (paylaşılan SEV_STYLES)', async () => {
    const now = Math.floor(Date.now() / 1000)
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) => {
        if (url === '/api/v1/devices') return Promise.resolve({ status: 200, json: async () => [DEVICE] } as Response)
        if (url.endsWith('/interfaces')) return Promise.resolve({ status: 200, json: async () => [] } as Response)
        if (url.startsWith('/api/v1/flows')) return Promise.resolve({ status: 200, json: async () => [] } as Response)
        if (url.startsWith('/api/v1/syslog')) {
          return Promise.resolve({
            status: 200,
            json: async () => [{ id: 1, ts: now, host: '10.0.0.1', source_ip: '10.0.0.1', severity: 0, tag: 'kernel', message: 'acil durum' }],
          } as Response)
        }
        return Promise.resolve({ status: 404, json: async () => ({}) } as Response)
      }),
    )
    renderPage()
    const badge = await screen.findByText('emergency')
    expect(badge.className).toContain('rose')
  })
})
