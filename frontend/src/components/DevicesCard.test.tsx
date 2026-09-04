import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DevicesCard } from './DevicesCard'

// Faz 4.5 smoke test (bkz. GeoMapCard.test.tsx üstteki not).
// DevicesCard cihaz adını react-router <Link> ile render ediyor → Router
// context gerekli.

const DEVICES = [
  {
    id: 1,
    name: 'core-switch',
    host: '10.0.0.1',
    kind: 'switch',
    site: 'ofis-a',
    vendor: 'snmp',
    snmp_version: 2,
    api_url: '',
    vdom: '',
    poll_seconds: 60,
    enabled: true,
    sys_name: '',
    sys_descr: '',
    last_poll: Math.floor(Date.now() / 1000),
    last_error: '',
  },
]

function mockFetch(devices: unknown[] = DEVICES) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/devices') return Promise.resolve({ status: 200, json: async () => devices } as Response)
      return Promise.resolve({ status: 404, json: async () => ({}) } as Response)
    }),
  )
}

function renderCard() {
  return render(
    <MemoryRouter>
      <DevicesCard refreshKey={0} />
    </MemoryRouter>,
  )
}

describe('DevicesCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur ve cihaz listesini gösterir', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText('core-switch')).toBeInTheDocument())
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
    expect(screen.getByText('snmp v2c')).toBeInTheDocument()
  })

  it('cihaz listesi boşken boş-durum mesajı gösterir', async () => {
    mockFetch([])
    renderCard()
    await waitFor(() => expect(screen.getByText(/Cihaz yok/)).toBeInTheDocument())
  })

  it('"+ Cihaz Ekle" formu açıp kapatır', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText('core-switch')).toBeInTheDocument())
    screen.getByRole('button', { name: '+ Cihaz Ekle' }).click()
    expect(await screen.findByPlaceholderText('ad * (core-sw)')).toBeInTheDocument()
  })
})
