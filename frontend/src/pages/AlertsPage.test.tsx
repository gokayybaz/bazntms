import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AlertsPage } from './AlertsPage'
import type { AlertConfig, AlertEvent } from '../types'

const MOCK_CONFIG: AlertConfig = {
  enabled: true,
  cooldown_min: 10,
  bandwidth: { enabled: true, in_mbps: 100, out_mbps: 50, seconds: 10 },
  ports: { enabled: true, ports: [23, 4444] },
  new_proc: { enabled: true, ignore: [] },
  new_target: { enabled: true, min_total_mb: 10 },
  anomaly: { enabled: true, sensitivity: 3, min_samples: 120, window_min: 5 },
  forti: { vpn_down: true, sdwan_latency_ms: 100, sdwan_jitter_ms: 30, sdwan_loss_pct: 1, max_sessions: 2000 },
  notifiers: { desktop: true, generic_url: '', discord_url: '', slack_url: '', telegram_token: '', telegram_chat_id: '' },
}

function ev(id: number, kind: AlertEvent['kind']): AlertEvent {
  return { id, ts: Math.floor(Date.now() / 1000), kind, key: `k${id}`, message: `olay ${id}` }
}

describe('AlertsPage', () => {
  beforeEach(() => {
    // AlertsCard alt bileseni /api/alerts'ten ayar cekiyor — sayfanin
    // kendi mantigini (byKind gruplama/siralama) test etmek icin gecerli
    // bir config donduruluyor
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ json: async () => MOCK_CONFIG } as Response)))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('olay yokken tur rozeti gostermez, "0 olay" yazar', () => {
    render(<AlertsPage alertEvents={[]} />)
    expect(screen.getByText('0 olay')).toBeInTheDocument()
    expect(screen.queryByText('bant genişliği')).not.toBeInTheDocument()
  })

  it('turlere gore sayar ve rozetleri COK OLANDAN AZA dogru siralar', () => {
    const events = [ev(1, 'bw'), ev(2, 'port'), ev(3, 'port'), ev(4, 'port'), ev(5, 'anomaly')]
    render(<AlertsPage alertEvents={events} />)

    expect(screen.getByText('5 olay')).toBeInTheDocument()

    const badges = screen.getAllByText(/bant genişliği|şüpheli port|anomali/)
    // "şüpheli port" 3 olayla en onde olmali (buyukten kucuge siralama)
    expect(badges[0]).toHaveTextContent('şüpheli port')

    // "şüpheli port" rozetinin sayaci 3 olmali
    const portBadge = badges[0].closest('span')
    expect(portBadge).toHaveTextContent('3')
  })

  it('KIND_LABELS listesinde olmayan bir tur icin ham kind adini gosterir', () => {
    // backend yeni bir alert kind'i eklenip frontend'in union tipi henuz
    // guncellenmemisse bile UI cokmemeli — ham kind adi fallback olarak
    // gosterilir (bkz. AlertsPage: KIND_LABELS[kind] ?? kind)
    const events = [{ id: 1, ts: 0, kind: 'yeni_bilinmeyen_tur', key: 'k1', message: 'x' }] as unknown as AlertEvent[]
    render(<AlertsPage alertEvents={events} />)
    expect(screen.getByText('yeni_bilinmeyen_tur')).toBeInTheDocument()
  })
})
