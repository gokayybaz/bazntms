import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ComplianceCard } from './ComplianceCard'

// Faz 4.5 smoke test (bkz. GeoMapCard.test.tsx üstteki not).

const STATUS = {
  config: { enabled: true, tsa_url: 'https://tsa.example', sign_key: true, worm_dir: '/data/worm', mask_pii: true, retention_days: 730 },
  records: 12_345,
  last_record_ts: Math.floor(Date.now() / 1000),
}

function mockFetch(status: unknown = STATUS, reviews: unknown[] = []) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/compliance/status') return Promise.resolve({ ok: true, status: 200, json: async () => status } as Response)
      if (url.startsWith('/api/v1/compliance/reviews')) return Promise.resolve({ ok: true, status: 200, json: async () => reviews } as Response)
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

describe('ComplianceCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur ve motor durumunu gösterir', async () => {
    mockFetch()
    render(<ComplianceCard refreshKey={0} />)
    expect(screen.getByText('yükleniyor…')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('motor aktif')).toBeInTheDocument())
    expect(screen.getByText('12.345')).toBeInTheDocument()
    expect(screen.getByText(/saklama: 730 gün/)).toBeInTheDocument()
  })

  it('inceleme tutanağı yokken boş-durum mesajı gösterir', async () => {
    mockFetch()
    render(<ComplianceCard refreshKey={0} />)
    await waitFor(() => expect(screen.getByText(/tutanak yok/)).toBeInTheDocument())
  })

  it('/status başarısız olunca hata mesajı gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: false, status: 500, json: async () => ({}) } as Response)))
    render(<ComplianceCard refreshKey={0} />)
    await waitFor(() => expect(screen.getByText('durum alınamadı')).toBeInTheDocument())
  })
})
