import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AnomalyPage } from './AnomalyPage'

const BASELINE = {
  dim: 'fleet',
  metric: 'bps',
  seasonality: 'weekday',
  rows: [
    { dim: 'fleet', metric: 'bps', key: '', bucket: 9, n: 400, mean: 8_000_000, m2: 400 * 1_000_000 * 1_000_000 },
    { dim: 'fleet', metric: 'bps', key: '', bucket: 10, n: 400, mean: 9_000_000, m2: 400 * 1_200_000 * 1_200_000 },
  ],
}
const ACTIVE = {
  deviations: [
    { metric: 'bps', dim: 'agent', key: '7', scope: 'Agent #7', bucket: 9, mean: 50_000, std: 8_000, cur: 1_400_000, z: 168, n: 300 },
    { metric: 'dns_qps', dim: 'fleet', key: '', scope: 'Filo geneli', bucket: 9, mean: 3, std: 0.4, cur: 41, z: 95, n: 500 },
  ],
}

function mockFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url.startsWith('/api/v1/anomaly/baseline')) {
        return Promise.resolve({ ok: true, status: 200, json: async () => BASELINE } as Response)
      }
      if (url === '/api/v1/anomaly/active') {
        return Promise.resolve({ ok: true, status: 200, json: async () => ACTIVE } as Response)
      }
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('AnomalyPage', () => {
  it('baseline eğrisini ve aktif sapmaları gösterir', async () => {
    mockFetch()
    render(<AnomalyPage />)

    expect(screen.getByRole('heading', { name: 'Anomali' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('img', { name: /beklenen bant/i })).toBeInTheDocument())

    // aktif sapma satırları
    await waitFor(() => expect(screen.getByText('Agent #7')).toBeInTheDocument())
    expect(screen.getByText('Filo geneli')).toBeInTheDocument()
    expect(screen.getByText('2 sapma · z ≥ eşik')).toBeInTheDocument()
  })

  it('metrik seçiciyi değiştirince baseline yeniden istenir', async () => {
    mockFetch()
    render(<AnomalyPage />)
    await waitFor(() => expect(screen.getByRole('img', { name: /beklenen bant/i })).toBeInTheDocument())

    await userEvent.click(screen.getByRole('button', { name: 'DNS sorgu hızı' }))
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith('/api/v1/anomaly/baseline?dim=fleet&metric=dns_qps'),
    )
  })
})
