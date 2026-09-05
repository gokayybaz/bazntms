import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AlertsCard } from './AlertsCard'
import type { AlertConfig, AlertEvent } from '../types'

// Faz 4.5 smoke test (bkz. GeoMapCard.test.tsx üstteki not).

const CONFIG: AlertConfig = {
  enabled: true,
  cooldown_min: 10,
  bandwidth: { enabled: true, in_mbps: 100, out_mbps: 50, seconds: 30 },
  ports: { enabled: true, ports: [23, 4444] },
  new_proc: { enabled: true, ignore: [] },
  new_target: { enabled: true, min_total_mb: 50 },
  anomaly: { enabled: false, sensitivity: 3, min_samples: 10, window_min: 60 },
  forti: { vpn_down: true, sdwan_latency_ms: 100, sdwan_jitter_ms: 30, sdwan_loss_pct: 5, max_sessions: 0 },
  notifiers: {
    desktop: true,
    generic_url: '',
    discord_url: '',
    slack_url: '',
    telegram_token: '',
    telegram_chat_id: '',
  },
}

const EVENTS: AlertEvent[] = [{ id: 1, ts: Math.floor(Date.now() / 1000), kind: 'port', key: 'k1', message: 'şüpheli port 4444' }]

function mockFetch(cfg: AlertConfig = CONFIG) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 200, ok: true, json: async () => cfg } as Response)))
}

describe('AlertsCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('çöküş olmadan render olur, olay akışını ve ayarları gösterir', async () => {
    mockFetch()
    render(<AlertsCard events={EVENTS} />)
    expect(screen.getByText('Ayarlar yükleniyor…')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('Olay Akışı')).toBeInTheDocument())
    expect(screen.getByText('şüpheli port')).toBeInTheDocument()
    expect(screen.getByText('şüpheli port 4444')).toBeInTheDocument()
    expect(screen.getByText('Ayarları Kaydet')).toBeInTheDocument()
  })

  it('olay listesi boşken boş-durum mesajı gösterir', async () => {
    mockFetch()
    render(<AlertsCard events={[]} />)
    await waitFor(() => expect(screen.getByText('Henüz uyarı yok.')).toBeInTheDocument())
  })

  it('ayarlar alınamayınca hata mesajı gösterir', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 500, ok: false, json: async () => ({}) } as Response)))
    render(<AlertsCard events={[]} />)
    await waitFor(() => expect(screen.getByText(/ayarlar alınamadı/)).toBeInTheDocument())
  })

  it('403 → yetki mesajı gösterir ama olay akışını korur (S11.3)', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 403, ok: false, json: async () => ({}) } as Response)))
    render(<AlertsCard events={EVENTS} />)
    await waitFor(() => expect(screen.getByText(/yalnızca/)).toBeInTheDocument())
    expect(screen.getByText(/yöneticilere/)).toBeInTheDocument()
    expect(screen.queryByText('Ayarları Kaydet')).not.toBeInTheDocument()
    // olay akışı viewer'a da görünür
    expect(screen.getByText('şüpheli port 4444')).toBeInTheDocument()
  })

  it('soğuma alanı ve SIEM taşıma seçici gerçek <label> ile eşleşiyor', async () => {
    mockFetch()
    render(<AlertsCard events={[]} />)
    await waitFor(() => expect(screen.getByLabelText(/soğuma/)).toBeInTheDocument())
    expect(screen.getByLabelText('Taşıma')).toBeInTheDocument()
  })

  it('"Kanalları Test Et" POST /api/alerts/test çağırır ve durumu günceller (D3)', async () => {
    const seen: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, init?: RequestInit) => {
        seen.push((init?.method ?? 'GET') + ' ' + url)
        if (url === '/api/alerts/status')
          return Promise.resolve({ ok: true, json: async () => ({ channels: {} }) } as Response)
        if (url === '/api/alerts/test' && init?.method === 'POST')
          return Promise.resolve({
            ok: true,
            json: async () => ({ channels: { discord: { last_attempt: 1_700_000_000, ok: false, error: 'HTTP 500' } } }),
          } as Response)
        return Promise.resolve({ status: 200, ok: true, json: async () => CONFIG } as Response)
      }),
    )
    render(<AlertsCard events={[]} />)
    await waitFor(() => expect(screen.getByText('Kanalları Test Et')).toBeInTheDocument())
    await userEvent.click(screen.getByText('Kanalları Test Et'))
    await waitFor(() => expect(seen).toContain('POST /api/alerts/test'))
    // discord etiketinin yanında kırmızı durum noktası (title'da hata)
    await waitFor(() => {
      const dot = document.querySelector('span[title*="HTTP 500"]')
      expect(dot).toBeTruthy()
    })
  })
})
