import { type ReactElement, useEffect, useMemo, useRef, useState } from 'react'
import { classifyDir, stripPort, type TrafficDir } from '../lib/traffic'

// Canlı olay akışının görsel karşılığı: sol sütunda AGENT FİLOSUNUN her üyesi
// ayrı bir düğüm — Router/Güvenlik Duvarı — İnternet. Her yeni akış/agent/
// syslog olayı için, olayı üreten agent'ın düğümünden yönü belli animasyonlu
// bir "paket" (ok + kuyruk) geçer. Harici grafik kütüphanesi yok — sahne ve
// animasyon elle yazılmış SVG (bkz. ThroughputChart deseni).

export interface TrafficEvent {
  /** akıştaki satırın kararlı anahtarı — yeni olay tespiti bununla yapılır */
  key: string
  kind: 'flow' | 'agent' | 'syslog'
  ts: number
  /** ham kaynak adres (ip veya ip:port) */
  from: string
  /** ham hedef adres; syslog / LISTEN soketi için boş olabilir */
  to?: string
  /** olayı üreten agent'ın adı — paket o agent'ın düğümünden çıkar/gelir */
  agent?: string
  /** pakette gösterilecek hazır etiket (yoksa from ▸ to üretilir) */
  label?: string
  /** olayın büyüklüğü (bayt) — paket yarıçapını ölçekler */
  weight?: number
}

/** şemada gösterilecek agent — Overview'daki AgentWithRates'ten türetilir */
export interface DiagramAgent {
  name: string
  /** false olanlar şemadan tamamen çıkarılır — düğüm de yok, paket de almaz */
  online: boolean
  site?: string
  /** en yoğun arayüzün gelen/giden hızı (bayt/sn) — düğüm etiketinde gösterilir */
  rxBps?: number
  txBps?: number
}

type Dir = TrafficDir

// --- sahne geometrisi (viewBox koordinatları; yükseklik agent sayısıyla büyür) ---
const W = 1000
const AGENT_X = 152
const FW_X = 524
const NET_X = 884
const NODE_W = 138
/** resolveIdx dönüşü: -1 = agent'sız eksen (cihaz/firewall), -2 = çevrimdışı/bilinmeyen agent → paket üretme */
const DROP = -2

/** agent sayısına göre satır yüksekliği — çok kalabalık filoda daralır */
function rowHeight(count: number): number {
  if (count <= 10) return 46
  if (count <= 20) return 32
  if (count <= 36) return 23
  return 17
}
type Detail = 'full' | 'compact' | 'mini'
function detailFor(count: number): Detail {
  if (count <= 10) return 'full'
  if (count <= 28) return 'compact'
  return 'mini'
}
function sceneHeight(count: number): number {
  return Math.max(320, 58 + Math.max(1, count) * rowHeight(count) + 34)
}
function agentY(i: number, count: number, H: number): number {
  const top = 58
  const bot = 34
  const avail = H - top - bot
  return top + (avail / Math.max(1, count)) * (i + 0.5)
}

const DIR_COLOR: Record<Dir, string> = {
  // eskiden fuchsia (#e879f9) — DESIGN.md'nin 7 sabit renginde olmayan icat
  // edilmiş bir 8. renkti; ThroughputChart.tsx aynı "giden/tx" anlamı için
  // zaten violet kullanıyor, buraya da o taşındı (impeccable critique
  // 2026-09-05)
  out: '#a78bfa', // violet-400 — agent'tan çıkan (tx, ThroughputChart ile aynı)
  in: '#22d3ee', // cyan-400 — internetten gelen
  lan: '#34d399', // emerald-400 — yerel ağ (agent ↔ agent)
  log: '#fbbf24', // amber-400 — syslog / cihaz olayı
}
const DIR_LABEL: Record<Dir, string> = {
  out: 'Giden · agent → internet',
  in: 'Gelen · internet → agent',
  lan: 'Yerel ağ · agent ↔ agent',
  log: 'Olay / Syslog · cihaz bildirimi',
}

function jitterInterior(pts: Array<[number, number]>): Array<[number, number]> {
  const jx = (Math.random() - 0.5) * 14
  const jy = (Math.random() - 0.5) * 18
  return pts.map((pt, i) => (i === 0 || i === pts.length - 1 ? pt : [pt[0] + jx, pt[1] + jy]))
}

function polyMeta(pts: Array<[number, number]>): { seg: number[]; total: number } {
  const seg: number[] = []
  let total = 0
  for (let i = 0; i < pts.length - 1; i++) {
    const l = Math.hypot(pts[i + 1][0] - pts[i][0], pts[i + 1][1] - pts[i][1])
    seg.push(l)
    total += l
  }
  return { seg, total }
}

function sampleAt(
  pts: Array<[number, number]>,
  seg: number[],
  total: number,
  dist: number,
): { x: number; y: number; ang: number } {
  let d = Math.max(0, Math.min(total, dist))
  for (let i = 0; i < seg.length; i++) {
    if (d <= seg[i] || i === seg.length - 1) {
      const [x0, y0] = pts[i]
      const [x1, y1] = pts[i + 1]
      const f = seg[i] === 0 ? 0 : d / seg[i]
      return { x: x0 + (x1 - x0) * f, y: y0 + (y1 - y0) * f, ang: Math.atan2(y1 - y0, x1 - x0) }
    }
    d -= seg[i]
  }
  const [x0, y0] = pts[pts.length - 2]
  const [x1, y1] = pts[pts.length - 1]
  return { x: x1, y: y1, ang: Math.atan2(y1 - y0, x1 - x0) }
}

const easeInOut = (t: number): number => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2)

interface Packet {
  id: number
  dir: Dir
  agentIdx: number // hangi agent düğümünden çıktı (-1 = yok / cihaz)
  pts: Array<[number, number]>
  seg: number[]
  total: number
  t0: number
  dur: number
  label: string
  r: number
}

const STYLE = `
@keyframes tfd-dash { to { stroke-dashoffset: -32; } }
.tfd-dash { animation: tfd-dash 1.6s linear infinite; }
@keyframes tfd-spin { from { transform: translateX(0); } to { transform: translateX(-80px); } }
.tfd-spin { animation: tfd-spin 9s linear infinite; }
@keyframes tfd-led { 0%,100% { opacity: .3 } 50% { opacity: 1 } }
.tfd-led { animation: tfd-led 1.8s ease-in-out infinite; }
@keyframes tfd-ring { 0% { opacity: .55; transform: scale(.86) } 70% { opacity: 0 } 100% { opacity: 0; transform: scale(1.08) } }
.tfd-ring { animation: tfd-ring 3.2s ease-out infinite; transform-box: fill-box; transform-origin: center; }
@keyframes tfd-node { 0% { opacity: .7; transform: scale(1) } 60% { opacity: 0; transform: scale(1.9) } 100% { opacity: 0 } }
.tfd-node { animation: tfd-node 1s ease-out forwards; transform-box: fill-box; transform-origin: center; }
@media (prefers-reduced-motion: reduce) {
  .tfd-dash, .tfd-spin, .tfd-led, .tfd-ring, .tfd-node { animation: none }
}
`

const WILDCARD = new Set(['', '*', '0.0.0.0', '::', '[::]', '[::]:', 'localhost'])
function cleanHost(raw: string): string {
  const h = stripPort(raw)
  return WILDCARD.has(h) ? '' : h
}

function hashStr(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

function fmtBps(bps: number): string {
  const bits = bps * 8
  if (bits >= 1e9) return (bits / 1e9).toFixed(1) + 'G'
  if (bits >= 1e6) return (bits / 1e6).toFixed(1) + 'M'
  if (bits >= 1e3) return (bits / 1e3).toFixed(0) + 'k'
  return Math.round(bits) + ''
}

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(
    () => window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches ?? false,
  )
  useEffect(() => {
    const mq = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    if (!mq) return
    const on = (): void => setReduced(mq.matches)
    mq.addEventListener('change', on)
    return () => mq.removeEventListener('change', on)
  }, [])
  return reduced
}

function AgentNode({
  x,
  y,
  agent,
  detail,
  flash,
}: {
  x: number
  y: number
  agent: DiagramAgent
  detail: Detail
  flash: boolean
}): ReactElement {
  const col = '#34d399' // şemada yalnızca çevrimiçi agent bulunur
  const short = agent.name.length > 14 ? agent.name.slice(0, 13) + '…' : agent.name
  const titleText = `${agent.name}${agent.site ? ` · ${agent.site}` : ''}`
  // istemci (client) olduğu belli olsun diye her düğümün sağında bir monitör ikonu
  const monitor = (cx: number, s: number): ReactElement => (
    <g transform={`translate(${cx},0) scale(${s})`}>
      <rect x={-7} y={-6} width={14} height={10} rx={1.5} fill="#0a1120" stroke={col} strokeWidth={1.1} />
      <rect x={-4.5} y={-3.6} width={9} height={5} rx={0.5} fill={col} opacity={0.22} />
      <rect x={-2} y={4} width={4} height={2} fill={col} />
      <rect x={-5} y={5.6} width={10} height={1.7} rx={0.85} fill={col} />
    </g>
  )
  // düğüm gerçek, değişken veri taşıyor (ad/site/hız) — eskiden yalnızca
  // fare-hover <title> ile erişilebilirdi (role="img" altında ekran
  // okuyucuya hiç ulaşmıyordu); artık klavye/ekran okuyucu ile de erişilebilir
  if (detail === 'mini') {
    return (
      <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={titleText}>
        <title>{titleText}</title>
        {flash && <circle cx={-NODE_W / 2} r={3} fill={col} className="tfd-node" />}
        <circle cx={-NODE_W / 2} r={3} fill={col} className="tfd-led" />
        <text x={-NODE_W / 2 + 9} y={3} fontSize={9} className="fill-slate-400" fontFamily="ui-monospace, monospace">
          {short}
        </text>
        {monitor(NODE_W / 2 - 7, 0.62)}
      </g>
    )
  }
  const h = detail === 'full' ? 30 : 20
  return (
    <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={titleText}>
      <title>{titleText}</title>
      {flash && <circle cx={-NODE_W / 2 + 8} cy={0} r={4} fill={col} className="tfd-node" />}
      <rect
        x={-NODE_W / 2}
        y={-h / 2}
        width={NODE_W}
        height={h}
        rx={4}
        fill="#0d1424"
        stroke="#1e5245"
        strokeWidth={1.2}
      />
      <circle cx={-NODE_W / 2 + 9} cy={0} r={2.6} fill={col} className="tfd-led" />
      <text
        x={-NODE_W / 2 + 17}
        y={detail === 'full' ? -2 : 3.2}
        fontSize={detail === 'full' ? 9.5 : 9}
        fill="#cbd5e1"
        fontFamily="ui-monospace, monospace"
      >
        {short}
      </text>
      {detail === 'full' && (
        <text x={-NODE_W / 2 + 17} y={9} fontSize={8} className="fill-dim-aa" fontFamily="ui-monospace, monospace">
          ↓{fmtBps(agent.rxBps ?? 0)} ↑{fmtBps(agent.txBps ?? 0)}
        </text>
      )}
      {monitor(NODE_W / 2 - 13, detail === 'full' ? 1 : 0.82)}
    </g>
  )
}

function Firewall({ y, reduced }: { y: number; reduced: boolean }): ReactElement {
  const shield = 'M 0 -54 L 44 -38 L 44 6 C 44 31 25 50 0 59 C -25 50 -44 31 -44 6 L -44 -38 Z'
  return (
    // sabit düğüm, değişken veri taşımıyor — dekoratif
    <g transform={`translate(${FW_X},${y})`} aria-hidden="true">
      <path d={shield} fill="rgba(56,189,248,0.05)" stroke="#38bdf8" strokeOpacity={0.4} strokeWidth={1.5} />
      {!reduced && <path d={shield} className="tfd-ring" fill="none" stroke="#38bdf8" strokeWidth={1.5} />}
      <rect x={-32} y={-20} width={64} height={16} rx={3} fill="#0d1526" stroke="#334155" strokeWidth={1.2} />
      <rect x={-32} y={0} width={64} height={16} rx={3} fill="#0d1526" stroke="#334155" strokeWidth={1.2} />
      {[0, 1, 2].map((i) => (
        <circle
          key={i}
          cx={-22 + i * 7}
          cy={-12}
          r={2}
          fill={['#34d399', '#fbbf24', '#22d3ee'][i]}
          className="tfd-led"
          style={{ animationDelay: `${i * 0.35}s` }}
        />
      ))}
      {[0, 1, 2].map((i) => (
        <circle key={i} cx={-22 + i * 7} cy={8} r={2} fill="#475569" />
      ))}
      <text
        x={0}
        y={76}
        textAnchor="middle"
        fontSize={10}
        className="fill-slate-400"
        fontFamily="ui-monospace, monospace"
        letterSpacing={1}
      >
        ROUTER · GÜVENLİK DUVARI
      </text>
    </g>
  )
}

function Globe({ y, reduced, remote }: { y: number; reduced: boolean; remote: string | null }): ReactElement {
  const R = 42
  return (
    // sabit düğüm — "son: X" değişken ama ikincil bilgi, ana gösterge şeridinde
    // ve paket etiketlerinde zaten aynı bilgi metin olarak mevcut; dekoratif
    <g transform={`translate(${NET_X},${y})`} aria-hidden="true">
      <circle r={R} fill="#0a1120" stroke="#334155" strokeWidth={1.5} />
      <g clipPath="url(#tfd-globe)">
        {[-24, -12, 0, 12, 24].map((oy, i) => (
          <ellipse
            key={i}
            cx={0}
            cy={oy}
            rx={Math.sqrt(Math.max(1, (R - 1) * (R - 1) - oy * oy))}
            ry={4.2}
            fill="none"
            stroke="#1e3a5f"
            strokeWidth={1}
          />
        ))}
        <g className={reduced ? '' : 'tfd-spin'}>
          {[0, 1].map((rep) => (
            <g key={rep} transform={`translate(${rep * 80},0)`}>
              {[-40, -20, 0, 20, 40].map((ox, i) => {
                const bow = ox === 0 ? 12 : ox > 0 ? 8 : -8
                return (
                  <path
                    key={i}
                    d={`M ${ox} -40 C ${ox + bow} -13, ${ox + bow} 13, ${ox} 40`}
                    fill="none"
                    stroke="#1e3a5f"
                    strokeWidth={1}
                  />
                )
              })}
              <path d="M -28 -8 q 10 -7 18 -1 q 6 6 -3 11 q -11 3 -16 -5 z" fill="#14532d" fillOpacity={0.55} />
              <path d="M 5 9 q 11 -4 17 3 q 3 7 -6 10 q -11 1 -12 -7 z" fill="#14532d" fillOpacity={0.55} />
            </g>
          ))}
        </g>
      </g>
      <circle r={R} fill="none" stroke="#38bdf8" strokeOpacity={0.16} strokeWidth={3} />
      <text
        x={0}
        y={60}
        textAnchor="middle"
        fontSize={10}
        className="fill-slate-400"
        fontFamily="ui-monospace, monospace"
        letterSpacing={1}
      >
        İNTERNET
      </text>
      {remote && (
        <text x={0} y={74} textAnchor="middle" fontSize={9} className="fill-dim-aa" fontFamily="ui-monospace, monospace">
          son: {remote}
        </text>
      )}
    </g>
  )
}

export function TrafficFlowDiagram({
  events,
  agents = [],
}: {
  events: TrafficEvent[]
  agents?: DiagramAgent[]
}): ReactElement {
  const reduced = usePrefersReducedMotion()
  const packetsRef = useRef<Packet[]>([])
  const idRef = useRef(0)
  const seenRef = useRef<Set<string>>(new Set())
  const primedRef = useRef(false)
  const prevRef = useRef(0)
  const lastCountRef = useRef(0)
  const [, setFrame] = useState(0)
  const [tally, setTally] = useState<Record<Dir, number>>({ out: 0, in: 0, lan: 0, log: 0 })
  const [netEnd, setNetEnd] = useState<string | null>(null)
  const flashRef = useRef<Map<number, number>>(new Map()) // agentIdx → son aktivite zamanı

  // yalnızca ÇEVRİMİÇİ agent'lar şemada düğüm olur — kapalı agent trafik üretmez.
  // Filo listesi Overview'dan online-first sıralı + tam gelir; süzme yine de
  // burada yapılır ki poll gecikmesinde bayat bir "agent" olayı masum bir
  // düğüme sıçramasın ("N çevrimdışı gizli" ipucu için tam sayıya ihtiyaç var).
  const onlineAgents = useMemo(() => agents.filter((a) => a.online), [agents])
  const offlineCount = agents.length - onlineAgents.length

  // agent adı → düğüm indeksi (yalnızca online). Adı bilinen-ama-kapalı ya da
  // hiç bilinmeyen agent DROP döndürür → o olay için paket üretilmez.
  const agentIndex = useMemo(() => {
    const m = new Map<string, number>()
    onlineAgents.forEach((a, i) => m.set(a.name, i))
    return m
  }, [onlineAgents])
  const count = onlineAgents.length
  const H = sceneHeight(count)
  const midY = H / 2
  const detail = detailFor(count)
  const FW_IN: [number, number] = [FW_X - 44, midY]
  const FW_OUT: [number, number] = [FW_X + 44, midY]
  const nodeRight = AGENT_X + NODE_W / 2

  const resolveIdx = (name: string | undefined, hashKey: string): number => {
    if (count === 0) return -1
    if (name) return agentIndex.has(name) ? agentIndex.get(name)! : DROP
    return hashStr(hashKey) % count // yalnızca anonim uç (lan hedefi) — online havuzdan
  }

  const pathFor = (dir: Dir, idxA: number, idxB: number): Array<[number, number]> => {
    const a = (i: number): [number, number] =>
      i < 0 ? FW_IN : [nodeRight, agentY(i, count, H)]
    switch (dir) {
      case 'out':
        return idxA < 0
          ? [FW_OUT, [(FW_X + NET_X) / 2, midY - 8], [NET_X - 36, midY]]
          : [a(idxA), FW_IN, FW_OUT, [NET_X - 36, midY]]
      case 'in':
        return idxA < 0
          ? [[NET_X - 36, midY], [(FW_X + NET_X) / 2, midY + 8], FW_OUT]
          : [[NET_X - 36, midY], FW_OUT, FW_IN, a(idxA)]
      case 'lan': {
        const b = idxB >= 0 && idxB !== idxA ? idxB : (idxA + 1) % Math.max(1, count)
        return [a(idxA), [FW_X - 8, midY - 14], a(b)]
      }
      case 'log': {
        const to = idxA < 0 ? FW_IN : a(idxA)
        return [[FW_X, midY + 16], [(FW_X + to[0]) / 2, (midY + to[1]) / 2 + 8], to]
      }
    }
  }

  // spawn — en güncel closure'ı ref üzerinden tut (RAF döngüsü tek kez kurulur)
  // eskiden `ev === null` özel bir dal ile sahte "ambient" paket üretiyordu
  // (gerçek trafik yokken sahneyi canlı göstermek için) — bu paketler gerçek
  // trafikten görsel olarak ayırt edilemiyordu ve tetikleme koşulu (pps>0)
  // gerçek trafik arttıkça daha sık ateşleniyordu; bir izleme aracında güven
  // riski olduğu için tamamen kaldırıldı (impeccable critique 2026-09-05, P0)
  const spawnRef = useRef<(ev: TrafficEvent, at: number) => void>(() => {})
  spawnRef.current = (ev, at) => {
    const dir0 = classifyDir(ev)
    const deviceFlow = ev.kind === 'flow' && !ev.agent
    const idxA0 = deviceFlow ? -1 : resolveIdx(ev.agent, ev.from)
    if (idxA0 === DROP) return // çevrimdışı/bilinmeyen agent olayı — şemada gösterilmez
    let dir = dir0
    let idxA = idxA0
    let idxB = -1
    // syslog/olay bildirimi bir online agent'a ait değilse firewall ekseninde kalır
    if (dir === 'log' && !(ev.agent && agentIndex.has(ev.agent))) idxA = -1
    if (count === 0 && dir === 'lan') dir = 'in'
    if (dir === 'lan') idxB = resolveIdx(undefined, ev.to ?? ev.key)
    const fromH = cleanHost(ev.from)
    const toH = cleanHost(ev.to ?? '')
    const raw =
      dir === 'log'
        ? fromH || stripPort(ev.from)
        : (ev.label ?? (toH ? `${fromH || '?'} ▸ ${toH}` : `${fromH || stripPort(ev.from)} · dinliyor`))
    const label = raw.length > 26 ? raw.slice(0, 25) + '…' : raw
    let r = 4.5
    if (ev.weight && ev.weight > 0) r = Math.min(8, 4.5 + Math.log10(ev.weight) * 0.7)
    // tally/netEnd — gerçek veri, hareket-azaltmada da güncellenmeye devam
    // etmeli (bkz. aşağıdaki effect'in artık `reduced`'ı hiç kontrol etmemesi)
    setTally((p) => ({ ...p, [dir]: p[dir] + 1 }))
    if (dir === 'out') setNetEnd(toH || null)
    else if (dir === 'in') setNetEnd(fromH || null)
    if (idxA >= 0) flashRef.current.set(idxA, at)
    // hareket-azaltma açıkken görsel paket kuyruğuna hiç eklenmiyor — zaten
    // RAF döngüsü çalışmadığından uçmayacak, veri yukarıda zaten işlendi
    if (reduced) return
    const pts = jitterInterior(pathFor(dir, idxA, idxB))
    const { seg, total } = polyMeta(pts)
    const base = dir === 'log' ? 1050 : dir === 'lan' ? 1750 : 2300
    packetsRef.current.push({
      id: idRef.current++,
      dir,
      agentIdx: idxA,
      pts,
      seg,
      total,
      t0: at,
      dur: base + Math.random() * 420,
      label,
      r,
    })
    if (packetsRef.current.length > 34) packetsRef.current.splice(0, packetsRef.current.length - 34)
  }

  // yeni olayları paket olarak kuyruğa al (ilk dolu partide sadece "görüldü"
  // işaretle) — eskiden `reduced` iken tamamen atlanıyordu, bu yüzden
  // hareket-azaltma tercih eden kullanıcılar sayaç/son-uç verisini de hiç
  // almıyordu; artık yalnızca görsel paket kuyruğu (spawnRef içinde) hareket
  // tercihine göre atlanıyor, veri işleme her zaman çalışır
  useEffect(() => {
    if (events.length === 0) return
    if (!primedRef.current) {
      for (const e of events) seenRef.current.add(e.key)
      primedRef.current = true
      return
    }
    const fresh = events.filter((e) => !seenRef.current.has(e.key))
    if (fresh.length === 0) return
    for (const e of fresh) seenRef.current.add(e.key)
    if (seenRef.current.size > 4000) seenRef.current = new Set(events.map((e) => e.key))
    const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
    fresh
      .filter((e) => !e.agent || agentIndex.has(e.agent)) // kapalı agent olayları dilime hiç girmesin
      .slice(-12)
      .reverse()
      .forEach((e, i) => spawnRef.current(e, now + Math.min(i, 10) * 180))
  }, [events, agentIndex])

  // RAF döngüsü — yalnızca hareket tercih edilince kurulur; sadece animasyon
  // varken yeniden çizer (boştayken sessiz)
  useEffect(() => {
    if (reduced) return
    let raf = 0
    prevRef.current = performance.now()
    const loop = (now: number): void => {
      raf = requestAnimationFrame(loop)
      prevRef.current = now
      if (packetsRef.current.length) {
        packetsRef.current = packetsRef.current.filter((p) => now - p.t0 < p.dur + 150)
      }
      if (flashRef.current.size) {
        for (const [k, t] of flashRef.current) if (now - t > 1000) flashRef.current.delete(k)
      }
      const n = packetsRef.current.length
      if (n > 0 || lastCountRef.current > 0 || flashRef.current.size > 0) setFrame((f) => (f + 1) % 1_000_000)
      lastCountRef.current = n
    }
    raf = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(raf)
  }, [reduced])

  // imperatif animasyon: paket listesi ref'te tutulur, RAF döngüsü her karede
  // setFrame ile yeniden çizdirir — konumlar o anki saate göre burada hesaplanır
  const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
  const live = packetsRef.current.filter((p) => {
    const raw = (now - p.t0) / p.dur
    return raw >= 0 && raw <= 1
  })
  const seenLabels = new Set<string>()
  const labelIds = new Set(
    [...live]
      .filter((p) => p.label)
      .sort((a, b) => b.t0 - a.t0)
      .filter((p) => !seenLabels.has(p.label) && seenLabels.add(p.label))
      .slice(0, 4)
      .map((p) => p.id),
  )

  return (
    <div>
      <style>{STYLE}</style>
      <div className="overflow-x-auto">
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className="w-full min-w-[680px]"
          role="group"
          aria-label="Çevrimiçi agent'lar, router/güvenlik duvarı ve internet arasında canlı paket akışı şeması"
        >
          <defs>
            <radialGradient id="tfd-bg" cx="50%" cy="0%" r="120%">
              <stop offset="0%" stopColor="#0f1e33" />
              <stop offset="60%" stopColor="#0a1220" />
              <stop offset="100%" stopColor="#070d18" />
            </radialGradient>
            <clipPath id="tfd-globe">
              <circle r={41} cx={0} cy={0} />
            </clipPath>
            <filter id="tfd-glow" x="-60%" y="-60%" width="220%" height="220%">
              <feGaussianBlur stdDeviation="2.4" result="b" />
              <feMerge>
                <feMergeNode in="b" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <rect aria-hidden="true" x={0} y={0} width={W} height={H} rx={8} fill="url(#tfd-bg)" />

          {/* bölge başlıkları — dekoratif */}
          <g aria-hidden="true">
            <text x={AGENT_X} y={28} textAnchor="middle" fontSize={11} className="fill-slate-300" fontFamily="ui-monospace, monospace" letterSpacing={1}>
              AGENT FİLOSU
            </text>
            <text x={AGENT_X} y={H - 14} textAnchor="middle" fontSize={9} className="fill-dim-aa" fontFamily="ui-monospace, monospace">
              {count === 0
                ? 'aktif agent yok'
                : `${count} aktif${offlineCount > 0 ? ` · ${offlineCount} çevrimdışı gizli` : ''}`}
            </text>
          </g>

          {/* altyapı bağlantıları: her çevrimiçi agent düğümünden güvenlik duvarına — dekoratif */}
          <g aria-hidden="true">
            {count === 0 ? (
              <line
                x1={nodeRight}
                y1={midY}
                x2={FW_IN[0]}
                y2={FW_IN[1]}
                stroke="#1e3a5f"
                strokeWidth={1.5}
                strokeDasharray="2 6"
                className={reduced ? '' : 'tfd-dash'}
              />
            ) : (
              onlineAgents.map((_, i) => (
                <line
                  key={i}
                  x1={nodeRight}
                  y1={agentY(i, count, H)}
                  x2={FW_IN[0]}
                  y2={FW_IN[1]}
                  stroke="#17324f"
                  strokeWidth={1}
                  strokeDasharray="2 6"
                  className={reduced ? '' : 'tfd-dash'}
                />
              ))
            )}
            <line
              x1={FW_OUT[0]}
              y1={FW_OUT[1]}
              x2={NET_X - 38}
              y2={midY}
              stroke="#1e3a5f"
              strokeWidth={1.5}
              strokeDasharray="2 6"
              className={reduced ? '' : 'tfd-dash'}
            />
          </g>

          {onlineAgents.map((a, i) => (
            <AgentNode
              key={a.name}
              x={AGENT_X}
              y={agentY(i, count, H)}
              agent={a}
              detail={detail}
              flash={!reduced && flashRef.current.has(i)}
            />
          ))}
          {count === 0 && (
            <text x={AGENT_X} y={midY} textAnchor="middle" fontSize={10} className="fill-dim-aa" fontFamily="ui-monospace, monospace" aria-hidden="true">
              aktif agent bekleniyor
            </text>
          )}
          <Firewall y={midY} reduced={reduced} />
          <Globe y={midY} reduced={reduced} remote={netEnd} />

          {/* uçan paketler — dekoratif animasyon; taşıdığı bilgi (yön/sayaç)
              alttaki gösterge şeridinde zaten metin olarak mevcut */}
          <g aria-hidden="true">
          {live.map((p) => {
            const raw = (now - p.t0) / p.dur
            const u = easeInOut(raw)
            const head = sampleAt(p.pts, p.seg, p.total, u * p.total)
            const fade = Math.min(1, raw / 0.08) * Math.min(1, (1 - raw) / 0.14)
            const col = DIR_COLOR[p.dir]
            const deg = (head.ang * 180) / Math.PI
            return (
              <g key={p.id} opacity={fade}>
                {[1, 2, 3, 4].map((k) => {
                  const s = sampleAt(p.pts, p.seg, p.total, Math.max(0, u * p.total - k * 6))
                  return (
                    <circle
                      key={k}
                      cx={s.x}
                      cy={s.y}
                      r={Math.max(0.5, p.r - k * 1.05)}
                      fill={col}
                      opacity={0.45 - k * 0.09}
                    />
                  )
                })}
                <g transform={`translate(${head.x},${head.y}) rotate(${deg})`}>
                  <circle r={p.r} fill={col} filter="url(#tfd-glow)" />
                  <path d={`M ${p.r + 5} 0 L ${p.r - 1} ${p.r - 0.5} L ${p.r - 1} ${-(p.r - 0.5)} Z`} fill={col} />
                </g>
                {labelIds.has(p.id) && raw < 0.84 && (
                  <g transform={`translate(${head.x},${head.y - 14})`} opacity={Math.min(1, (0.84 - raw) / 0.2)}>
                    <rect
                      x={-p.label.length * 3.15 - 5}
                      y={-9}
                      width={p.label.length * 6.3 + 10}
                      height={15}
                      rx={3}
                      fill="#0b1220"
                      stroke={col}
                      strokeOpacity={0.4}
                    />
                    <text textAnchor="middle" y={2} fontSize={9} className="fill-slate-300" fontFamily="ui-monospace, monospace">
                      {p.label}
                    </text>
                  </g>
                )}
              </g>
            )
          })}
          </g>
        </svg>
      </div>

      {/* gösterge + sayaçlar */}
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[11px]">
        {(['out', 'in', 'lan', 'log'] as Dir[]).map((d) => (
          <span key={d} className="flex items-center gap-1.5">
            <span className="size-2 rounded-full" style={{ background: DIR_COLOR[d] }} />
            <span className="text-slate-400">{DIR_LABEL[d]}</span>
            <span className="font-mono text-dim-aa">{tally[d]}</span>
          </span>
        ))}
        <span className="ml-auto font-mono text-dim-aa">
          {reduced ? 'hareket azaltma açık' : `${live.length} aktif paket`}
        </span>
      </div>
    </div>
  )
}
