import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TopologyCard } from './TopologyCard'

const BASE = {
  generated_at: 0,
  devices: [
    { id: 1, name: 'core-sw', host: '10.0.0.2', kind: 'switch', sys_name: 'core-sw', online: true },
    { id: 2, name: 'edge-fw', host: '10.0.0.1', kind: 'firewall', sys_name: 'edge-fw', online: true },
  ],
  agents: [
    { id: 10, name: 'agent-a', site: 'ofis', online: true },
    { id: 11, name: 'agent-b', site: '', online: false },
  ],
  links: [],
}

function mockFetch(graph: unknown = BASE) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/topology') return Promise.resolve({ ok: true, json: async () => graph } as Response)
      return Promise.resolve({ ok: false, status: 404, text: async () => 'yok', json: async () => ({}) } as Response)
    }),
  )
}

describe('TopologyCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('client ▸ hub ▸ cihaz ▸ router ▸ internet omurgasını çizer; firewall Router yuvasına türer', async () => {
    mockFetch()
    render(<TopologyCard refreshKey={0} />)
    await waitFor(() => expect(screen.getByText('HUB')).toBeInTheDocument())
    expect(screen.getByText('ROUTER')).toBeInTheDocument()
    expect(screen.getByText('internet')).toBeInTheDocument()
    // firewall cihazı Router yuvasına türetildi → adı router etiketinde
    expect(screen.getByText('edge-fw')).toBeInTheDocument()
    // client (agent) solda, orta sütun cihazı sağda
    expect(screen.getByText('agent-a')).toBeInTheDocument()
    expect(screen.getByText('core-sw')).toBeInTheDocument()
  })

  it('çevrimdışı agent client sütunundan çıkar, sayısı alt not olur; cihaz çevrimdışıysa kalır', async () => {
    mockFetch()
    render(<TopologyCard refreshKey={5} />)
    await waitFor(() => expect(screen.getByText('agent-a')).toBeInTheDocument())
    // agent-b kapalı → gizli
    expect(screen.queryByText('agent-b')).not.toBeInTheDocument()
    expect(screen.getByText('+1 çevrimdışı gizli')).toBeInTheDocument()
  })

  it('router/firewall cihaz yoksa sabit Router düğümü gösterir', async () => {
    mockFetch({ ...BASE, devices: [BASE.devices[0]] })
    render(<TopologyCard refreshKey={1} />)
    await waitFor(() => expect(screen.getByText('ROUTER')).toBeInTheDocument())
    expect(screen.getByText('sabit')).toBeInTheDocument()
  })

  it('cihaz ve agent yoksa boş mesaj gösterir', async () => {
    mockFetch({ generated_at: 0, devices: [], agents: [], links: [] })
    render(<TopologyCard refreshKey={2} />)
    await waitFor(() => expect(screen.getByText(/Topoloji boş/)).toBeInTheDocument())
  })
})
