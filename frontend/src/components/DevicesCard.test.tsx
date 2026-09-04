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

  it('"+ Cihaz Ekle" formu açıp kapatır, alanlar gerçek <label> ile eşleşiyor', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText('core-switch')).toBeInTheDocument())
    screen.getByRole('button', { name: '+ Cihaz Ekle' }).click()
    // placeholder değil, gerçek etiket üzerinden bulunabiliyor olmalı (a11y fix)
    expect(await screen.findByLabelText('Ad *')).toBeInTheDocument()
    expect(screen.getByLabelText('Host / IP *')).toBeInTheDocument()
  })

  it('silme iki-aşamalı onay ister — ilk tık onay durumuna geçer, ikinci tık siler', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText('core-switch')).toBeInTheDocument())
    const del = screen.getByRole('button', { name: /core-switch cihazını sil/ })
    del.click()
    const confirmBtn = await screen.findByRole('button', { name: /core-switch silinsin mi/ })
    expect(confirmBtn).toHaveTextContent('emin misiniz?')
    confirmBtn.click()
    await waitFor(() => expect(screen.getByText(/core-switch silindi/)).toBeInTheDocument())
  })
})
