import { useEffect, useMemo, useState } from 'react'
import type { AgentWithRates, AlertEvent } from '../types'
import type { FleetSummary } from '../lib/useLive'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { Card } from './Card'
import { TopologyCard } from './TopologyCard'
import { GeoMapCard } from './GeoMapCard'
import { TrafficFlowDiagram } from './TrafficFlowDiagram'
import type { DiagramAgent, TrafficEvent } from './TrafficFlowDiagram'

// --- yerel API tipleri (DevicesCard/FlowsCard/SyslogCard ile ayni sema) ---

interface Device {
  id: number
  name: string
  host: string
  kind: string
  vendor: string
  snmp_version: number
  enabled: boolean
  last_poll: number
  last_error: string
}

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
  process?: string
  pid?: number
}

type StreamKind = 'flow' | 'syslog' | 'agent'

type StreamItem =
  | { kind: 'flow'; ts: number; key: string; primary: string; source: string; bytes: number; packets: number; src: string; dst: string; dport: number; proto: string }
  | { kind: 'syslog'; ts: number; key: string; primary: string; source: string; severity: number }
  | { kind: 'agent'; ts: number; key: string; primary: string; source: string; pid?: number; local: string; remote?: string }

const ALERT_KIND_STYLES: Record<string, string> = {
  bw: 'border-amber-500/30 bg-amber-500/10 text-amber-400',
  port: 'border-rose-500/30 bg-rose-500/10 text-rose-400',
  proc: 'border-sky-500/30 bg-sky-500/10 text-sky-400',
  target: 'border-violet-500/30 bg-violet-500/10 text-violet-400',
  ioc: 'border-red-500/40 bg-red-500/15 text-red-300',
}
const ALERT_KIND_LABELS: Record<string, string> = {
  bw: 'bant genişliği',
  port: 'şüpheli port',
  proc: 'yeni süreç',
  target: 'yeni hedef',
  ioc: 'ioc / tehdit',
}

function relTime(unix: number): string {
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

export function Overview({
  refreshKey,
  alertEvents,
  fleet,
}: {
  refreshKey: number
  alertEvents: AlertEvent[]
  fleet?: FleetSummary | null
}) {
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [syslog, setSyslog] = useState<SyslogEvent[]>([])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.status === 401) return
        if (!stop) setAgents(await res.json())
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
  }, [refreshKey])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/devices')
        if (res.status === 401) return
        if (!stop) setDevices(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 8_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [refreshKey])

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/flows?minutes=15&limit=20')
        if (res.status === 401) return
        if (!stop) setFlows(await res.json())
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
        if (res.status === 401) return
        if (!stop) setSyslog(await res.json())
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

  // agent bağlantıları: her online agent'ın son telemetri anındaki gerçek
  // bağlantı listesi (/api/v1/agents/{id}) — akışa "agent" kaynağı olarak girer
  const [agentConns, setAgentConns] = useState<{ agentName: string; ts: number; conns: AgentConnSample[] }[]>([])
  const onlineAgentKey = agents.filter((a) => a.online).map((a) => a.id).join(',')

  useEffect(() => {
    if (!onlineAgentKey) {
      setAgentConns([])
      return
    }
    let stop = false
    const ids = onlineAgentKey.split(',').slice(0, 40) // asiri buyuk filoda istek patlamasin
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

  // --- turetilmis metrikler ---
  // fleet (WS tick, 1 sn) varsa canlı; yoksa 5 sn'lik REST polling'inden.
  const onlineAgents = fleet ? fleet.agents_online : agents.filter((a) => a.online).length
  const agentsTotal = fleet ? fleet.agents_total : agents.length
  const totalConns = agents.reduce((sum, a) => sum + (a.conns || 0), 0)
  // cihaz icin ayri bir "online" alani yok: son 3 poll araligi icinde hata
  // vermeden yoklanmis olmasi saglikli kabul edilir (kaba tahmin)
  const healthyDevices = devices.filter((d) => d.enabled && d.last_poll > 0 && !d.last_error).length

  const stream = useMemo<StreamItem[]>(() => {
    const items: StreamItem[] = [
      ...flows.map((f) => ({
        kind: 'flow' as const,
        ts: f.ts,
        key: `f${f.ts}-${f.src}-${f.dst}-${f.src_port}`,
        primary: `${f.src}:${f.src_port} → ${f.dst}:${f.dst_port} ${f.proto}`,
        source: f.device,
        bytes: f.octets ?? 0,
        packets: f.packets ?? 0,
        src: f.src,
        dst: f.dst,
        dport: f.dst_port,
        proto: f.proto,
      })),
      ...syslog.map((e) => ({
        kind: 'syslog' as const,
        ts: e.ts,
        key: `s${e.id}`,
        primary: `${e.tag ? e.tag + ': ' : ''}${e.message}`,
        source: e.host,
        severity: e.severity,
      })),
      ...agentConns.flatMap((a) =>
        // agent'in son telemetri anindaki TUM baglanti envanteri — LISTEN
        // (karsi ucu olmayan) satirlar da dahil, hicbiri gizlenmiyor
        a.conns.map((c, i) => ({
          kind: 'agent' as const,
          ts: a.ts,
          key: `a${a.agentName}-${i}-${c.local_addr}-${c.remote_addr ?? ''}`,
          primary: `${c.local_addr}${c.remote_addr ? ' → ' + c.remote_addr : ''} ${c.proto}${c.status ? ' · ' + c.status : ''}${c.process ? ' · ' + c.process : ''}`,
          source: a.agentName,
          pid: c.pid,
          local: c.local_addr,
          remote: c.remote_addr,
        })),
      ),
    ]
    return items.sort((a, b) => b.ts - a.ts).slice(0, 200)
  }, [flows, syslog, agentConns])

  // canli akis tur filtresi (flow / agent / syslog) + tur bazli sayaclar
  const [streamFilter, setStreamFilter] = useState<StreamKind | 'all'>('all')
  const streamCounts = useMemo(() => {
    const c: Record<StreamKind, number> = { flow: 0, agent: 0, syslog: 0 }
    for (const it of stream) c[it.kind]++
    return c
  }, [stream])
  const visibleStream = streamFilter === 'all' ? stream : stream.filter((it) => it.kind === streamFilter)

  // canlı trafik şeması: sol sütunda filonun HER agent'ı ayrı düğüm
  const diagramAgents = useMemo<DiagramAgent[]>(
    () =>
      [...agents]
        .sort((a, b) => Number(b.online) - Number(a.online) || a.name.localeCompare(b.name))
        .map((a) => {
          const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
          return {
            name: a.name,
            online: a.online,
            site: a.site || undefined,
            rxBps: busiest?.rx_bps ?? 0,
            txBps: busiest?.tx_bps ?? 0,
          }
        }),
    [agents],
  )

  // canlı trafik şeması için olay listesi — akıştaki en yeni 80 satır,
  // yön sınıflandırması diyagramın içinde (from/to özel/genel IP kontrolü).
  // Karşı ucu olmayan agent satırları (LISTEN soketleri) şemaya alınmaz —
  // yön taşımazlar, yalnızca "· dinliyor" gürültüsü olurlar.
  const diagramEvents = useMemo<TrafficEvent[]>(
    () =>
      visibleStream.slice(0, 80).flatMap((it): TrafficEvent[] => {
        if (it.kind === 'flow') {
          return [{ key: it.key, kind: 'flow', ts: it.ts, from: it.src, to: `${it.dst}:${it.dport}`, weight: it.bytes }]
        }
        if (it.kind === 'agent') {
          return it.remote ? [{ key: it.key, kind: 'agent', ts: it.ts, agent: it.source, from: it.local, to: it.remote }] : []
        }
        return [{ key: it.key, kind: 'syslog', ts: it.ts, from: it.source }]
      }),
    [visibleStream],
  )

  const polledEventRate = useMemo(() => {
    const now = Math.floor(Date.now() / 1000)
    const recent = [...flows, ...syslog].filter((i) => now - i.ts <= 60).length
    return recent / 60
  }, [flows, syslog])
  const eventRate = fleet ? fleet.flows_per_min / 60 : polledEventRate

  const recentAlerts = [...alertEvents].sort((a, b) => b.ts - a.ts).slice(0, 8)

  // agent filosu toplam trafiği: her agent'ın her arayüzünün son iki
  // telemetri örneğinden hesaplanmış rx_bps/tx_bps/pps toplamı (backend'de
  // hesaplanır, bkz. store.ListAgents)
  const agentTraffic = useMemo(() => {
    let rxBps = 0, txBps = 0, rxBytes = 0, txBytes = 0, pps = 0
    for (const a of agents) {
      for (const r of a.rates ?? []) {
        rxBps += r.rx_bps
        txBps += r.tx_bps
        rxBytes += r.rx_bytes
        txBytes += r.tx_bytes
        pps += r.pps
      }
    }
    return { rxBps, txBps, totalBytes: rxBytes + txBytes, pps }
  }, [agents])

  // canlı hız/pps: fleet (WS, bit/sn → bayt/sn) varsa; toplam bayt polling'den
  const liveRxBps = fleet ? fleet.rx_bps / 8 : agentTraffic.rxBps
  const liveTxBps = fleet ? fleet.tx_bps / 8 : agentTraffic.txBps
  const livePps = fleet ? fleet.pps : agentTraffic.pps

  return (
    <div className="space-y-4">
      {/* özet stat şeridi */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">
            Aktif Agent {fleet && <span className="ml-1 text-emerald-400" title="WS canlı (1 sn)">●</span>}
          </p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-slate-100">
            {onlineAgents}
            <span className="text-sm font-medium text-slate-600"> / {agentsTotal}</span>
          </p>
          <p className="mt-0.5 text-[10.5px] text-slate-600">{Math.max(0, agentsTotal - onlineAgents)} offline</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Aktif Cihaz</p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-slate-100">
            {healthyDevices}
            <span className="text-sm font-medium text-slate-600"> / {devices.length}</span>
          </p>
          <p className="mt-0.5 text-[10.5px] text-slate-600">SNMP + FortiGate</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Aktif Bağlantı</p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-slate-100">{formatNum(totalConns)}</p>
          <p className="mt-0.5 text-[10.5px] text-slate-600">agent filosu toplamı</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Olay Hızı</p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-slate-100">
            {eventRate.toFixed(1)}
            <span className="text-sm font-medium text-slate-600">/sn</span>
          </p>
          <p className="mt-0.5 text-[10.5px] text-slate-600">netflow + syslog</p>
        </div>
        <div className={`rounded-md border bg-slate-900/70 p-3.5 ${recentAlerts.length > 0 ? 'border-rose-500/30' : 'border-slate-800'}`}>
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Açık Uyarı</p>
          <p className={`mt-1.5 font-mono text-2xl font-bold ${alertEvents.length > 0 ? 'text-rose-400' : 'text-slate-100'}`}>
            {formatNum(alertEvents.length)}
          </p>
          <p className="mt-0.5 text-[10.5px] text-slate-600">bu oturumda</p>
        </div>
      </div>

      {/* agent filosu trafiği (fleet toplamı) */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div className="rounded-md border border-l-2 border-slate-800 border-l-cyan-500 bg-slate-900/70 p-4">
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">İndirilen Hız (Agent)</p>
          <p className="mt-1 font-mono text-2xl font-bold text-cyan-300">{formatBits(liveRxBps * 8)}</p>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-600">
            filo toplamı · gelen {fleet ? '· canlı' : ''}
          </p>
        </div>
        <div className="rounded-md border border-l-2 border-slate-800 border-l-violet-500 bg-slate-900/70 p-4">
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">Gönderilen Hız (Agent)</p>
          <p className="mt-1 font-mono text-2xl font-bold text-violet-300">{formatBits(liveTxBps * 8)}</p>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-600">
            filo toplamı · giden {fleet ? '· canlı' : ''}
          </p>
        </div>
        <div className="rounded-md border border-l-2 border-slate-800 border-l-emerald-500 bg-slate-900/70 p-4">
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">Toplam Veri (Agent)</p>
          <p className="mt-1 font-mono text-2xl font-bold text-emerald-300">{formatBytes(agentTraffic.totalBytes)}</p>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-600">arayüz sayaçları · kümülatif</p>
        </div>
        <div className="rounded-md border border-l-2 border-slate-800 border-l-amber-500 bg-slate-900/70 p-4">
          <p className="text-[10px] font-medium uppercase tracking-widest text-slate-500">Paket Hızı (Agent)</p>
          <p className="mt-1 font-mono text-2xl font-bold text-amber-300">{formatNum(Math.round(livePps))} pps</p>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-600">filo toplamı {fleet ? '· canlı' : ''}</p>
        </div>
      </div>

      {/* canlı trafik şeması — agent filosu ↔ router/güvenlik duvarı ↔ internet */}
      <Card
        title="Canlı Trafik Şeması"
        right={
          <span className="hidden text-xs text-slate-500 sm:inline">
            Agent filosu → Router/Güvenlik Duvarı → İnternet · animasyonlu paket akışı
          </span>
        }
      >
        <TrafficFlowDiagram events={diagramEvents} agents={diagramAgents} pps={livePps} />
      </Card>

      {/* canlı olay akışı — tam genişlik */}
      <Card
        title="Canlı Olay Akışı"
        right={
          <div className="flex items-center gap-3">
            <span className="hidden text-xs text-slate-500 sm:inline">Agent + NetFlow v5 + Syslog · en yeni üstte</span>
            <div className="flex items-center gap-1">
              {(['all', 'flow', 'agent', 'syslog'] as const).map((k) => {
                const active = streamFilter === k
                return (
                  <button
                    key={k}
                    type="button"
                    onClick={() => setStreamFilter(k)}
                    className={`rounded px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wider transition-colors ${
                      active ? 'bg-slate-700 text-slate-100' : 'text-slate-500 hover:text-slate-300'
                    }`}
                  >
                    {k === 'all' ? 'tümü' : k}
                    <span className={`ml-1 ${active ? 'opacity-60' : 'text-slate-600'}`}>
                      {k === 'all' ? stream.length : streamCounts[k]}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        }
      >
        {visibleStream.length === 0 ? (
          <p className="py-8 text-center text-sm text-slate-600">
            {stream.length === 0
              ? "Henüz akış yok — online agent bekleyin ya da cihazları NetFlow/Syslog için hub'a yönlendirin."
              : 'Bu türde henüz olay yok.'}
          </p>
        ) : (
          <div className="max-h-[32rem] space-y-0.5 overflow-y-auto">
            {visibleStream.map((it, i) => (
              <div
                key={it.key}
                className={`flex items-baseline gap-2.5 rounded px-2 py-1 font-mono text-[11px] hover:bg-slate-800/40 ${i % 2 === 1 ? 'bg-slate-800/15' : ''}`}
              >
                <span className="w-16 flex-shrink-0 text-right text-[10px] text-slate-600">{new Date(it.ts * 1000).toLocaleTimeString('tr-TR')}</span>
                <span className="hidden w-16 flex-shrink-0 text-[10px] text-slate-700 sm:inline">{relTime(it.ts)}</span>
                {it.kind === 'flow' && (
                  <span className="flex-shrink-0 rounded border border-cyan-500/30 bg-cyan-500/10 px-1.5 py-0.5 text-[9px] uppercase tracking-wider text-cyan-300">flow</span>
                )}
                {it.kind === 'agent' && (
                  <span className="flex-shrink-0 rounded border border-violet-500/30 bg-violet-500/10 px-1.5 py-0.5 text-[9px] uppercase tracking-wider text-violet-300">agent</span>
                )}
                {it.kind === 'syslog' && (
                  <span
                    className={`flex-shrink-0 rounded border px-1.5 py-0.5 text-[9px] uppercase tracking-wider ${
                      it.severity <= 3
                        ? 'border-rose-500/30 bg-rose-500/10 text-rose-400'
                        : 'border-amber-500/30 bg-amber-500/10 text-amber-400'
                    }`}
                  >
                    syslog
                  </span>
                )}
                <span className="min-w-0 flex-1 truncate text-slate-300">{it.primary}</span>
                {it.kind === 'flow' && it.bytes > 0 && (
                  <span className="hidden flex-shrink-0 text-[10px] text-slate-500 md:inline">
                    {formatBytes(it.bytes)} · {formatNum(it.packets)} pkt
                  </span>
                )}
                {it.kind === 'agent' && it.pid ? (
                  <span className="hidden flex-shrink-0 text-[10px] text-slate-700 md:inline">pid {it.pid}</span>
                ) : null}
                <span className="w-28 flex-shrink-0 truncate text-right text-[10px] text-slate-600">{it.source}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* uyarılar — akışın altında, tam genişlik */}
      <Card title="Uyarılar" right={<span className="text-xs text-slate-500">{formatNum(alertEvents.length)} olay · bu oturum</span>}>
        {recentAlerts.length === 0 ? (
          <p className="py-8 text-center text-sm text-slate-600">Henüz uyarı yok.</p>
        ) : (
          <div className="grid gap-1.5 sm:grid-cols-2 lg:grid-cols-4">
            {recentAlerts.map((e) => (
              <div key={e.id} className="rounded-md border border-slate-800 bg-slate-900/50 px-2.5 py-2">
                <div className="flex items-center gap-2">
                  <span className={`rounded border px-1.5 py-0.5 text-[10px] font-semibold ${ALERT_KIND_STYLES[e.kind] ?? 'border-slate-700 bg-slate-800 text-slate-400'}`}>
                    {ALERT_KIND_LABELS[e.kind] ?? e.kind}
                  </span>
                  <span className="ml-auto font-mono text-[10px] text-slate-600">{relTime(e.ts)}</span>
                </div>
                <p className="mt-1 truncate text-xs text-slate-300">{e.message}</p>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* agent filosu + topoloji */}
      <div className="grid gap-4 lg:grid-cols-[1.35fr_1fr]">
        <Card title="Agent Filosu" right={<span className="text-xs text-slate-500">{onlineAgents}/{agents.length} online</span>}>
          {agents.length === 0 ? (
            <p className="py-8 text-center text-sm text-slate-600">Henüz agent yok.</p>
          ) : (
            <div className="grid gap-2.5 sm:grid-cols-2">
              {agents.slice(0, 6).map((a) => {
                const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
                return (
                  <div key={a.id} className={`rounded-md border border-slate-800 bg-slate-900/50 p-2.5 ${!a.online ? 'opacity-60' : ''}`}>
                    <div className="flex items-center gap-2">
                      <span className={`size-1.5 flex-shrink-0 rounded-full ${a.online ? 'bg-emerald-400' : 'bg-slate-500'}`} />
                      <span className="truncate font-mono text-xs font-semibold text-slate-100">{a.name}</span>
                      {a.site && <span className="flex-shrink-0 rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[9px] text-slate-400">{a.site}</span>}
                      <span className="ml-auto flex-shrink-0 font-mono text-[9.5px] text-slate-600">{relTime(a.last_seen)}</span>
                    </div>
                    {busiest && (
                      <p className="mt-1.5 truncate font-mono text-[10.5px]">
                        <span className="text-cyan-300/90">↓ {formatBits(busiest.rx_bps * 8)}</span>
                        <span className="mx-1.5 text-slate-700">|</span>
                        <span className="text-violet-300/90">↑ {formatBits(busiest.tx_bps * 8)}</span>
                      </p>
                    )}
                  </div>
                )
              })}
            </div>
          )}
          {agents.length > 6 && <p className="mt-2 text-center text-[10.5px] text-slate-600">+{agents.length - 6} agent daha — tam liste aşağıda</p>}
        </Card>

        <Card title="Ağ Topolojisi" right={<span className="text-xs text-slate-500">LLDP/CDP/ARP</span>}>
          <TopologyCard refreshKey={refreshKey} />
        </Card>
      </div>

      {/* coğrafi trafik haritası */}
      <Card title="Coğrafi Trafik" right={<span className="text-xs text-slate-500">GeoIP · uzak uç noktalar</span>}>
        <GeoMapCard />
      </Card>

      {/* cihazlar */}
      <Card title="Cihazlar" right={<span className="text-xs text-slate-500">SNMP v2c/v3 · FortiGate REST API</span>}>
        {devices.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Cihaz yok.</p>
        ) : (
          <div className="space-y-1.5">
            {devices.map((d) => (
              <div key={d.id} className="flex flex-wrap items-center gap-2.5 rounded-md border border-slate-800 bg-slate-900/50 px-3 py-2">
                <span className={`size-1.5 flex-shrink-0 rounded-full ${d.enabled && !d.last_error ? 'bg-emerald-400' : 'bg-slate-500'}`} />
                <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[9.5px] uppercase text-slate-400">{d.kind}</span>
                <span className="font-mono text-xs font-semibold text-slate-100">{d.name}</span>
                <span className="font-mono text-[11px] text-slate-500">{d.host}</span>
                {d.vendor === 'fortigate' ? (
                  <span className="rounded border border-orange-500/40 bg-orange-500/10 px-1.5 py-0.5 font-mono text-[9px] uppercase text-orange-300">rest api</span>
                ) : (
                  <span className="rounded border border-slate-700 px-1.5 py-0.5 font-mono text-[9px] uppercase text-slate-500">snmp v{d.snmp_version === 3 ? '3' : '2c'}</span>
                )}
                <span className="ml-auto font-mono text-[10px] text-slate-600">
                  {d.last_poll > 0 ? `son poll: ${new Date(d.last_poll * 1000).toLocaleTimeString('tr-TR')}` : 'hiç poll edilmedi'}
                </span>
                {d.last_error && <span className="w-full truncate font-mono text-[10.5px] text-rose-400/80">⚠ {d.last_error}</span>}
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
