import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GeoMapCard } from './GeoMapCard'

// Faz 4.5 smoke test — impeccable Faz 5'e (test kapsamı dışı bileşen
// düzeltmeleri) girmeden önce güvenlik ağı: render-without-crash + en az
// bir kritik veri noktasının DOM'da bulunduğu kontrolü.

const GEO = [{ country: 'TR', name: 'Türkiye', lat: 39, lon: 35.2, bytes: 123_456_789, sessions: 4 }]

function mockFetch(rows: unknown[] = GEO) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 200, ok: true, json: async () => rows } as Response)))
}

describe('GeoMapCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur ve ülke verisini gösterir', async () => {
    mockFetch()
    render(<GeoMapCard />)
    // "TR" hem SVG etiketinde hem alttaki özet şeridinde geçiyor — getAllByText
    await waitFor(() => expect(screen.getAllByText('TR').length).toBeGreaterThan(0))
    expect(screen.getByRole('group', { name: /dünya haritası/i })).toBeInTheDocument()
  })

  it('veri boşken "veri yok" durumuna düşer', async () => {
    mockFetch([])
    render(<GeoMapCard />)
    await waitFor(() => expect(screen.getByText(/Coğrafi veri yok/)).toBeInTheDocument())
  })

  it('zaman aralığı butonları render olur', async () => {
    mockFetch()
    render(<GeoMapCard />)
    await waitFor(() => expect(screen.getByRole('button', { name: '1 saat' })).toBeInTheDocument())
    expect(screen.getByRole('button', { name: '6 saat' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '24 saat' })).toBeInTheDocument()
  })

  it('ülke balonu klavye/ekran-okuyucu için erişilebilir isim taşır', async () => {
    mockFetch()
    render(<GeoMapCard />)
    const bubble = await screen.findByRole('button', { name: /Türkiye \(TR\), .* trafik, .* oturum/ })
    expect(bubble).toHaveAttribute('tabindex', '0')
  })

  it('fetch başarısız olunca satır-içi hata bildirimi gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 500, ok: false, json: async () => [] } as Response)))
    render(<GeoMapCard />)
    await waitFor(() => expect(screen.getByText(/veri alınamadı/)).toBeInTheDocument())
  })
})
