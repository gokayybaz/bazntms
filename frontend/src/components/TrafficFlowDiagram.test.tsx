import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TrafficFlowDiagram, type DiagramAgent, type TrafficEvent } from './TrafficFlowDiagram'

function mockReducedMotion(matches: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({
      matches,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  )
}

const now = Math.floor(Date.now() / 1000)

const AGENTS: DiagramAgent[] = [
  { name: 'agent-a', online: true, rxBps: 1000, txBps: 2000, site: 'ofis-a' },
  { name: 'agent-b', online: true, rxBps: 500, txBps: 1000, site: 'ofis-b' },
  { name: 'agent-c', online: false, site: 'ofis-c' },
]

const EVENTS: TrafficEvent[] = [
  { key: 'f1', kind: 'flow', ts: now, from: '10.0.0.5', to: '8.8.8.8:53', weight: 4096 },
  { key: 'a1', kind: 'agent', ts: now, agent: 'agent-a', from: '10.0.0.1:1000', to: '2.2.2.2:443' },
  { key: 's1', kind: 'syslog', ts: now, from: 'switch-1', label: 'link down' },
]

describe('TrafficFlowDiagram', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('üç bölgeyi ve göstergeyi çizer', () => {
    render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} />)
    expect(screen.getByText('AGENT FİLOSU')).toBeInTheDocument()
    expect(screen.getByText('ROUTER · GÜVENLİK DUVARI')).toBeInTheDocument()
    expect(screen.getByText('İNTERNET')).toBeInTheDocument()
    expect(screen.getByText('Giden · agent → internet')).toBeInTheDocument()
    expect(screen.getByText('Gelen · internet → agent')).toBeInTheDocument()
  })

  it('yalnızca ÇEVRİMİÇİ agent için düğüm çizer — kapalı agent şemadan tamamen çıkar', () => {
    render(<TrafficFlowDiagram events={[]} agents={AGENTS} />)
    expect(screen.getByText('agent-a')).toBeInTheDocument()
    expect(screen.getByText('agent-b')).toBeInTheDocument()
    // agent-c kapalı → hiçbir düğüm/başlık üretmez
    expect(screen.queryByText('agent-c')).not.toBeInTheDocument()
    expect(screen.getByText('2 aktif · 1 çevrimdışı gizli')).toBeInTheDocument()
  })

  it('kalabalık filoda yalnızca çevrimiçi düğümler çizilir', () => {
    const many: DiagramAgent[] = Array.from({ length: 30 }, (_, i) => ({
      name: `edge-prob-${String(i).padStart(2, '0')}`,
      online: i % 4 !== 0,
    }))
    render(<TrafficFlowDiagram events={[]} agents={many} />)
    // i=0 kapalı → gizli; i=1 ve i=29 çevrimiçi → görünür (her düğüm <title>+<text> üretir)
    expect(screen.queryAllByText(/edge-prob-00/).length).toBe(0)
    expect(screen.getAllByText(/edge-prob-01/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/edge-prob-29/).length).toBeGreaterThan(0)
    // 30 agent, 8 kapalı (i%4===0), 22 aktif
    expect(screen.getByText('22 aktif · 8 çevrimdışı gizli')).toBeInTheDocument()
  })

  it('agent listesi boşken hata vermeden render olur', () => {
    render(<TrafficFlowDiagram events={[]} agents={[]} />)
    expect(screen.getByText('İNTERNET')).toBeInTheDocument()
    expect(screen.getByText('aktif agent yok')).toBeInTheDocument()
  })

  it('ilk dolu partiden sonra gelen yeni olay "giden" sayacına işlenir', () => {
    const { rerender } = render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} />)
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('0')
    rerender(
      <TrafficFlowDiagram
        events={[{ key: 'f2', kind: 'flow', ts: now + 1, from: '10.0.0.9', to: '9.9.9.9:443' }, ...EVENTS]}
        agents={AGENTS}
      />,
    )
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('1')
  })

  it('ilk partiden sonra gelen ÇEVRİMDIŞI agent olayı hiçbir sayaca işlenmez', () => {
    const { rerender } = render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} />)
    rerender(
      <TrafficFlowDiagram
        events={[
          // agent-c kapalı → paket de sayaç da yok
          { key: 'off1', kind: 'agent', ts: now + 1, agent: 'agent-c', from: '10.0.0.3:5000', to: '3.3.3.3:443' },
          ...EVENTS,
        ]}
        agents={AGENTS}
      />,
    )
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('0')
  })

  it('hareket azaltma açıkken bile yeni olaylar sayaca işlenir — veri donmaz, yalnızca animasyon durur', () => {
    // eskiden `reduced` iken olay-işleme effect'i de tamamen atlanıyordu;
    // bu yüzden hareket-azaltma tercih eden kullanıcılar tally/netEnd
    // verisini de hiç almıyordu (impeccable critique 2026-09-05, P1)
    mockReducedMotion(true)
    const { rerender } = render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} />)
    expect(screen.getByText('hareket azaltma açık')).toBeInTheDocument()
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('0')
    rerender(
      <TrafficFlowDiagram
        events={[{ key: 'f2', kind: 'flow', ts: now + 1, from: '10.0.0.9', to: '9.9.9.9:443' }, ...EVENTS]}
        agents={AGENTS}
      />,
    )
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('1')
    vi.unstubAllGlobals()
  })
})
