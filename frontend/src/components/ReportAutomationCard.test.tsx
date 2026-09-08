import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReportAutomationCard } from './ReportAutomationCard'
import { SLATargetsCard } from './SLATargetsCard'
import { DialogProvider } from '../lib/dialog'

afterEach(() => vi.unstubAllGlobals())

describe('ReportAutomationCard', () => {
  it('zamanlamaları ve arşivi listeler', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) => {
        if (url.includes('/schedules')) {
          return Promise.resolve({
            ok: true,
            json: async () => ({
              schedules: [
                { id: 1, spec: 'weekly:mon:07:00', enabled: true, next_run_ts: 0, last_run_ts: 0, last_status: 'ok', payload: { type: 'enterprise', days: 30, site: '', format: 'pdf', email: ['ops@x.com'] } },
              ],
            }),
          } as Response)
        }
        if (url.includes('/archive')) {
          return Promise.resolve({
            ok: true,
            json: async () => ({ archive: [{ id: 5, kind: 'enterprise', site: '', days: 30, format: 'pdf', size: 4096, generated_ts: 1_700_000_000, delivered_to: 'ops@x.com', status: 'ok' }] }),
          } as Response)
        }
        return Promise.resolve({ ok: true, json: async () => ({}) } as Response)
      }),
    )
    render(
      <DialogProvider>
        <ReportAutomationCard />
      </DialogProvider>,
    )
    await waitFor(() => expect(screen.getByText('weekly:mon:07:00')).toBeInTheDocument())
    expect(screen.getByText(/Arşiv \(1\)/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /enterprise · pdf/ })).toHaveAttribute('href', '/api/v1/reports/archive/5')
  })
})

describe('SLATargetsCard', () => {
  it('hedefleri gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: async () => ({ targets: [{ scope: 'global', site: '', agent_uptime_pct: 95, device_health_pct: 90, iface_err_ceiling: 0 }] }) } as Response)))
    render(
      <DialogProvider>
        <SLATargetsCard />
      </DialogProvider>,
    )
    await waitFor(() => expect(screen.getByText('global')).toBeInTheDocument())
    expect(screen.getByText('uptime ≥ 95%')).toBeInTheDocument()
  })
})
