import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { TrafficFlowDiagram, type DiagramAgent, type TrafficEvent } from './TrafficFlowDiagram'

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
  it('üç bölgeyi ve göstergeyi çizer', () => {
    render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} pps={0} />)
    expect(screen.getByText('AGENT FİLOSU')).toBeInTheDocument()
    expect(screen.getByText('ROUTER · GÜVENLİK DUVARI')).toBeInTheDocument()
    expect(screen.getByText('İNTERNET')).toBeInTheDocument()
    expect(screen.getByText('Giden · agent → internet')).toBeInTheDocument()
    expect(screen.getByText('Gelen · internet → agent')).toBeInTheDocument()
  })

  it('filodaki HER agent için bir düğüm çizer (online + offline)', () => {
    render(<TrafficFlowDiagram events={[]} agents={AGENTS} pps={0} />)
    expect(screen.getByText('agent-a')).toBeInTheDocument()
    expect(screen.getByText('agent-b')).toBeInTheDocument()
    expect(screen.getByText('agent-c')).toBeInTheDocument()
    // 3 agent, 2 online
    expect(screen.getByText('2/3 online')).toBeInTheDocument()
  })

  it('agent sayısı çoğaldıkça hepsi yine de çizilir', () => {
    const many: DiagramAgent[] = Array.from({ length: 30 }, (_, i) => ({
      name: `edge-prob-${String(i).padStart(2, '0')}`,
      online: i % 4 !== 0,
    }))
    render(<TrafficFlowDiagram events={[]} agents={many} pps={0} />)
    // her düğüm hem <title> hem görünür <text> üretir — en az biri var
    expect(screen.getAllByText(/edge-prob-00/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/edge-prob-29/).length).toBeGreaterThan(0)
    expect(screen.getByText('22/30 online')).toBeInTheDocument()
  })

  it('agent listesi boşken hata vermeden render olur', () => {
    render(<TrafficFlowDiagram events={[]} agents={[]} />)
    expect(screen.getByText('İNTERNET')).toBeInTheDocument()
    expect(screen.getByText('agent yok')).toBeInTheDocument()
  })

  it('ilk dolu partiden sonra gelen yeni olay "giden" sayacına işlenir', () => {
    const { rerender } = render(<TrafficFlowDiagram events={EVENTS} agents={AGENTS} pps={0} />)
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('0')
    rerender(
      <TrafficFlowDiagram
        events={[{ key: 'f2', kind: 'flow', ts: now + 1, from: '10.0.0.9', to: '9.9.9.9:443' }, ...EVENTS]}
        agents={AGENTS}
        pps={0}
      />,
    )
    expect(screen.getByText('Giden · agent → internet').parentElement?.textContent).toContain('1')
  })
})
