import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ComplianceCard } from './ComplianceCard'
import { DialogProvider } from '../lib/dialog'

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

const renderCard = () => render(
  <DialogProvider>
    <ComplianceCard refreshKey={0} />
  </DialogProvider>,
)

describe('ComplianceCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur ve motor durumunu gösterir', async () => {
    mockFetch()
    renderCard()
    expect(screen.getByText('yükleniyor…')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('motor aktif')).toBeInTheDocument())
    expect(screen.getByText('12.345')).toBeInTheDocument()
    expect(screen.getByText(/saklama: 730 gün/)).toBeInTheDocument()
  })

  it('inceleme tutanağı yokken boş-durum mesajı gösterir', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText(/tutanak yok/)).toBeInTheDocument())
  })

  it('/status başarısız olunca hata mesajı gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: false, status: 500, json: async () => ({}) } as Response)))
    renderCard()
    await waitFor(() => expect(screen.getByText('durum alınamadı')).toBeInTheDocument())
  })

  it('tarih alanları aria-label ile erişilebilir', async () => {
    mockFetch()
    renderCard()
    await waitFor(() => expect(screen.getByText('motor aktif')).toBeInTheDocument())
    expect(screen.getByLabelText('Başlangıç tarihi')).toBeInTheDocument()
    expect(screen.getByLabelText('Bitiş tarihi')).toBeInTheDocument()
  })

  it('tutanak eklerken ikinci dialog iptal edilince POST atılmaz', async () => {
    mockFetch()
    const user = userEvent.setup()
    renderCard()
    await waitFor(() => expect(screen.getByText('motor aktif')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /log inceleme/ }))
    await user.type(screen.getByRole('textbox'), 'notlar')
    await user.click(screen.getByRole('button', { name: 'Tamam' })) // ilk dialog OK
    // ikinci dialog (bulgu) → İptal
    await user.click(await screen.findByRole('button', { name: 'İptal' }))

    const postCalls = (window.fetch as ReturnType<typeof vi.fn>).mock.calls.filter(
      (call) => (call[1] as RequestInit | undefined)?.method === 'POST',
    )
    expect(postCalls.length).toBe(0)
  })

  it("tutanak POST'u başarısız olunca hata mesajı gösterir", async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, opts?: RequestInit) => {
        if (opts?.method === 'POST') return Promise.resolve({ ok: false, status: 500, json: async () => ({}) } as Response)
        if (url === '/api/v1/compliance/status') return Promise.resolve({ ok: true, status: 200, json: async () => STATUS } as Response)
        return Promise.resolve({ ok: true, status: 200, json: async () => [] } as Response)
      }),
    )
    const user = userEvent.setup()
    renderCard()
    await waitFor(() => expect(screen.getByText('motor aktif')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /log inceleme/ }))
    await user.type(screen.getByRole('textbox'), 'notlar')
    await user.click(screen.getByRole('button', { name: 'Tamam' }))
    // ikinci dialog: bulgu boş → Tamam
    await user.click(await screen.findByRole('button', { name: 'Tamam' }))

    await waitFor(() => expect(screen.getByText(/tutanak kaydedilemedi/)).toBeInTheDocument())
  })
})
