import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FortiPanel } from './FortiPanel'

// Faz 4.5 smoke test (bkz. GeoMapCard.test.tsx üstteki not).

const RESOURCES = [{ ts: Math.floor(Date.now() / 1000), cpu_pct: 42, mem_pct: 60, disk_pct: 20, sessions: 1500 }]
const VPN = [{ vdom: 'root', kind: 'ipsec', name: 'sube-a', peer: '203.0.113.5', status: 'up', uptime: 3600, rx_bytes: 1000, tx_bytes: 2000, ts: Math.floor(Date.now() / 1000) }]

function mockFetch({ resources = RESOURCES, vpn = VPN, sdwan = [], policies = [] }: { resources?: unknown[]; vpn?: unknown[]; sdwan?: unknown[]; policies?: unknown[] } = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url.includes('/resources')) return Promise.resolve({ ok: true, status: 200, json: async () => resources } as Response)
      if (url.includes('/vpn')) return Promise.resolve({ ok: true, status: 200, json: async () => vpn } as Response)
      if (url.includes('/sdwan')) return Promise.resolve({ ok: true, status: 200, json: async () => sdwan } as Response)
      if (url.includes('/policies')) return Promise.resolve({ ok: true, status: 200, json: async () => policies } as Response)
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

describe('FortiPanel', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur, kaynak gauge\'larını ve VPN tablosunu gösterir', async () => {
    mockFetch()
    render(<FortiPanel deviceId={1} />)
    await waitFor(() => expect(screen.getByText('cpu')).toBeInTheDocument())
    expect(screen.getByText('42%')).toBeInTheDocument()
    expect(screen.getByText('1.500')).toBeInTheDocument()
    expect(screen.getByText('sube-a')).toBeInTheDocument()
  })

  it('hiç veri yokken bekleme mesajı gösterir', async () => {
    mockFetch({ resources: [], vpn: [], sdwan: [], policies: [] })
    render(<FortiPanel deviceId={1} />)
    await waitFor(() => expect(screen.getByText(/FortiGate verisi henüz yok/)).toBeInTheDocument())
  })

  it('/resources başarısız olunca hata mesajı gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: false, status: 500, json: async () => ({}) } as Response)))
    render(<FortiPanel deviceId={1} />)
    await waitFor(() => expect(screen.getByText('veri alınamadı')).toBeInTheDocument())
  })
})
