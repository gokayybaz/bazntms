import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AgentsListPage } from './AgentsListPage'

describe('AgentsListPage', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('yüklenirken bekleme mesajı gösterir', () => {
    vi.mocked(fetch).mockReturnValue(new Promise(() => {})) // hic cozulmez
    render(
      <MemoryRouter>
        <AgentsListPage />
      </MemoryRouter>,
    )
    // agent tablosu + alt taraftaki ProcessesCard ikisi de "Yükleniyor…" gösterir
    expect(screen.getAllByText('Yükleniyor…').length).toBeGreaterThan(0)
  })

  it('boş filo için "eşleşen agent yok" gösterir', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [],
    } as Response)

    render(
      <MemoryRouter>
        <AgentsListPage />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Eşleşen agent yok.')).toBeInTheDocument())
  })

  it('gelen agent listesini tabloda render eder', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [
        {
          id: 1,
          name: 'sunucu-01',
          site: 'ofis-a',
          first_seen: 0,
          last_seen: Math.floor(Date.now() / 1000),
          version: 'v0.2.4',
          protocol_version: 1,
          remote_ip: '10.0.0.5',
          online: true,
          rates: [],
          conns: 42,
        },
      ],
    } as Response)

    render(
      <MemoryRouter>
        <AgentsListPage />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('sunucu-01')).toBeInTheDocument())
    expect(screen.getByRole('link', { name: 'sunucu-01' })).toHaveAttribute('href', '/agentlar/1')
  })

  it('poll başarısız olursa donukluk uyarısı gösterir — eskiden sessizce yutuluyordu', async () => {
    // impeccable critique 2026-09-05, P0: AgentDetailPage/DeviceDetailPage'deki
    // dataStale desenini burada da uygula
    // json() boş dizi döner — sayfanın kendisi res.ok=false'ta zaten agents
    // state'ine dokunmuyor, ama alttaki ProcessesCard/L7Card/DnsCard aynı blanket
    // mock'u paylaşıyor ve dizi bekliyor (obje değil)
    vi.mocked(fetch).mockResolvedValue({ ok: false, status: 500, json: async () => [] } as Response)
    render(
      <MemoryRouter>
        <AgentsListPage />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText(/Liste güncellenemiyor/)).toBeInTheDocument())
  })
})
