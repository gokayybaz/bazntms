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

// --- yatay sahne: client (agent) ▸ HUB ▸ keşfedilen cihazlar ▸ ROUTER ▸ İNTERNET ---
const W = 1060
const ROW_H = 52 // sütun içi dikey boşluk
const TOP_PAD = 44
const BOTTOM_PAD = 76 // depo düğümü + gösterge
const CLIENT_X = 168 // agent (client) sütunu — sol
const HUB_X = 448 // hub — merkez
const DEV_X = 688 // keşfedilen cihazlar — hub ile router arası
const ROUTER_X = 902 // router — gerçek cihazdan türer, yoksa sabit
const NET_X = 1006 // internet — uç

/** router/güvenlik duvarı yuvasına çıkacak cihaz türleri */
const ROUTER_KINDS = new Set(['router', 'firewall'])

function fmtAgo(ts: number) {
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts)
  if (s < 60) return `${s} sn`
  if (s < 3600) return `${Math.floor(s / 60)} dk`
  return `${Math.floor(s / 3600)} sa`
}

function trunc(s: string, n: number) {
  return s.length > n ? s.slice(0, n - 1) + '…' : s
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
    // client sütununda yalnızca ÇEVRİMİÇİ agent'lar — uykudaki/ölü loadgen
    // filosu haritayı binlerce piksele şişirmesin (canlı trafik şemasıyla aynı
    // yaklaşım); kaç tanesinin gizlendiği alt not olarak yazılır.
    const agents = graph.agents.filter((a) => a.online)
    const hiddenAgents = graph.agents.length - agents.length

    // router/firewall türündeki İLK cihaz "Router" yuvasına çıkar; kalan
    // cihazlar (fazladan router/firewall dahil) hub ile router arası sütunda.
    // Cihazlar çevrimdışıyken de gösterilir (down bir switch bilgi taşır).
    const routerDev = graph.devices.find((d) => ROUTER_KINDS.has(d.kind)) ?? null
    const midDevices = graph.devices.filter((d) => d !== routerDev)

    const rows = Math.max(agents.length, midDevices.length, 1)
    const H = TOP_PAD + rows * ROW_H + BOTTOM_PAD
    const midY = TOP_PAD + (rows * ROW_H) / 2

    const HUB = { x: HUB_X, y: midY }
    const ROUTER = { x: ROUTER_X, y: midY }
    const NET = { x: NET_X, y: midY }

    const colTop = (n: number) => TOP_PAD + Math.max(0, (rows - n) * ROW_H) / 2 + ROW_H / 2
    const agentPos = new Map<number, { x: number; y: number }>()
    const devPos = new Map<number, { x: number; y: number }>()
    agents.forEach((a, i) => agentPos.set(a.id, { x: CLIENT_X, y: colTop(agents.length) + i * ROW_H }))
    midDevices.forEach((d, i) => devPos.set(d.id, { x: DEV_X, y: colTop(midDevices.length) + i * ROW_H }))

    // cihaz düğümü (orta sütun veya router yuvası) konumu
    const posOf = (id: number): { x: number; y: number } | undefined =>
      routerDev && id === routerDev.id ? ROUTER : devPos.get(id)

    // kenar çözümleme: peer adı/ip → cihaz düğümü
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
      if (l.kind === 'subnet') continue // subnet linkleri spoke altında dolaylı gösterilir
      if (l.source_type !== 'device') continue
      const src = posOf(l.source_id)
      if (!src) continue
      // peer bir cihaz mı?
      let dst: { x: number; y: number } | undefined
      let dstId = -1
      for (const d of graph.devices) {
        if (nameMatch(l.peer_name, d) || (l.peer_ip && l.peer_ip === d.host)) {
          dst = posOf(d.id)
          dstId = d.id
          break
        }
      }
      if (dst && dstId !== l.source_id) {
        discoveredEdges.push({ x1: src.x, y1: src.y, x2: dst.x, y2: dst.y, kind: l.kind, label: l.local_port || '' })
      } else {
        // çözümlenmemiş komşu (ARP/LLDP ucu) — kaynağın hub tarafına küçük nokta
        const j = hosts.filter((h) => h.source === String(l.source_id)).length
        hosts.push({
          x: src.x - (38 + (j % 3) * 12),
          y: src.y - 12 + Math.floor(j / 3) * 10,
          label: l.kind === 'arp' ? l.peer_ip : l.peer_name.split(' ')[0] || l.peer_name,
          kind: l.kind === 'arp' ? 'arp' : l.kind,
          source: String(l.source_id),
          ts: l.ts,
        })
      }
    }

    // depo düğümü: hub'ın her zaman bağlandığı tek sabit yönetim noktası
    const storage = { x: HUB_X, y: H - BOTTOM_PAD + 30 }

    return { agentPos, devPos, midDevices, agents, hiddenAgents, routerDev, discoveredEdges, hosts, H, HUB, ROUTER, NET, storage }
  }, [graph])

  if (error) {
    return <p className="text-xs text-rose-400">{error}</p>
  }
  if (!graph || !layout) {
    return <p className="text-xs text-slate-500">yükleniyor…</p>
  }

  const edgeColor = (kind: string) =>
    kind === 'lldp' ? '#34d399' : kind === 'cdp' ? '#38bdf8' : kind === 'subnet' ? '#a78bfa' : '#475569'

  const { H, HUB, ROUTER, NET, storage, routerDev, hiddenAgents } = layout

  return (
    <div>
      {graph.devices.length === 0 && graph.agents.length === 0 ? (
        <p className="text-xs text-slate-500">
          Topoloji boş — cihaz ekleyin (SNMP LLDP/CDP/ARP keşfi) veya agent kurun; yerel ağlar otomatik haritaya işlenir.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <svg viewBox={`0 0 ${W} ${H}`} className="w-full min-w-[720px]">
            {/* bölge başlıkları */}
            <g className="fill-slate-600" fontSize={8} fontFamily="monospace" letterSpacing={0.5}>
              <text x={CLIENT_X} y={16} textAnchor="middle">CLIENT (AGENT)</text>
              {layout.midDevices.length > 0 && (
                <text x={DEV_X} y={16} textAnchor="middle">KEŞFEDİLEN CİHAZ</text>
              )}
              <text x={NET_X} y={16} textAnchor="middle">İNTERNET</text>
              {hiddenAgents > 0 && (
                <text x={CLIENT_X} y={26} textAnchor="middle" className="fill-slate-700">
                  +{hiddenAgents} çevrimdışı gizli
                </text>
              )}
            </g>

            {/* spoke hatları: hub → client'lar (sol), hub → cihazlar (sağ) */}
            <g stroke="#1e293b" strokeWidth={1.3}>
              {layout.agents.map((a) => {
                const p = layout.agentPos.get(a.id)!
                return <line key={`sa${a.id}`} x1={HUB.x} y1={HUB.y} x2={p.x} y2={p.y} />
              })}
              {layout.midDevices.map((d) => {
                const p = layout.devPos.get(d.id)!
                return <line key={`sd${d.id}`} x1={HUB.x} y1={HUB.y} x2={p.x} y2={p.y} />
              })}
              {/* orta sütun cihazları → router (internete giden yol) */}
              {layout.midDevices.map((d) => {
                const p = layout.devPos.get(d.id)!
                return <line key={`dr${d.id}`} x1={p.x} y1={p.y} x2={ROUTER.x} y2={ROUTER.y} strokeDasharray="2 5" />
              })}
              <line x1={HUB.x} y1={HUB.y} x2={storage.x} y2={storage.y} strokeDasharray="3 4" />
            </g>

            {/* omurga: hub — router — internet (veri yolu) */}
            <g stroke="#0e7490" strokeWidth={2}>
              <line x1={HUB.x} y1={HUB.y} x2={ROUTER.x} y2={ROUTER.y} />
              <line x1={ROUTER.x} y1={ROUTER.y} x2={NET.x} y2={NET.y} />
            </g>

            {/* keşif ile bulunan zengin bağlantılar (LLDP/CDP) — spoke'un üstüne */}
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

            {/* router — gerçek cihaz varsa adı/durumu, yoksa sabit düğüm */}
            <rect
              x={ROUTER.x - 27}
              y={ROUTER.y - 16}
              width={54}
              height={32}
              rx={5}
              className={
                routerDev
                  ? routerDev.online
                    ? 'fill-slate-900 stroke-emerald-500'
                    : 'fill-slate-900 stroke-slate-600'
                  : 'fill-slate-900 stroke-slate-700'
              }
              strokeWidth={1.4}
            />
            <text x={ROUTER.x} y={ROUTER.y - 3} textAnchor="middle" className="fill-slate-300" fontSize={8} fontFamily="monospace">
              ROUTER
            </text>
            <text x={ROUTER.x} y={ROUTER.y + 8} textAnchor="middle" className="fill-slate-500" fontSize={7} fontFamily="monospace">
              {routerDev ? trunc(routerDev.name, 10) : 'sabit'}
            </text>
            {routerDev && (
              <text x={ROUTER.x} y={ROUTER.y + 28} textAnchor="middle" className="fill-slate-600" fontSize={7.5}>
                {routerDev.kind} · {routerDev.online ? 'online' : 'offline'}
              </text>
            )}

            {/* internet */}
            <circle cx={NET.x} cy={NET.y} r={20} className="fill-slate-900 stroke-sky-500/60" strokeWidth={1.4} />
            <text x={NET.x} y={NET.y + 3} textAnchor="middle" className="fill-sky-400" fontSize={8} fontFamily="monospace">
              NET
            </text>
            <text x={NET.x} y={NET.y + 32} textAnchor="middle" className="fill-slate-500" fontSize={7.5}>
              internet
            </text>

            {/* depo düğümü */}
            <rect x={storage.x - 54} y={storage.y - 12} width={108} height={24} rx={5} className="fill-slate-900 stroke-slate-700" strokeWidth={1.2} />
            <text x={storage.x} y={storage.y + 3.5} textAnchor="middle" className="fill-slate-400" fontSize={8.5} fontFamily="monospace">
              depolama
            </text>

            {/* client'lar (agent) — sol, etiket solda; hepsi çevrimiçi */}
            {layout.agents.map((a) => {
              const p = layout.agentPos.get(a.id)!
              return (
                <g key={`a${a.id}`}>
                  <circle cx={p.x} cy={p.y} r={7} className="fill-cyan-400" />
                  <text x={p.x - 13} y={p.y + 1} textAnchor="end" className="fill-slate-300" fontSize={9.5} fontFamily="monospace">
                    {trunc(a.name, 18)}
                  </text>
                  <text x={p.x - 13} y={p.y + 12} textAnchor="end" className="fill-slate-600" fontSize={8}>
                    {a.site || 'client'}
                  </text>
                </g>
              )
            })}

            {/* orta sütun cihazları — etiket sağda */}
            {layout.midDevices.map((d) => {
              const p = layout.devPos.get(d.id)!
              return (
                <g key={`d${d.id}`}>
                  <circle cx={p.x} cy={p.y} r={7} className={d.online ? 'fill-emerald-500/80' : 'fill-slate-600'} />
                  <text x={p.x + 14} y={p.y + 1} textAnchor="start" className="fill-slate-300" fontSize={9.5} fontFamily="monospace">
                    {trunc(d.name, 18)}
                  </text>
                  <text x={p.x + 14} y={p.y + 12.5} textAnchor="start" className="fill-slate-600" fontSize={8}>
                    {d.kind} · {d.host}
                  </text>
                </g>
              )
            })}

            {/* çözümlenmemiş komşular (ARP/LLDP uçları) */}
            {layout.hosts.map((h, i) => (
              <circle key={i} cx={h.x} cy={h.y} r={2.2} fill={edgeColor(h.kind)} fillOpacity={0.8}>
                <title>
                  {h.label} · {h.kind} · {fmtAgo(h.ts)} önce görüldü
                </title>
              </circle>
            ))}

            {/* gösterge */}
            <g transform={`translate(16, ${H - 12})`}>
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
              <line x1={318} y1={-3} x2={334} y2={-3} stroke="#0e7490" strokeWidth={2} />
              <text x={338} y={0} className="fill-slate-600" fontSize={8.5}>omurga (hub▸router▸net)</text>
            </g>
          </svg>
        </div>
      )}
    </div>
  )
}
