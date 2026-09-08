import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AlertEventsPanel } from './AlertEventsPanel'
import { DialogProvider } from '../lib/dialog'

const EVENTS = [
  {
    id: 7, ts: 1000, kind: 'ioc', key: 'evil.com', message: 'tehdit alan adı',
    severity: 'crit', state: 'firing', site: 'dc1', count: 3, first_ts: 900, last_ts: Math.floor(Date.now() / 1000),
    group_id: 'g-5',
  },
  {
    id: 8, ts: 1000, kind: 'bw', key: 'agent-in:x', message: 'yüksek bant',
    severity: 'warn', state: 'firing', site: 'dc2', count: 1, first_ts: 1000, last_ts: Math.floor(Date.now() / 1000),
  },
]

function mockFetch(onAck?: () => void) {
  return vi.fn((url: string, init?: RequestInit) => {
    if (url.includes('/api/v1/alerts/events') && (!init || init.method !== 'POST')) {
      return Promise.resolve({ ok: true, json: async () => ({ events: EVENTS, next_cursor: 0 }) } as Response)
    }
    if (url.includes('/ack')) {
      onAck?.()
      return Promise.resolve({ ok: true, json: async () => ({ ok: true }) } as Response)
    }
    if (url.includes('/api/v1/alerts/silences')) {
      return Promise.resolve({ ok: true, json: async () => ({ silences: [] }) } as Response)
    }
    return Promise.resolve({ ok: true, json: async () => ({}) } as Response)
  })
}

afterEach(() => vi.unstubAllGlobals())

describe('AlertEventsPanel', () => {
  it('olayları listeler ve önem/durum rozetlerini gösterir', async () => {
    vi.stubGlobal('fetch', mockFetch())
    render(
      <DialogProvider>
        <AlertEventsPanel />
      </DialogProvider>,
    )
    await waitFor(() => expect(screen.getByText('tehdit alan adı')).toBeInTheDocument())
    expect(screen.getByText('yüksek bant')).toBeInTheDocument()
    expect(screen.getAllByText('crit').length).toBeGreaterThan(0)
    expect(screen.getByText('g-5')).toBeInTheDocument()
  })

  it('satır etkinleştirince işlem dialogu açılır ve kabul et POST eder', async () => {
    const ack = vi.fn()
    vi.stubGlobal('fetch', mockFetch(ack))
    const user = userEvent.setup()
    render(
      <DialogProvider>
        <AlertEventsPanel />
      </DialogProvider>,
    )
    await waitFor(() => expect(screen.getByText('tehdit alan adı')).toBeInTheDocument())
    await user.dblClick(screen.getByText('tehdit alan adı'))
    // form dialogu (varsayılan işlem "kabul et")
    await waitFor(() => expect(screen.getByText(/Uyarı #7/)).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: 'Uygula' }))
    await waitFor(() => expect(ack).toHaveBeenCalled())
  })
})
