import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import type { AgentWithRates, AlertEvent } from '../types'
import type { FleetSummary } from '../lib/useLive'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { usePeak } from '../lib/usePeak'
import { Panel } from './Panel'
import { Meter } from './Meter'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'
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

// DESIGN.md'nin renk-anlam sözleşmesi dışına çıkılmıştı (impeccable
// critique P1): ioc sözleşme-dışı bir "red" kullanıyordu (rose zaten
// "kritik alarm" için ayrılmışken) ve target, violet'i — sözleşmenin asla
// tek başına birincil vurgu olarak kullanılmamasını söylediği rengi —
// tek başına taşıyordu. ioc → rose (en kritik uyarı, kritik-alarm rengi);
// target → amber (bw ile aynı "eşik/davranışsal uyarı" katmanı, yeni renk
// icat edilmedi).
const ALERT_KIND_STYLES: Record<string, string> = {
  bw: 'border-amber-500/30 bg-amber-500/10 text-amber-400',
  port: 'border-rose-500/30 bg-rose-500/10 text-rose-400',
  proc: 'border-sky-500/30 bg-sky-500/10 text-sky-400',
  target: 'border-amber-500/30 bg-amber-500/10 text-amber-400',
  ioc: 'border-rose-500/40 bg-rose-500/15 text-rose-300',
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

// Filo Özeti paneli için tek satır: ETİKET  değer  (açıklama)
function StatRow({
  label,
  value,
  caption,
  tone,
  live,
}: {
  label: string
  value: string
  caption: string
  tone?: 'rose'
  live?: boolean
}) {
  return (
    <div className="min-w-0">
      <dt className="flex items-center gap-1 truncate text-[10px] uppercase tracking-[0.04em] text-tui-dim">
        {label}
        {live && <span className="text-emerald-400" title="WS canlı (1 sn)">●</span>}
      </dt>
      <dd className={`truncate text-[13px] font-bold ${tone === 'rose' ? 'text-rose-400' : 'text-ink-hi'}`}>{value}</dd>
      <dd className="truncate text-[10px] text-tui-dim">{caption}</dd>
    </div>
  )
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
  const navigate = useNavigate()
  const [agents, setAgents] = useState<AgentWithRates[]>([])
  const [devices, setDevices] = useState<Device[]>([])
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [syslog, setSyslog] = useState<SyslogEvent[]>([])

  // hangi veri kaynaklarının son yoklaması başarısız oldu — önceden her
  // fetch hatası sessizce yutuluyordu, hub çökse/oturum düşse operatör
  // "canlı" görünen ama aslında bayat sayılara güvenebiliyordu (impeccable
  // critique P0). key → insan-okunur kaynak adı; başarıyla anahtar silinir.
  const [staleSources, setStaleSources] = useState<Record<string, string>>({})
  const markSource = (key: string, label: string, ok: boolean) =>
    setStaleSources((prev) => {
      if (ok) {
        if (!(key in prev)) return prev
        const next = { ...prev }
        delete next[key]
        return next
      }
      return prev[key] === label ? prev : { ...prev, [key]: label }
    })

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/agents')
        if (res.status === 401) return
        if (!res.ok) return markSource('agents', 'agent listesi', false)
        if (!stop) {
          setAgents(await res.json())
          markSource('agents', 'agent listesi', true)
        }
      } catch {
        markSource('agents', 'agent listesi', false)
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
        if (!res.ok) return markSource('devices', 'cihaz listesi', false)
        if (!stop) {
          setDevices(await res.json())
          markSource('devices', 'cihaz listesi', true)
        }
      } catch {
        markSource('devices', 'cihaz listesi', false)
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
        if (!res.ok) return markSource('flows', 'akış (NetFlow)', false)
        if (!stop) {
          setFlows(await res.json())
          markSource('flows', 'akış (NetFlow)', true)
        }
      } catch {
        markSource('flows', 'akış (NetFlow)', false)
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
        if (!res.ok) return markSource('syslog', 'syslog', false)
        if (!stop) {
          setSyslog(await res.json())
          markSource('syslog', 'syslog', true)
        }
      } catch {
        markSource('syslog', 'syslog', false)
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
        if (!stop) {
          setAgentConns(results.filter((r): r is { agentName: string; ts: number; conns: AgentConnSample[] } => r !== null))
          markSource('agentConns', 'agent bağlantı envanteri', true)
        }
      } catch {
        markSource('agentConns', 'agent bağlantı envanteri', false)
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
  // agent sayısı: /api/v1/agents REST listesi TEK kaynak — canlı trafik şeması,
  // topoloji ve alttaki filo kartları da aynı listeyi kullanır. WS fleet sayacı
  // ~4 sn daha taze ama AYRI bir örnek olduğu için "43 aktif" derken şema
  // "4 aktif" gösterebiliyordu (aynı online penceresi, farklı örnekleme anı).
  // İlk REST poll gelene kadar fleet'e düş. (rx/tx/pps aşağıda hâlâ WS'ten.)
  const onlineAgents = agents.length > 0 ? agents.filter((a) => a.online).length : (fleet?.agents_online ?? 0)
  const agentsTotal = agents.length > 0 ? agents.length : (fleet?.agents_total ?? 0)
  const totalConns = agents.reduce((sum, a) => sum + (a.conns || 0), 0)
  // cihaz icin ayri bir "online" alani yok: hata vermeden calisiyor olmasi
  // saglikli kabul edilir. last_poll>0 sarti eskiden buradaydi ama yeni
  // eklenmis, ilk poll'unu henuz almamis bir cihazi da "sagliksiz" sayiyordu
  // (impeccable critique 2026-09-05, DeviceDetailPage.tsx'teki ayni hatanin
  // eslenigi) — kaldirildi, enabled+hatasiz olmasi yeterli
  const healthyDevices = devices.filter((d) => d.enabled && !d.last_error).length

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
  // log-tail klavye gezinme imleci (↑↓/jk) — liste yeniden sıralanınca 0'a çeker
  const [logSel, setLogSel] = useState(0)
  const [logFocused, setLogFocused] = useState(false)
  // olay akışındaki agent satırlarını agent detay sayfasına bağlamak için —
  // önceden hiçbir satır tıklanamıyordu (impeccable critique P3, Alex
  // persona: şüpheli bir IP görüp agent'a geçmek için sidebar'dan manuel
  // arama gerekiyordu)
  const agentIdByName = useMemo(() => new Map(agents.map((a) => [a.name, a.id])), [agents])

  // canlı trafik şeması: yalnızca ÇEVRİMİÇİ agent'lar düğüm olur (süzme bileşen
  // içinde) — kapalı agent trafik üretmez. Tüm filo yine de geçilir ki bileşen
  // "N çevrimdışı gizli" ipucunu gösterebilsin; sıralama online-first.
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

  // Meter ölçekleri: oturum-içi tepe (throughput'un sabit üst sınırı yok)
  const rxPeak = usePeak(liveRxBps * 8)
  const txPeak = usePeak(liveTxBps * 8)
  const ppsPeak = usePeak(livePps)

  const staleList = Object.values(staleSources)

  const alertsTotal = alertEvents.length

  const deviceCols: TuiColumn<Device>[] = [
    {
      key: 'st',
      header: '',
      width: '2.2rem',
      render: (d) => <span className={d.enabled && !d.last_error ? 'text-emerald-400' : 'text-tui-dim'}>{d.enabled && !d.last_error ? '●' : '○'}</span>,
    },
    { key: 'kind', header: 'Tür', width: '5rem', sortable: true, render: (d) => <span className="uppercase text-tui-dim">{d.kind}</span> },
    { key: 'name', header: 'Ad', sortable: true, render: (d) => <span className="font-semibold text-ink-hi">{d.name}</span> },
    { key: 'host', header: 'Host', sortable: true, render: (d) => <span className="text-tui-dim">{d.host}</span> },
    {
      key: 'src',
      header: 'Kaynak',
      width: '6rem',
      render: (d) =>
        d.vendor === 'fortigate' ? (
          <span className="text-orange-300">rest api</span>
        ) : (
          <span className="text-tui-dim">snmp v{d.snmp_version === 3 ? '3' : '2c'}</span>
        ),
    },
    {
      key: 'poll',
      header: 'Son Poll',
      align: 'right',
      sortValue: (d) => d.last_poll,
      render: (d) => (
        <span className={d.last_error ? 'text-rose-400' : 'text-tui-dim'}>
          {d.last_error ? `⚠ ${d.last_error}` : d.last_poll > 0 ? new Date(d.last_poll * 1000).toLocaleTimeString('tr-TR') : '—'}
        </span>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      {/* bağlantı sorunu şeridi — aşağıdaki panellerin "canlı" görünüp aslında
          bayat veri gösterme riskini ortadan kaldırır (impeccable critique P0) */}
      {staleList.length > 0 && (
        <div className="border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 font-mono text-[11px] text-rose-300">
          ⚠ Bağlantı sorunu — {staleList.join(', ')} güncellenemiyor, gösterilen veriler bayat olabilir.
        </div>
      )}

      <div className="grid gap-3 lg:grid-cols-[1fr_1.1fr]">
        {/* filo özeti — sayaç satırları (htop üst panel dili) */}
        <Panel title="Filo Özeti">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 font-mono text-[11px]">
            <StatRow label="Aktif Agent" value={`${onlineAgents}/${agentsTotal}`} caption={`${Math.max(0, agentsTotal - onlineAgents)} offline`} live={!!fleet} />
            <StatRow label="Aktif Cihaz" value={`${healthyDevices}/${devices.length}`} caption="snmp + fortigate" />
            <StatRow label="Bağlantı" value={formatNum(totalConns)} caption="agent filosu toplamı" />
            <StatRow label="Olay Hızı" value={`${eventRate.toFixed(1)}/sn`} caption="netflow + syslog" />
            <StatRow label="Açık Uyarı" value={formatNum(alertsTotal)} caption="bu oturumda" tone={alertsTotal > 0 ? 'rose' : undefined} />
          </dl>
        </Panel>

        {/* agent filosu trafiği — Meter bandı */}
        <Panel title="Agent Trafiği" right={<span className="font-mono text-[10px] text-tui-dim">{fleet ? 'canlı · 1 sn' : 'poll · 5 sn'}</span>}>
          <div className="space-y-2">
            <Meter label="RX" value={liveRxBps * 8} max={rxPeak} accent="rx" display={formatBits(liveRxBps * 8)} width={28} />
            <Meter label="TX" value={liveTxBps * 8} max={txPeak} accent="tx" display={formatBits(liveTxBps * 8)} width={28} />
            <Meter label="PPS" value={livePps} max={ppsPeak} display={`${formatNum(Math.round(livePps))} pps`} width={28} />
            <div className="flex items-center gap-2 font-mono text-[11px]">
              <span className="w-12 shrink-0 uppercase tracking-[0.04em] text-tui-dim">VERİ</span>
              <span className="text-emerald-400">{formatBytes(agentTraffic.totalBytes)}</span>
              <span className="ml-auto text-tui-dim">arayüz sayaçları · kümülatif</span>
            </div>
          </div>
        </Panel>
      </div>

      {/* canlı trafik şeması — agent filosu ↔ router/güvenlik duvarı ↔ internet */}
      <Panel
        title="Canlı Trafik Şeması"
        right={
          <span className="hidden font-mono text-[10px] text-tui-dim sm:inline">
            agent filosu → router/güvenlik duvarı → internet
          </span>
        }
      >
        <TrafficFlowDiagram events={diagramEvents} agents={diagramAgents} />
      </Panel>

      {/* canlı olay akışı — log-tail */}
      <Panel
        title="Canlı Olay Akışı"
        right={
          <div className="flex items-center gap-1">
            {(['all', 'flow', 'agent', 'syslog'] as const).map((k) => {
              const active = streamFilter === k
              return (
                <button
                  key={k}
                  type="button"
                  onClick={() => {
                    setStreamFilter(k)
                    setLogSel(0)
                  }}
                  className={`px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] transition ${
                    active ? 'bg-rx text-ground' : 'text-tui-dim hover:text-ink-hi'
                  }`}
                >
                  {k === 'all' ? 'tümü' : k}
                  <span className={`ml-1 ${active ? 'opacity-70' : 'text-tui-dim'}`}>{k === 'all' ? stream.length : streamCounts[k]}</span>
                </button>
              )
            })}
          </div>
        }
      >
        {visibleStream.length === 0 ? (
          <p className="py-8 text-center font-mono text-[11px] text-tui-dim">
            {stream.length === 0
              ? "Henüz akış yok — online agent bekleyin ya da cihazları NetFlow/Syslog için hub'a yönlendirin."
              : 'Bu türde henüz olay yok.'}
          </p>
        ) : (
          <div
            tabIndex={0}
            role="log"
            aria-live="polite"
            aria-label="Canlı olay akışı"
            onFocus={() => setLogFocused(true)}
            onBlur={() => setLogFocused(false)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowDown' || e.key === 'j') {
                setLogSel((s) => Math.min(s + 1, visibleStream.length - 1))
                e.preventDefault()
              } else if (e.key === 'ArrowUp' || e.key === 'k') {
                setLogSel((s) => Math.max(s - 1, 0))
                e.preventDefault()
              } else if (e.key === 'Enter') {
                const it = visibleStream[Math.min(logSel, visibleStream.length - 1)]
                const aid = it && it.kind === 'agent' ? agentIdByName.get(it.source) : undefined
                if (aid !== undefined) {
                  navigate(`/agentlar/${aid}`)
                  e.preventDefault()
                }
              }
            }}
            className="max-h-[28rem] overflow-y-auto font-mono text-[11px] outline-none focus-visible:ring-1 focus-visible:ring-rx/40"
          >
            {visibleStream.map((it, i) => {
              const agentId = it.kind === 'agent' ? agentIdByName.get(it.source) : undefined
              const clickable = agentId !== undefined
              // seçim yalnızca log odaktayken reverse-video; odak dışında sade
              const sel = logFocused && i === Math.min(logSel, visibleStream.length - 1)
              const kindCls = it.kind === 'flow' ? 'text-rx' : it.kind === 'agent' ? 'text-tx' : it.severity <= 3 ? 'text-rose-400' : 'text-amber-400'
              return (
                <div
                  key={it.key}
                  ref={sel ? (el) => el?.scrollIntoView({ block: 'nearest' }) : undefined}
                  onClick={clickable ? () => navigate(`/agentlar/${agentId}`) : undefined}
                  title={clickable ? `${it.source} agent detayına git` : undefined}
                  className={`flex items-baseline gap-2 px-2 py-0.5 ${
                    sel ? 'bg-rule-hi/50 text-ink-hi' : i % 2 ? 'bg-panel-2/40 text-ink' : 'text-ink'
                  } ${clickable ? 'cursor-pointer' : ''} ${!sel ? 'hover:bg-panel-2' : ''}`}
                >
                  <span className="w-16 shrink-0 text-right text-tui-dim">
                    {new Date(it.ts * 1000).toLocaleTimeString('tr-TR')}
                  </span>
                  <span className={`w-14 shrink-0 uppercase tracking-[0.04em] ${kindCls}`}>{it.kind}</span>
                  <span className="min-w-0 flex-1 truncate">{it.primary}</span>
                  {it.kind === 'flow' && it.bytes > 0 && (
                    <span className={`hidden shrink-0 text-[10px] md:inline text-tui-dim`}>
                      {formatBytes(it.bytes)} · {formatNum(it.packets)} pkt
                    </span>
                  )}
                  {it.kind === 'agent' && it.pid ? (
                    <span className={`hidden shrink-0 text-[10px] md:inline text-tui-dim`}>pid {it.pid}</span>
                  ) : null}
                  <span className={`w-28 shrink-0 truncate text-right text-[10px] text-tui-dim`}>{it.source}</span>
                </div>
              )
            })}
          </div>
        )}
      </Panel>

      {/* uyarılar — akışın altında, tam genişlik */}
      <Panel title="Uyarılar" right={<span className="font-mono text-[10px] text-tui-dim">{formatNum(alertsTotal)} olay · bu oturum</span>}>
        {recentAlerts.length === 0 ? (
          <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Henüz uyarı yok.</p>
        ) : (
          <div className="grid gap-1 sm:grid-cols-2 lg:grid-cols-4">
            {recentAlerts.map((e) => (
              <div key={e.id} className="border border-rule bg-panel-2/40 px-2 py-1.5">
                <div className="flex items-center gap-2">
                  <span className={`border px-1 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] ${ALERT_KIND_STYLES[e.kind] ?? 'border-rule-hi text-tui-dim'}`}>
                    {ALERT_KIND_LABELS[e.kind] ?? e.kind}
                  </span>
                  <span className="ml-auto font-mono text-[10px] text-tui-dim">{relTime(e.ts)}</span>
                </div>
                <p className="mt-1 truncate font-mono text-[11px] text-ink">{e.message}</p>
              </div>
            ))}
          </div>
        )}
      </Panel>

      {/* agent filosu + topoloji */}
      <div className="grid gap-3 lg:grid-cols-[1.35fr_1fr]">
        <Panel title="Agent Filosu" right={<span className="font-mono text-[10px] text-tui-dim">{onlineAgents}/{agents.length} online</span>}>
          {agents.length === 0 ? (
            <p className="py-8 text-center font-mono text-[11px] text-tui-dim">Henüz agent yok.</p>
          ) : (
            <div className="space-y-0.5 font-mono text-[11px]">
              {agents.slice(0, 6).map((a) => {
                const busiest = [...(a.rates ?? [])].sort((x, y) => y.rx_bps + y.tx_bps - (x.rx_bps + x.tx_bps))[0]
                return (
                  <div key={a.id} className={`flex items-baseline gap-2 px-1 py-0.5 ${!a.online ? 'opacity-60' : ''}`}>
                    <span className={a.online ? 'text-emerald-400' : 'text-tui-dim'}>{a.online ? '●' : '○'}</span>
                    <Link to={`/agentlar/${a.id}`} className="shrink-0 truncate font-semibold text-rx hover:underline">
                      {a.name}
                    </Link>
                    {a.site && <span className="shrink-0 text-tui-dim">{a.site}</span>}
                    {busiest && (
                      <span className="ml-auto shrink-0">
                        <span className="text-rx">↓{formatBits(busiest.rx_bps * 8)}</span>
                        <span className="mx-1 text-rule-hi">|</span>
                        <span className="text-tx">↑{formatBits(busiest.tx_bps * 8)}</span>
                      </span>
                    )}
                    <span className="w-16 shrink-0 text-right text-[10px] text-tui-dim">{relTime(a.last_seen)}</span>
                  </div>
                )
              })}
            </div>
          )}
          {agents.length > 6 && (
            <p className="mt-2 font-mono text-[10px] text-tui-dim">
              +{agents.length - 6} agent daha —{' '}
              <Link to="/agentlar" className="text-rx hover:underline">
                tam liste →
              </Link>
            </p>
          )}
        </Panel>

        <Panel title="Ağ Topolojisi" right={<span className="font-mono text-[10px] text-tui-dim">LLDP/CDP/ARP</span>}>
          <TopologyCard refreshKey={refreshKey} />
        </Panel>
      </div>

      {/* coğrafi trafik haritası */}
      <Panel title="Coğrafi Trafik" right={<span className="font-mono text-[10px] text-tui-dim">netflow + agent · geoip</span>}>
        <GeoMapCard />
      </Panel>

      {/* cihazlar */}
      <Panel title="Cihazlar" right={<span className="font-mono text-[10px] text-tui-dim">snmp v2c/v3 · fortigate rest</span>} bodyClassName="">
        {devices.length === 0 ? (
          <p className="py-6 text-center font-mono text-[11px] text-tui-dim">Cihaz yok.</p>
        ) : (
          <TuiTable
            columns={deviceCols}
            rows={devices}
            getKey={(d) => String(d.id)}
            onActivate={(d) => navigate(`/cihazlar/${d.id}`)}
            filterText={(d) => `${d.name} ${d.host} ${d.kind} ${d.vendor}`}
            filterLabel="Cihaz filtrele…"
            initialSort={{ key: 'name', dir: 'asc' }}
            scrollClass="max-h-[22rem]"
            className="border-0"
          />
        )}
      </Panel>
    </div>
  )
}
