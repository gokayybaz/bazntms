import { render, screen, waitFor } from '@testing-library/react'
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
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ status: 200, json: async () => cfg } as Response)))
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
})
