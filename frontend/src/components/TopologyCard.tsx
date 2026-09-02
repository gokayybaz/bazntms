import { useEffect, useMemo, useState } from 'react'

type TopoDevice = {
  id: number
  name: string
  host: string
  kind: string
  sys_name: string
  online: boolean
}
type TopoAgent = {
  id: number
  name: string
  site: string
  online: boolean
}
type TopoLink = {
  id: number
  ts: number
  kind: string
  source_type: string
  source_id: number
  source_name: string
  local_port: string
  peer_type: string
  peer_id: number
  peer_name: string
  peer_ip: string
}
type Graph = {
  generated_at: number
  devices: TopoDevice[]
  agents: TopoAgent[]
  links: TopoLink[]
}

const W = 880
const ROW_H = 58 // dugumler arasi dikey bosluk
const TOP_PAD = 70 // hub'in altindaki bosluk
const BOTTOM_PAD = 78 // depo dugumu + gosterge icin bosluk
const COL_GAP = 190 // hub'dan sutuna yatay uzaklik (bos taraf varsa tek sutun ortalanir)

function fmtAgo(ts: number) {
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts)
  if (s < 60) return `${s} sn`
  if (s < 3600) return `${Math.floor(s / 60)} dk`
  return `${Math.floor(s / 3600)} sa`
}

export function TopologyCard({ refreshKey }: { refreshKey: number }) {
  const [graph, setGraph] = useState<Graph | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/v1/topology')
      .then(async (r) => {
        if (!r.ok) throw new Error(await r.text())
        return r.json()
      })
      .then(setGraph)
      .catch((e) => setError(String(e.message || e)))
  }, [refreshKey])

  const layout = useMemo(() => {
    if (!graph) return null
    const devices = graph.devices
    const agents = graph.agents

    const rows = Math.max(devices.length, agents.length, 1)
    const H = TOP_PAD + rows * ROW_H + BOTTOM_PAD
    const HUB = { x: W / 2, y: 40 }

    // her iki taraf da doluysa iki sutun; yalnizca biri doluysa tek sutun
    // hub'in altinda ortalanir — bos taraf yuzunden kompozisyon carpik durmaz
    const bothSides = devices.length > 0 && agents.length > 0
    const colX = {
      devices: bothSides ? HUB.x - COL_GAP : HUB.x,
      agents: bothSides ? HUB.x + COL_GAP : HUB.x,
    }

    const devPos = new Map<number, { x: number; y: number }>()
    const agentPos = new Map<number, { x: number; y: number }>()

    const colTop = (n: number) => TOP_PAD + Math.max(0, (rows - n) * ROW_H) / 2 + ROW_H / 2
    devices.forEach((d, i) => devPos.set(d.id, { x: colX.devices, y: colTop(devices.length) + i * ROW_H }))
    agents.forEach((a, i) => agentPos.set(a.id, { x: colX.agents, y: colTop(agents.length) + i * ROW_H }))

    // kenar cözümleme: peer adı/ip → cihaz veya agent düğümü
    const peer_ip_of = (peer: string) => {
      const m = peer.match(/\((\d+\.\d+\.\d+\.\d+)\)/)
      return m ? m[1] : ''
    }
    const nameMatch = (peer: string, d: TopoDevice) =>
      !!peer &&
      (d.sys_name === peer ||
        d.name === peer ||
        d.host === peer ||
        peer.startsWith(d.sys_name + ' ') ||
        d.host === peer_ip_of(peer))

    type Edge = { x1: number; y1: number; x2: number; y2: number; kind: string; label: string }
    const discoveredEdges: Edge[] = []
    const hosts: { x: number; y: number; label: string; kind: string; source: string; ts: number }[] = []

    for (const l of graph.links) {
      if (l.kind === 'subnet') continue // subnet linkleri artik ayri "cozumlenmemis" liste yerine spoke altinda dolayli gosterilir
      if (l.source_type !== 'device') continue
      const src = devPos.get(l.source_id)
      if (!src) continue
      // peer bir cihaz mi?
      let dst: { x: number; y: number } | undefined
      let dstId = -1
      for (const d of devices) {
        if (nameMatch(l.peer_name, d) || (l.peer_ip && l.peer_ip === d.host)) {
          dst = devPos.get(d.id)
          dstId = d.id
          break
        }
      }
      if (dst && dstId !== l.source_id) {
        discoveredEdges.push({ x1: src.x, y1: src.y, x2: dst.x, y2: dst.y, kind: l.kind, label: l.local_port || '' })
      } else if (l.kind === 'arp') {
        const j = hosts.filter((h) => h.source === String(l.source_id)).length
        const side = src.x < HUB.x ? -1 : 1
        hosts.push({
          x: src.x + side * (44 + (j % 3) * 12),
          y: src.y - 14 + Math.floor(j / 3) * 11,
          label: l.peer_ip,
          kind: 'arp',
          source: String(l.source_id),
          ts: l.ts,
        })
      } else {
        const j = hosts.filter((h) => h.source === String(l.source_id)).length
        const side = src.x < HUB.x ? -1 : 1
        hosts.push({
          x: src.x + side * (44 + (j % 3) * 12),
          y: src.y - 14 + Math.floor(j / 3) * 11,
          label: l.peer_name.split(' ')[0] || l.peer_name,
          kind: l.kind,
          source: String(l.source_id),
          ts: l.ts,
        })
      }
    }

    // depo dugumu: hub'in her zaman baglandigi tek sabit nokta (yonetim
    // katmani artik havada asili kalmiyor)
    const storage = { x: HUB.x, y: H - BOTTOM_PAD + 28 }

    return { devPos, agentPos, discoveredEdges, hosts, devices, agents, H, HUB, storage }
  }, [graph])

  if (error) {
    return <p className="text-xs text-rose-400">{error}</p>
  }
  if (!graph || !layout) {
    return <p className="text-xs text-slate-500">yükleniyor…</p>
  }

  const edgeColor = (kind: string) =>
    kind === 'lldp' ? '#34d399' : kind === 'cdp' ? '#38bdf8' : kind === 'subnet' ? '#a78bfa' : '#475569'

  const { H, HUB, storage } = layout

  return (
    <div>
      {graph.devices.length === 0 && graph.agents.length === 0 ? (
        <p className="text-xs text-slate-500">
          Topoloji boş — cihaz ekleyin (SNMP LLDP/CDP/ARP keşfi) veya agent kurun; yerel ağlar otomatik haritaya işlenir.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <svg viewBox={`0 0 ${W} ${H}`} className="min-w-[640px]">
            {/* temel spoke hatlari: hub her zaman tum dugumlere baglidir —
                LLDP/CDP/subnet verisi olmasa bile diyagram baglantili gorunur */}
            <g stroke="#1e293b" strokeWidth={1.3}>
              {layout.devices.map((d) => {
                const p = layout.devPos.get(d.id)!
                return <line key={`sd${d.id}`} x1={HUB.x} y1={HUB.y} x2={p.x} y2={p.y} />
              })}
              {layout.agents.map((a) => {
                const p = layout.agentPos.get(a.id)!
                return <line key={`sa${a.id}`} x1={HUB.x} y1={HUB.y} x2={p.x} y2={p.y} />
              })}
              <line x1={HUB.x} y1={HUB.y} x2={storage.x} y2={storage.y} strokeDasharray="3 4" />
            </g>

            {/* kesif ile bulunan zengin baglantilar (LLDP/CDP) — spoke'un ustune */}
            {layout.discoveredEdges.map((e, i) => (
              <g key={i}>
                <line x1={e.x1} y1={e.y1} x2={e.x2} y2={e.y2} stroke={edgeColor(e.kind)} strokeWidth={1.6} strokeOpacity={0.7} />
                {e.label && (
                  <text x={(e.x1 + e.x2) / 2} y={(e.y1 + e.y2) / 2 - 4} textAnchor="middle" className="fill-slate-500" fontSize={8}>
                    {e.label}
                  </text>
                )}
              </g>
            ))}

            {/* hub */}
            <circle cx={HUB.x} cy={HUB.y} r={22} className="fill-slate-900 stroke-cyan-500" strokeWidth={1.5} />
            <text x={HUB.x} y={HUB.y + 3.5} textAnchor="middle" className="fill-cyan-400" fontSize={9} fontFamily="monospace">
              HUB
            </text>

            {/* depo dugumu */}
            <rect x={storage.x - 54} y={storage.y - 12} width={108} height={24} rx={5} className="fill-slate-900 stroke-slate-700" strokeWidth={1.2} />
            <text x={storage.x} y={storage.y + 3.5} textAnchor="middle" className="fill-slate-400" fontSize={8.5} fontFamily="monospace">
              depolama
            </text>

            {/* cihazlar */}
            {layout.devices.map((d) => {
              const p = layout.devPos.get(d.id)!
              const onRight = p.x > HUB.x
              return (
                <g key={`d${d.id}`}>
                  <circle cx={p.x} cy={p.y} r={7} className={d.online ? 'fill-emerald-500/80' : 'fill-slate-600'} />
                  <text
                    x={onRight ? p.x + 14 : p.x - 14}
                    y={p.y + 1}
                    textAnchor={onRight ? 'start' : 'end'}
                    className="fill-slate-300"
                    fontSize={9.5}
                    fontFamily="monospace"
                  >
                    {d.name.length > 18 ? d.name.slice(0, 17) + '…' : d.name}
                  </text>
                  <text
                    x={onRight ? p.x + 14 : p.x - 14}
                    y={p.y + 12.5}
                    textAnchor={onRight ? 'start' : 'end'}
                    className="fill-slate-600"
                    fontSize={8}
                  >
                    {d.kind} · {d.host}
                  </text>
                </g>
              )
            })}

            {/* agentlar */}
            {layout.agents.map((a) => {
              const p = layout.agentPos.get(a.id)!
              const onRight = p.x >= HUB.x
              return (
                <g key={`a${a.id}`}>
                  <circle cx={p.x} cy={p.y} r={7} className={a.online ? 'fill-cyan-400' : 'fill-slate-600'} />
                  <text
                    x={onRight ? p.x + 14 : p.x - 14}
                    y={p.y + 1}
                    textAnchor={onRight ? 'start' : 'end'}
                    className="fill-slate-300"
                    fontSize={9.5}
                    fontFamily="monospace"
                  >
                    {a.name.length > 18 ? a.name.slice(0, 17) + '…' : a.name}
                  </text>
                  <text
                    x={onRight ? p.x + 14 : p.x - 14}
                    y={p.y + 12.5}
                    textAnchor={onRight ? 'start' : 'end'}
                    className="fill-slate-600"
                    fontSize={8}
                  >
                    {a.online ? 'online' : 'offline'}
                    {a.site ? ` · ${a.site}` : ''}
                  </text>
                </g>
              )
            })}

            {/* cözümlenmemis komsular (ARP/LLDP uçları) */}
            {layout.hosts.map((h, i) => (
              <circle key={i} cx={h.x} cy={h.y} r={2.2} fill={edgeColor(h.kind)} fillOpacity={0.8}>
                <title>
                  {h.label} · {h.kind} · {fmtAgo(h.ts)} önce görüldü
                </title>
              </circle>
            ))}

            {/* gosterge */}
            <g transform={`translate(${W / 2 - 190}, ${H - 14})`}>
              <circle cx={0} cy={-3} r={3.4} fill="#34d399" />
              <text x={8} y={0} className="fill-slate-500" fontSize={8.5}>LLDP</text>
              <circle cx={48} cy={-3} r={3.4} fill="#38bdf8" />
              <text x={56} y={0} className="fill-slate-500" fontSize={8.5}>CDP</text>
              <circle cx={96} cy={-3} r={3.4} fill="#a78bfa" />
              <text x={104} y={0} className="fill-slate-500" fontSize={8.5}>subnet</text>
              <circle cx={156} cy={-3} r={3.4} fill="#475569" />
              <text x={164} y={0} className="fill-slate-500" fontSize={8.5}>ARP ucu</text>
              <line x1={220} y1={-3} x2={236} y2={-3} stroke="#1e293b" strokeWidth={1.3} />
              <text x={240} y={0} className="fill-slate-600" fontSize={8.5}>hub bağlantısı</text>
            </g>
          </svg>
        </div>
      )}
    </div>
  )
}
