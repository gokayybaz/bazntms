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
const H = 340
const HUB = { x: W / 2, y: 44 }

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

    const colX = { devices: 110, agents: W - 110 }
    const devPos = new Map<number, { x: number; y: number }>()
    const agentPos = new Map<number, { x: number; y: number }>()

    devices.forEach((d, i) => {
      const n = devices.length
      const y = n === 1 ? H / 2 : 70 + i * ((H - 140) / (n - 1))
      devPos.set(d.id, { x: colX.devices, y })
    })
    agents.forEach((a, i) => {
      const n = agents.length
      const y = n === 1 ? H / 2 : 70 + i * ((H - 140) / (n - 1))
      agentPos.set(a.id, { x: colX.agents, y })
    })

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
    const edges: Edge[] = []
    const hosts: { x: number; y: number; label: string; kind: string; source: string }[] = []

    for (const l of graph.links) {
      if (l.kind === 'subnet') {
        const p = agentPos.get(l.source_id)
        if (p) edges.push({ x1: p.x, y1: p.y, x2: p.x + 46, y2: p.y + 12, kind: 'subnet', label: l.peer_ip })
        continue
      }
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
        edges.push({
          x1: src.x, y1: src.y, x2: dst.x, y2: dst.y,
          kind: l.kind, label: l.local_port || '',
        })
      } else if (l.kind === 'arp') {
        // cözümlenmemis ARP uç noktaları: cihaz yanina küçük konum
        const j = hosts.length
        hosts.push({
          x: src.x + 52 + (j % 3) * 14, y: src.y - 16 + Math.floor(j / 3) * 10,
          label: l.peer_ip, kind: 'arp', source: String(l.source_id),
        })
      } else {
        // LLDP/CDP komsusu: cihaz yanina isaret
        const j = hosts.length
        hosts.push({
          x: src.x + 52 + (j % 3) * 14, y: src.y - 16 + Math.floor(j / 3) * 10,
          label: l.peer_name.split(' ')[0] || l.peer_name, kind: l.kind, source: String(l.source_id),
        })
      }
    }

    return { devPos, agentPos, edges, hosts, devices, agents }
  }, [graph])

  if (error) {
    return <p className="text-xs text-rose-400">{error}</p>
  }
  if (!graph || !layout) {
    return <p className="text-xs text-slate-500">yükleniyor…</p>
  }

  const edgeColor = (kind: string) =>
    kind === 'lldp' ? '#34d399' : kind === 'cdp' ? '#38bdf8' : kind === 'subnet' ? '#a78bfa' : '#475569'

  return (
    <div>
      {graph.devices.length === 0 && graph.agents.length === 0 ? (
        <p className="text-xs text-slate-500">
          Topoloji boş — cihaz ekleyin (SNMP LLDP/CDP/ARP keşfi) veya agent kurun; yerel ağlar otomatik haritaya işlenir.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <svg viewBox={`0 0 ${W} ${H}`} className="min-w-[640px]">
            {/* hatlar */}
            {layout.edges.map((e, i) => (
              <g key={i}>
                <line
                  x1={e.x1} y1={e.y1} x2={e.x2} y2={e.y2}
                  stroke={edgeColor(e.kind)} strokeWidth={1.4} strokeOpacity={0.55}
                />
                {e.kind !== 'subnet' && e.label && (
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
            <line x1={HUB.x} y1={HUB.y + 22} x2={HUB.x} y2={H - 24} stroke="#334155" strokeDasharray="3 4" />
            <text x={HUB.x + 8} y={H - 30} className="fill-slate-600" fontSize={8.5}>
              yönetim katmanı
            </text>

            {/* cihazlar */}
            {layout.devices.map((d) => {
              const p = layout.devPos.get(d.id)!
              return (
                <g key={`d${d.id}`}>
                  <circle
                    cx={p.x} cy={p.y} r={7}
                    className={d.online ? 'fill-emerald-500/80' : 'fill-slate-600'}
                  />
                  <text x={p.x - 14} y={p.y + 3.5} textAnchor="end" className="fill-slate-300" fontSize={9.5} fontFamily="monospace">
                    {d.name.length > 18 ? d.name.slice(0, 17) + '…' : d.name}
                  </text>
                  <text x={p.x - 14} y={p.y + 14} textAnchor="end" className="fill-slate-600" fontSize={8}>
                    {d.kind} · {d.host}
                  </text>
                </g>
              )
            })}

            {/* agentlar */}
            {layout.agents.map((a) => {
              const p = layout.agentPos.get(a.id)!
              return (
                <g key={`a${a.id}`}>
                  <circle
                    cx={p.x} cy={p.y} r={7}
                    className={a.online ? 'fill-cyan-400' : 'fill-slate-600'}
                  />
                  <text x={p.x + 14} y={p.y + 3.5} className="fill-slate-300" fontSize={9.5} fontFamily="monospace">
                    {a.name.length > 18 ? a.name.slice(0, 17) + '…' : a.name}
                  </text>
                  <text x={p.x + 14} y={p.y + 14} className="fill-slate-600" fontSize={8}>
                    {a.online ? 'online' : 'offline'}{a.site ? ` · ${a.site}` : ''}
                  </text>
                </g>
              )
            })}

            {/* cözümlenmemis komsular */}
            {layout.hosts.map((h, i) => (
              <circle key={i} cx={h.x} cy={h.y} r={2.2} fill={edgeColor(h.kind)} fillOpacity={0.8}>
                <title>{h.label} · {h.kind} · {fmtAgo(graph.generated_at)}</title>
              </circle>
            ))}

            {/* gosterge */}
            <g transform={`translate(12, ${H - 12})`}>
              <circle cx={0} cy={-3} r={3.4} fill="#34d399" />
              <text x={8} y={0} className="fill-slate-500" fontSize={8.5}>LLDP</text>
              <circle cx={48} cy={-3} r={3.4} fill="#38bdf8" />
              <text x={56} y={0} className="fill-slate-500" fontSize={8.5}>CDP</text>
              <circle cx={96} cy={-3} r={3.4} fill="#a78bfa" />
              <text x={104} y={0} className="fill-slate-500" fontSize={8.5}>subnet</text>
              <circle cx={156} cy={-3} r={3.4} fill="#475569" />
              <text x={164} y={0} className="fill-slate-500" fontSize={8.5}>ARP ucu</text>
            </g>
          </svg>
        </div>
      )}
    </div>
  )
}
