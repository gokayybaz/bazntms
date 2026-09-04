import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Overview } from './Overview'
import type { AlertEvent } from '../types'

// Overview artık olay akışındaki agent satırlarını Link/useNavigate ile
// agent detayına bağlıyor (impeccable critique P3) — Router context'i şart.
function renderOverview(props: Parameters<typeof Overview>[0]) {
  return render(
    <MemoryRouter>
      <Overview {...props} />
    </MemoryRouter>,
  )
}

const now = Math.floor(Date.now() / 1000)

const AGENTS = [
  {
    id: 1,
    name: 'agent-a',
    site: 'ofis-a',
    first_seen: 0,
    last_seen: now,
    version: 'v0.2.8',
    protocol_version: 1,
    remote_ip: '10.0.0.5',
    online: true,
    conns: 5,
    rates: [{ name: 'eth0', rx_bps: 1000, tx_bps: 2000, pps: 10, rx_bytes: 500_000, tx_bytes: 300_000, rx_packets: 0, tx_packets: 0 }],
  },
  {
    id: 2,
    name: 'agent-b',
    site: 'ofis-b',
    first_seen: 0,
    last_seen: now,
    version: 'v0.2.8',
    protocol_version: 1,
    remote_ip: '10.0.0.6',
    online: true,
    conns: 3,
    rates: [{ name: 'eth0', rx_bps: 500, tx_bps: 1000, pps: 5, rx_bytes: 200_000, tx_bytes: 100_000, rx_packets: 0, tx_packets: 0 }],
  },
]

const DEVICES = [
  { id: 1, name: 'switch-1', host: '10.0.0.1', kind: 'switch', vendor: 'generic', snmp_version: 2, enabled: true, last_poll: now, last_error: '' },
]

const FLOWS = [{ ts: now - 5, device: 'switch-1', src: '10.0.0.5', dst: '8.8.8.8', src_port: 5353, dst_port: 53, proto: 'udp', packets: 12, octets: 4096 }]
const SYSLOG = [{ id: 1, ts: now - 10, host: 'switch-1', severity: 5, tag: 'kernel', message: 'baglanti sifirlandi' }]

function agentDetailFor(id: number) {
  const agent = AGENTS.find((a) => a.id === id)!
  return {
    agent,
    connections: [{ proto: 'tcp', local_addr: `10.0.0.${id}:1000`, remote_addr: '2.2.2.2:443', status: 'ESTABLISHED', process: `proc-${id}` }],
  }
}

function mockFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/v1/agents') return Promise.resolve({ ok: true, status: 200, json: async () => AGENTS } as Response)
      if (url === '/api/v1/devices') return Promise.resolve({ ok: true, status: 200, json: async () => DEVICES } as Response)
      if (url.startsWith('/api/v1/flows')) return Promise.resolve({ ok: true, status: 200, json: async () => FLOWS } as Response)
      if (url.startsWith('/api/v1/syslog')) return Promise.resolve({ ok: true, status: 200, json: async () => SYSLOG } as Response)
      if (url === '/api/v1/topology') return Promise.resolve({ ok: true, json: async () => ({ devices: [], agents: [], links: [] }) } as Response)
      const m = url.match(/^\/api\/v1\/agents\/(\d+)$/)
      // handleAgentsList/Detail farkli olarak agentConns efekti res.ok
      // kontrol eder — bunu unutmak agentConns'in sessizce hep bos kalmasina
      // yol acar (bu test bunu canli olarak yakalayip duzeltti)
      if (m) return Promise.resolve({ ok: true, status: 200, json: async () => agentDetailFor(Number(m[1])) } as Response)
      return Promise.resolve({ ok: false, status: 404, json: async () => ({}) } as Response)
    }),
  )
}

describe('Overview', () => {
  beforeEach(() => mockFetch())
  afterEach(() => vi.unstubAllGlobals())

  it('agent filosu trafiğini (rx/tx/toplam/pps) tüm agent+arayüzler üzerinden doğru toplar', async () => {
    renderOverview({ refreshKey: 0, alertEvents: [] })

    // rx: 1000+500=1500 B/s ×8 = 12000 bit/s → 12.0 Kbit/s
    await waitFor(() => expect(screen.getByText('12.0 Kbit/s')).toBeInTheDocument())
    // tx: 2000+1000=3000 B/s ×8 = 24000 bit/s → 24.0 Kbit/s
    expect(screen.getByText('24.0 Kbit/s')).toBeInTheDocument()
    // toplam: (500000+300000)+(200000+100000) = 1.100.000 bayt → 1.0 MB
    expect(screen.getByText('1.0 MB')).toBeInTheDocument()
    // pps: 10+5=15
    expect(screen.getByText('15 pps')).toBeInTheDocument()
  })

  it('agent ve cihaz listelerini render eder', async () => {
    renderOverview({ refreshKey: 0, alertEvents: [] })
    // isimler hem filo/cihaz kartlarinda hem canli akistaki olaylarin
    // kaynagi olarak birden fazla yerde gecebilir — getAllByText kullanilir
    await waitFor(() => expect(screen.getAllByText('agent-a').length).toBeGreaterThan(0))
    expect(screen.getAllByText('agent-b').length).toBeGreaterThan(0)
    expect(screen.getAllByText('switch-1').length).toBeGreaterThan(0)
  })

  it('canlı olay akışında flow + syslog + agent bağlantılarını birleştirip en yeniden eskiye sıralar', async () => {
    renderOverview({ refreshKey: 0, alertEvents: [] })

    // agent baglantilari ikinci bir asenkron zincirle (agents yuklenince
    // tetiklenen ayri bir efekt) geldigi icin en son ortaya cikan — onu
    // beklemek digerlerinin de (flow/syslog) zaten yuklendigini garantiler
    await waitFor(() => expect(screen.getByText('proc-1', { exact: false })).toBeInTheDocument())

    const stream = screen.getByText('Canlı Olay Akışı').closest('div')!.parentElement!
    const text = stream.textContent ?? ''
    // agent baglantilari (ts=now, en yeni) flow (now-5) ve syslog (now-10)
    // mesajindan once gelmeli
    const agentIdx = text.indexOf('proc-1')
    const flowIdx = text.indexOf('8.8.8.8')
    const syslogIdx = text.indexOf('baglanti sifirlandi')
    expect(agentIdx).toBeGreaterThanOrEqual(0)
    expect(flowIdx).toBeGreaterThan(agentIdx)
    expect(syslogIdx).toBeGreaterThan(flowIdx)
  })

  it('canlı akış tür filtresi yalnızca seçili türü gösterir, "tümü" hepsini geri getirir', async () => {
    renderOverview({ refreshKey: 0, alertEvents: [] })
    await waitFor(() => expect(screen.getByText('proc-1', { exact: false })).toBeInTheDocument())

    // baslangicta uc kaynak da gorunur
    expect(screen.getByText('baglanti sifirlandi', { exact: false })).toBeInTheDocument()
    expect(screen.getByText(/8\.8\.8\.8/)).toBeInTheDocument()

    // "syslog" filtresine tikla → flow + agent satirlari gizlenir
    fireEvent.click(screen.getByRole('button', { name: /^syslog/i }))
    expect(screen.getByText('baglanti sifirlandi', { exact: false })).toBeInTheDocument()
    expect(screen.queryByText(/8\.8\.8\.8/)).not.toBeInTheDocument()
    expect(screen.queryByText('proc-1', { exact: false })).not.toBeInTheDocument()

    // "tümü" → hepsi geri gelir
    fireEvent.click(screen.getByRole('button', { name: /^tümü/i }))
    expect(screen.getByText(/8\.8\.8\.8/)).toBeInTheDocument()
    expect(screen.getByText('proc-1', { exact: false })).toBeInTheDocument()
  })

  it('flow satırında bayt + paket hacmini gösterir (API alanları octets/packets)', async () => {
    renderOverview({ refreshKey: 0, alertEvents: [] })
    // 4096 bayt → "4.0 KB", 12 paket → "12 pkt"
    await waitFor(() => expect(screen.getByText(/4\.0 KB/)).toBeInTheDocument())
    expect(screen.getByText(/12 pkt/)).toBeInTheDocument()
  })

  it('açık uyarıları listeler', async () => {
    const alerts: AlertEvent[] = [{ id: 1, ts: now, kind: 'port', key: 'k1', message: 'şüpheli port 4444' }]
    renderOverview({ refreshKey: 0, alertEvents: alerts })
    await waitFor(() => expect(screen.getByText('şüpheli port 4444')).toBeInTheDocument())
  })
})
