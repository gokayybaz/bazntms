import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AgentDetailPage } from './AgentDetailPage'

const navigateMock = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => navigateMock }
})

const AGENT = {
  id: 1,
  name: 'sunucu-01',
  site: 'ofis-a',
  first_seen: 0,
  last_seen: Math.floor(Date.now() / 1000),
  version: 'v0.2.8',
  protocol_version: 1,
  remote_ip: '10.0.0.5',
  online: true,
  rates: [],
  conns: 2,
}
const CONNECTIONS = [
  { proto: 'tcp', local_addr: '10.0.0.5:5000', remote_addr: '1.2.3.4:443', status: 'ESTABLISHED', pid: 100, process: 'chrome' },
  { proto: 'udp', local_addr: '10.0.0.5:5353', remote_addr: '8.8.8.8:53', status: '', pid: 200, process: 'mDNSResponder' },
]

function mockFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (typeof url === 'string' && url.startsWith('/api/v1/agents/1/history')) {
        return Promise.resolve({ ok: true, status: 200, json: async () => [] } as Response)
      }
      if (typeof url === 'string' && url === '/api/v1/agents/1' && (!init || init.method === undefined)) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({ agent: AGENT, connections: CONNECTIONS }),
        } as Response)
      }
      if (typeof url === 'string' && url === '/api/v1/agents/1' && init?.method === 'PATCH') {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ ok: true }) } as Response)
      }
      if (typeof url === 'string' && url === '/api/v1/agents/1' && init?.method === 'DELETE') {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ ok: true }) } as Response)
      }
      if (typeof url === 'string' && url.startsWith('/api/v1/processes')) {
        return Promise.resolve({ ok: true, status: 200, json: async () => [] } as Response)
      }
      return Promise.resolve({ status: 404, json: async () => ({}) } as Response)
    }),
  )
}

function renderPage(id = '1') {
  return render(
    <MemoryRouter initialEntries={[`/agentlar/${id}`]}>
      <Routes>
        <Route path="/agentlar/:id" element={<AgentDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('AgentDetailPage', () => {
  beforeEach(() => {
    mockFetch()
    navigateMock.mockClear()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('yüklenirken bekleme mesajı gösterir', () => {
    vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
    renderPage()
    expect(screen.getByText('Yükleniyor…')).toBeInTheDocument()
  })

  it('404 durumunda "agent bulunamadı" gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 404, json: async () => ({}) } as Response)))
    renderPage()
    await waitFor(() => expect(screen.getByText(/Agent bulunamadı/)).toBeInTheDocument())
  })

  it('agent bilgisini ve bağlantı tablosunu render eder', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument())
    expect(screen.getByText('ofis-a')).toBeInTheDocument()
    expect(screen.getByText('10.0.0.5')).toBeInTheDocument()
    expect(screen.getByText('chrome')).toBeInTheDocument()
    expect(screen.getByText('mDNSResponder')).toBeInTheDocument()
  })

  it('bağlantı arama kutusu filtreler', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByText('chrome')).toBeInTheDocument())

    const input = screen.getByPlaceholderText('Filtrele: adres, süreç, durum…')
    await user.type(input, 'mDNS')

    expect(screen.queryByText('chrome')).not.toBeInTheDocument()
    expect(screen.getByText('mDNSResponder')).toBeInTheDocument()
  })

  it('"Adı Değiştir" satır-içi alanı açar, Kaydet yeni ismi PATCH ile gönderir', async () => {
    // eskiden window.prompt() kullanıyordu — artık DevicesCard'daki gibi
    // satır-içi <input> + Kaydet/Vazgeç (impeccable critique 2026-09-05)
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Adı Değiştir' }))
    const input = await screen.findByLabelText('Yeni agent adı')
    expect(input).toHaveValue('sunucu-01')
    await user.clear(input)
    await user.type(input, 'yeni-ad')
    await user.click(screen.getByRole('button', { name: 'Kaydet' }))

    await waitFor(() => expect(screen.getByRole('heading', { name: 'yeni-ad' })).toBeInTheDocument())
    const patchCall = vi.mocked(fetch).mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === 'PATCH')
    expect(patchCall).toBeDefined()
    expect(JSON.parse((patchCall![1] as RequestInit).body as string)).toEqual({ name: 'yeni-ad' })
  })

  it('"Adı Değiştir" Vazgeç ile PATCH göndermeden eski isme döner', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Adı Değiştir' }))
    const input = await screen.findByLabelText('Yeni agent adı')
    await user.clear(input)
    await user.type(input, 'baska-ad')
    await user.click(screen.getByRole('button', { name: 'Vazgeç' }))

    expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument()
    const patchCall = vi.mocked(fetch).mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === 'PATCH')
    expect(patchCall).toBeUndefined()
  })

  it('silme iki-aşamalı onay ister — ilk tık onay durumuna geçer, ikinci tık DELETE gönderip listeye döner', async () => {
    // eskiden window.confirm() kullanıyordu — artık DevicesCard'daki iki-aşamalı
    // satır-içi buton deseni (impeccable critique 2026-09-05)
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /sunucu-01 agent'ını sil/ }))
    const confirmBtn = await screen.findByRole('button', { name: /sunucu-01 silinsin mi/ })
    expect(confirmBtn).toHaveTextContent('emin misiniz?')
    await user.click(confirmBtn)

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith('/agentlar'))
    const deleteCall = vi.mocked(fetch).mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === 'DELETE')
    expect(deleteCall).toBeDefined()
  })

  it('silme onay durumunda ikinci tık gelmezse istek gönderilmez', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'sunucu-01' })).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: /sunucu-01 agent'ını sil/ }))
    await screen.findByRole('button', { name: /sunucu-01 silinsin mi/ })

    expect(navigateMock).not.toHaveBeenCalled()
    const deleteCall = vi.mocked(fetch).mock.calls.find(([, init]) => (init as RequestInit | undefined)?.method === 'DELETE')
    expect(deleteCall).toBeUndefined()
  })

  it('bağlantı başlığı filtrelenmiş/toplam sayıyı gösterir', async () => {
    const user = userEvent.setup()
    renderPage()
    await waitFor(() => expect(screen.getByText('chrome')).toBeInTheDocument())
    expect(screen.getByText(/2 bağlantı/)).toBeInTheDocument()

    await user.type(screen.getByPlaceholderText('Filtrele: adres, süreç, durum…'), 'mDNS')
    expect(screen.getByText(/1 \/ 2 bağlantı/)).toBeInTheDocument()
  })
})
