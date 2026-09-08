import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TopConversationsCard } from './TopConversationsCard'

const CONVOS = [
  { src: '10.0.0.1', dst: '8.8.8.8', proto: 'tcp', flows: 3, packets: 23, octets: 5500, first_seen: 1, last_seen: Math.floor(Date.now() / 1000) },
  { src: '10.0.0.2', dst: '1.1.1.1', proto: 'udp', flows: 1, packets: 2, octets: 120, first_seen: 1, last_seen: Math.floor(Date.now() / 1000) },
]
const DRILL = {
  flows: [{ ts: Math.floor(Date.now() / 1000), device: 'fw1', src: '10.0.0.1', dst: '8.8.8.8', src_port: 5000, dst_port: 443, proto: 'tcp', packets: 10, octets: 2000 }],
  actors: [{ agent_id: 1, agent_name: 'host-a', process: 'curl', ip: '8.8.8.8' }],
  src_info: {},
  dst_info: { asn: 'AS15169 Google LLC' },
}

function mockFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url.includes('/flows/conversations?')) return Promise.resolve({ ok: true, status: 200, json: async () => CONVOS } as Response)
      if (url.includes('/flows/conversation?')) return Promise.resolve({ ok: true, status: 200, json: async () => DRILL } as Response)
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

describe('TopConversationsCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('konuşmaları listeler ve Enter ile drill-down açar', async () => {
    mockFetch()
    render(<TopConversationsCard />)
    await waitFor(() => expect(screen.getByText('8.8.8.8')).toBeInTheDocument())
    const user = userEvent.setup()
    await user.click(screen.getByText('8.8.8.8'))
    expect(await screen.findByText(/host-a\/curl/)).toBeInTheDocument()
    expect(screen.getByText(/AS15169 Google LLC/)).toBeInTheDocument()
  })
})
