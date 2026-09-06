import { useEffect, useMemo, useState } from 'react'
import type { AgentWithRates } from '../types'
import { TrafficFlowDiagram } from './TrafficFlowDiagram'
import type { DiagramAgent, TrafficEvent } from './TrafficFlowDiagram'

// TrafficFlowCard — canlı trafik şemasının kendi kendine yeten sürümü.
// Dashboard'dan ayrı bir sekmeye taşındı (çok agent / çok trafik senaryosunda
// pano şişmesin — rAF animasyonu yalnız bu sayfa açıkken çalışır).

interface FlowRow {
  ts: number
  device: string
  src: string
  dst: string
  src_port: number
  dst_port: number
  proto: string
  packets: number
  octets: number
}
interface SyslogEvent {
  id: number
  ts: number
  host: string
  severity: number
  tag: string
  message: string
}
interface AgentConnSample {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
}

export function TrafficFlowCard() {
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [syslog, setSyslog] = useState<SyslogEvent[]>([])
  const [agentConns, setAgentConns] = useState<{ agentName: string; ts: number; conns: AgentConnSample[] }[]>([])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.ok && !stop) setAgents(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/flows?minutes=15&limit=20')
        if (res.ok && !stop) setFlows(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 6_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/syslog?limit=20')
        if (res.ok && !stop) setSyslog(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  const onlineAgentKey = agents.filter((a) => a.online).map((a) => a.id).join(',')
  useEffect(() => {
    if (!onlineAgentKey) {
      setAgentConns([])
      return
    }
    let stop = false
    const ids = onlineAgentKey.split(',').slice(0, 40) // aşırı büyük filoda istek patlamasın
    const load = async () => {
      try {
        const results = await Promise.all(
          ids.map(async (idStr) => {
            const res = await fetch(`/api/v1/agents/${idStr}`)
            if (!res.ok) return null
            const data: { agent: AgentWithRates; connections: AgentConnSample[] } = await res.json()
            return { agentName: data.agent.name, ts: data.agent.last_seen, conns: data.connections ?? [] }
          }),
        )
        if (!stop) setAgentConns(results.filter((r): r is { agentName: string; ts: number; conns: AgentConnSample[] } => r !== null))
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 7_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [onlineAgentKey])

  // sadece ÇEVRİMİÇİ agent'lar düğüm olur (süzme bileşen içinde); tüm filo
  // geçilir ki "N çevrimdışı gizli" ipucu gösterilebilsin.
  const diagramAgents = useMemo<DiagramAgent[]>(
    () =>
      [...agents]
        .sort((a, b) => Number(b.online) - Number(a.online) || a.name.localeCompare(b.name))
        .map((a) => {
          const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
          return { name: a.name, online: a.online, site: a.site || undefined, rxBps: busiest?.rx_bps ?? 0, txBps: busiest?.tx_bps ?? 0 }
        }),
    [agents],
  )

  const diagramEvents = useMemo<TrafficEvent[]>(() => {
    type Item =
      | { kind: 'flow'; ts: number; key: string; src: string; dst: string; dport: number; bytes: number }
      | { kind: 'syslog'; ts: number; key: string; source: string }
      | { kind: 'agent'; ts: number; key: string; source: string; local: string; remote?: string }
    const items: Item[] = [
      ...flows.map((f) => ({ kind: 'flow' as const, ts: f.ts, key: `f${f.ts}-${f.src}-${f.dst}-${f.src_port}`, src: f.src, dst: f.dst, dport: f.dst_port, bytes: f.octets ?? 0 })),
      ...syslog.map((e) => ({ kind: 'syslog' as const, ts: e.ts, key: `s${e.id}`, source: e.host })),
      ...agentConns.flatMap((a) =>
        a.conns.map((c, i) => ({ kind: 'agent' as const, ts: a.ts, key: `a${a.agentName}-${i}-${c.local_addr}-${c.remote_addr ?? ''}`, source: a.agentName, local: c.local_addr, remote: c.remote_addr })),
      ),
    ]
    return items
      .sort((a, b) => b.ts - a.ts)
      .slice(0, 80)
      .flatMap((it): TrafficEvent[] => {
        if (it.kind === 'flow') return [{ key: it.key, kind: 'flow', ts: it.ts, from: it.src, to: `${it.dst}:${it.dport}`, weight: it.bytes }]
        if (it.kind === 'agent') return it.remote ? [{ key: it.key, kind: 'agent', ts: it.ts, agent: it.source, from: it.local, to: it.remote }] : []
        return [{ key: it.key, kind: 'syslog', ts: it.ts, from: it.source }]
      })
  }, [flows, syslog, agentConns])

  return <TrafficFlowDiagram events={diagramEvents} agents={diagramAgents} />
}
