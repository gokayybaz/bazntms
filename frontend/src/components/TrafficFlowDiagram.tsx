import {
  type KeyboardEvent as RKeyboardEvent,
  type MouseEvent as RMouseEvent,
  type ReactElement,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
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
  /** bağlı olduğu erişim katmanı cihazı (switch/AP). undefined = "Doğrudan". */
  uplinkId?: number
}

/** gruplama için erişim katmanı cihazı (switch/AP/router/firewall) */
export interface DiagramDevice {
  id: number
  name: string
  kind: string
  online?: boolean
}

type Dir = TrafficDir

// --- sahne geometrisi (viewBox koordinatları) ---
// Sahne agent sayısına VE tam-ekran bayrağına göre kurulur (buildScene): tek
// uzun sütun yerine kalabalık filo birden çok sütuna paketlenir, böylece
// yükseklik sınırlı kalır ("bir bakışta izleme"). Tam ekranda sahne genişler,
// daha çok sütun/satıra izin verilir.
const NODE_W = 138
const TOP = 58
const BOT = 34
/** resolveIdx dönüşü: -1 = agent'sız eksen (cihaz/firewall), -2 = çevrimdışı/bilinmeyen agent → paket üretme */
const DROP = -2

type Detail = 'full' | 'compact' | 'mini' | 'dot'

/** gruplama: bir uplink cihazının (veya "Doğrudan") altındaki agent bloğu */
interface GroupBand {
  key: string
  label: string
  kind: string
  deviceId: number | null
  online: boolean
  /** onlineAgents içindeki global indeksler */
  members: number[]
  y0: number
  rows: number
  cols: number
  /** switch/AP düğümünün + bu grubun paket rotasının dikey merkezi */
  switchY: number
}

interface Scene {
  W: number
  FW_X: number
  NET_X: number
  cols: number
  rows: number
  rowH: number
  bandLeft: number
  bandRight: number
  colGap: number
  nodeW: number
  headerX: number
  H: number
  midY: number
  detail: Detail
  /** gruplu düzen etkinse (en az bir uplink atanmış) doldurulur */
  bands?: GroupBand[]
  posByIdx?: Map<number, { x: number; y: number }>
  /** i. agent'ın grubunun switch düğümü y'si; grup "Doğrudan" ise null */
  switchYByIdx?: (number | null)[]
}

const GROUP_HEADER_H = 15

function buildScene(rawCount: number, fill: boolean): Scene {
  const count = Math.max(1, rawCount)
  const W = fill ? 1300 : 1000
  const bandLeft = 40
  const maxCols = fill ? 4 : 3
  const perCol = fill ? 32 : 22
  const cols = Math.max(1, Math.min(maxCols, Math.ceil(count / perCol)))
  const rows = Math.max(1, Math.ceil(count / cols))

  // yatay yerleşim: [agent bandı] — koridor (paketlerin uçtuğu boşluk) —
  // [firewall] — [internet]. Çok sütunda bant daha dar tutulup koridora yer
  // açılır ki paketler node ızgarasının üstünden geçmek zorunda kalmasın.
  const FW_X = fill ? 780 : 520
  const NET_X = fill ? 1160 : 884
  const corridor = cols > 1 ? (fill ? 150 : 140) : 96
  const bandRight = FW_X - corridor

  // tek sütun: eski kademeli satır yüksekliği. çok sütun: kompakt satır —
  // tam ekranda sahne dikeyde de dolsun diye satır ~860 hedefe göre açılır
  // (aksi halde `meet` ölçeklemesi üstte/altta büyük boşluk bırakıyordu).
  const idealRowH =
    cols > 1
      ? fill
        ? Math.max(20, Math.min(40, (860 - TOP - BOT) / rows))
        : 20
      : count <= 10
        ? 46
        : count <= 20
          ? 32
          : count <= 36
            ? 23
            : 17
  const maxH = fill ? 1600 : 760
  let rowH = idealRowH
  let H = TOP + rows * rowH + BOT
  if (H > maxH) {
    rowH = Math.max(11, (maxH - TOP - BOT) / rows)
    H = TOP + rows * rowH + BOT
  }
  H = Math.max(320, H)

  const detail: Detail = cols > 1 ? 'dot' : count <= 10 ? 'full' : count <= 28 ? 'compact' : 'mini'
  const colGap = (bandRight - bandLeft) / cols
  const nodeW = cols > 1 ? Math.max(64, Math.min(NODE_W, colGap - 12)) : NODE_W

  return {
    W,
    FW_X,
    NET_X,
    cols,
    rows,
    rowH,
    bandLeft,
    bandRight,
    colGap,
    nodeW,
    headerX: (bandLeft + bandRight) / 2,
    H,
    midY: H / 2,
    detail,
  }
}

/** i. çevrimiçi agent'ın sahne konumu (düğüm merkezi). Tek sütunda dikeyde eşit
 *  yayılır; çok sütunda sütunlar yukarıdan aşağıya dolar (column-major). */
function agentPos(i: number, s: Scene): { x: number; y: number } {
  if (s.posByIdx) return s.posByIdx.get(i) ?? { x: s.bandLeft, y: s.midY }
  if (s.cols === 1) {
    const avail = s.H - TOP - BOT
    return { x: s.headerX, y: TOP + (avail / Math.max(1, s.rows)) * (i + 0.5) }
  }
  const col = Math.min(s.cols - 1, Math.floor(i / s.rows))
  const row = i - col * s.rows
  return { x: s.bandLeft + s.colGap * col + s.colGap / 2, y: TOP + s.rowH * (row + 0.5) }
}

/** c. sütundaki agent sayısı (son sütun eksik dolabilir). */
function colCount(c: number, count: number, s: Scene): number {
  return c < s.cols - 1 ? s.rows : count - (s.cols - 1) * s.rows
}

/** gruplu sahne: agent bandı dikeyde uplink gruplarına bölünür; her grup kendi
 *  içinde sütunlara paketlenir, sağ kenarında (busX) bir switch/AP düğümü olur.
 *  "Doğrudan" grubu (uplink'siz) düğümsüzdür, router'a düz bağlanır. */
function buildGroupedScene(
  bandsIn: { key: string; label: string; kind: string; deviceId: number | null; online: boolean; members: number[] }[],
  fill: boolean,
): Scene {
  const W = fill ? 1300 : 1000
  const bandLeft = 40
  const FW_X = fill ? 780 : 520
  const NET_X = fill ? 1160 : 884
  const bandRight = FW_X - (fill ? 150 : 140)
  const maxCols = fill ? 4 : 3
  // sütun başına hedef satır — grup bloğu absürt uzun olmasın, küçük gruplar da
  // yayılsın (12 agent tek sütunda 12 satır yerine 2 sütunda 6 satır)
  const rowsPerCol = fill ? 11 : 8
  const colGap = (bandRight - bandLeft) / maxCols
  const nodeW = Math.max(64, Math.min(NODE_W, colGap - 12))

  const bands: GroupBand[] = bandsIn.map((b) => {
    const cols = Math.max(1, Math.min(maxCols, Math.ceil(b.members.length / rowsPerCol)))
    const rows = Math.max(1, Math.ceil(b.members.length / cols))
    return { ...b, cols, rows, y0: 0, switchY: 0 }
  })

  const totalRows = bands.reduce((n, b) => n + b.rows, 0)
  const headerTotal = bands.length * GROUP_HEADER_H
  const maxH = fill ? 1600 : 780
  const targetH = fill ? 940 : Math.min(760, TOP + BOT + headerTotal + totalRows * 22)
  let rowH = Math.max(12, Math.min(fill ? 34 : 24, (targetH - TOP - BOT - headerTotal) / Math.max(1, totalRows)))
  let H = TOP + BOT + headerTotal + totalRows * rowH
  if (H > maxH) {
    rowH = Math.max(11, (maxH - TOP - BOT - headerTotal) / Math.max(1, totalRows))
    H = TOP + BOT + headerTotal + totalRows * rowH
  }
  H = Math.max(320, H)

  const posByIdx = new Map<number, { x: number; y: number }>()
  const switchYByIdx: (number | null)[] = []
  let y = TOP
  for (const b of bands) {
    b.y0 = y
    const bodyTop = y + GROUP_HEADER_H
    const bodyH = b.rows * rowH
    b.switchY = bodyTop + bodyH / 2
    b.members.forEach((globalIdx, k) => {
      const col = Math.floor(k / b.rows)
      const row = k - col * b.rows
      posByIdx.set(globalIdx, {
        x: bandLeft + colGap * col + colGap / 2,
        y: bodyTop + rowH * (row + 0.5),
      })
      switchYByIdx[globalIdx] = b.deviceId === null ? null : b.switchY
    })
    y += GROUP_HEADER_H + bodyH
  }

  return {
    W,
    FW_X,
    NET_X,
    cols: maxCols,
    rows: Math.max(...bands.map((b) => b.rows), 1),
    rowH,
    bandLeft,
    bandRight,
    colGap,
    nodeW,
    headerX: (bandLeft + bandRight) / 2,
    H,
    midY: H / 2,
    detail: 'dot',
    bands,
    posByIdx,
    switchYByIdx,
  }
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

/** pts polyline'ının [d0, d1] yay-uzunluğu aralığını SVG path 'd' dizesi yapar —
 *  paket "ok"unun arkasında bıraktığı izi çizmek için (dolu yol veya son parça). */
function polySubPath(pts: Array<[number, number]>, seg: number[], d0: number, d1: number): string {
  if (d1 <= d0) return ''
  const cmds: string[] = []
  let acc = 0
  for (let i = 0; i < seg.length; i++) {
    const segStart = acc
    const lo = Math.max(d0, segStart)
    const hi = Math.min(d1, segStart + seg[i])
    if (hi > lo && seg[i] > 0) {
      const [x0, y0] = pts[i]
      const [x1, y1] = pts[i + 1]
      const fLo = (lo - segStart) / seg[i]
      const fHi = (hi - segStart) / seg[i]
      const ax = x0 + (x1 - x0) * fLo
      const ay = y0 + (y1 - y0) * fLo
      const bx = x0 + (x1 - x0) * fHi
      const by = y0 + (y1 - y0) * fHi
      if (cmds.length === 0) cmds.push(`M ${ax.toFixed(1)} ${ay.toFixed(1)}`)
      cmds.push(`L ${bx.toFixed(1)} ${by.toFixed(1)}`)
    }
    acc += seg[i]
  }
  return cmds.join(' ')
}

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

/** düzenleme modunda tıklanabilir SVG düğümü için ortak <g> prop'ları */
function activateProps(onActivate?: () => void) {
  if (!onActivate) return {}
  return {
    onClick: (e: RMouseEvent) => {
      e.stopPropagation()
      onActivate()
    },
    onKeyDown: (e: RKeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        onActivate()
      }
    },
    style: { cursor: 'pointer' as const },
  }
}

function AgentNode({
  x,
  y,
  agent,
  detail,
  flash,
  nodeW = NODE_W,
  onActivate,
}: {
  x: number
  y: number
  agent: DiagramAgent
  detail: Detail
  flash: boolean
  nodeW?: number
  onActivate?: () => void
}): ReactElement {
  const col = '#34d399' // şemada yalnızca çevrimiçi agent bulunur
  const act = activateProps(onActivate)
  // dar sütunda ad kutuya sığacak kadar kırpılır (yaklaşık 4.7px/karakter mono)
  const maxChars = detail === 'dot' ? Math.max(6, Math.floor((nodeW - 14) / 4.7)) : 14
  const short =
    agent.name.length > maxChars ? agent.name.slice(0, Math.max(1, maxChars - 1)) + '…' : agent.name
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
  // dot — çok sütunlu kalabalık filo: yalnızca LED + kırpılmış ad, kutu/ikon yok
  if (detail === 'dot') {
    return (
      <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={titleText} {...act}>
        <title>{titleText}</title>
        {flash && <circle cx={-nodeW / 2} r={3} fill={col} className="tfd-node" />}
        <circle cx={-nodeW / 2} r={2.3} fill={col} className="tfd-led" />
        <text x={-nodeW / 2 + 7} y={2.7} fontSize={8} className="fill-tui-dim" fontFamily="ui-monospace, monospace">
          {short}
        </text>
      </g>
    )
  }
  if (detail === 'mini') {
    return (
      <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={titleText} {...act}>
        <title>{titleText}</title>
        {flash && <circle cx={-nodeW / 2} r={3} fill={col} className="tfd-node" />}
        <circle cx={-nodeW / 2} r={3} fill={col} className="tfd-led" />
        <text x={-nodeW / 2 + 9} y={3} fontSize={9} className="fill-tui-dim" fontFamily="ui-monospace, monospace">
          {short}
        </text>
        {monitor(nodeW / 2 - 7, 0.62)}
      </g>
    )
  }
  const h = detail === 'full' ? 30 : 20
  return (
    <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={titleText} {...act}>
      <title>{titleText}</title>
      {flash && <circle cx={-nodeW / 2 + 8} cy={0} r={4} fill={col} className="tfd-node" />}
      <rect
        x={-nodeW / 2}
        y={-h / 2}
        width={nodeW}
        height={h}

        fill="#0d1424"
        stroke="#1e5245"
        strokeWidth={1.2}
      />
      <circle cx={-nodeW / 2 + 9} cy={0} r={2.6} fill={col} className="tfd-led" />
      <text
        x={-nodeW / 2 + 17}
        y={detail === 'full' ? -2 : 3.2}
        fontSize={detail === 'full' ? 9.5 : 9}
        fill="#ced7e3"
        fontFamily="ui-monospace, monospace"
      >
        {short}
      </text>
      {detail === 'full' && (
        <text x={-nodeW / 2 + 17} y={9} fontSize={8} className="fill-tui-dim" fontFamily="ui-monospace, monospace">
          ↓{fmtBps(agent.rxBps ?? 0)} ↑{fmtBps(agent.txBps ?? 0)}
        </text>
      )}
      {monitor(nodeW / 2 - 13, detail === 'full' ? 1 : 0.82)}
    </g>
  )
}

/** grup switch/AP düğümü — busX'te, grup bandının dikey merkezinde. Düzenleme
 *  modunda tıklanabilir (sil / router'a bağla). */
function GroupSwitch({
  x,
  y,
  band,
  onActivate,
}: {
  x: number
  y: number
  band: GroupBand
  onActivate?: () => void
}): ReactElement {
  const col = band.online ? '#38bdf8' : '#8794a8'
  const tag =
    band.kind === 'ap' ? 'AP' : band.kind === 'router' ? 'RT' : band.kind === 'firewall' ? 'FW' : 'SW'
  const act = activateProps(onActivate)
  return (
    <g transform={`translate(${x},${y})`} role="button" tabIndex={0} aria-label={`${band.label} (${band.kind || 'grup'})`} {...act}>
      <title>{`${band.label} · ${band.kind || 'grup'} · ${band.members.length} agent`}</title>
      <rect x={-11} y={-7} width={22} height={14} fill="#0d1526" stroke={col} strokeWidth={1.2} />
      <text x={0} y={3.2} textAnchor="middle" fontSize={8} fill={col} fontFamily="ui-monospace, monospace" letterSpacing={0.5}>
        {tag}
      </text>
    </g>
  )
}

function Firewall({ x, y, reduced }: { x: number; y: number; reduced: boolean }): ReactElement {
  const shield = 'M 0 -54 L 44 -38 L 44 6 C 44 31 25 50 0 59 C -25 50 -44 31 -44 6 L -44 -38 Z'
  return (
    // sabit düğüm, değişken veri taşımıyor — dekoratif
    <g transform={`translate(${x},${y})`} aria-hidden="true">
      <path d={shield} fill="rgba(56,189,248,0.05)" stroke="#38bdf8" strokeOpacity={0.4} strokeWidth={1.5} />
      {!reduced && <path d={shield} className="tfd-ring" fill="none" stroke="#38bdf8" strokeWidth={1.5} />}
      <rect x={-32} y={-20} width={64} height={16} fill="#0d1526" stroke="#35485f" strokeWidth={1.2} />
      <rect x={-32} y={0} width={64} height={16} fill="#0d1526" stroke="#35485f" strokeWidth={1.2} />
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
        className="fill-tui-dim"
        fontFamily="ui-monospace, monospace"
        letterSpacing={1}
      >
        ROUTER · GÜVENLİK DUVARI
      </text>
    </g>
  )
}

function Globe({
  x,
  y,
  reduced,
  remote,
}: {
  x: number
  y: number
  reduced: boolean
  remote: string | null
}): ReactElement {
  const R = 42
  return (
    // sabit düğüm — "son: X" değişken ama ikincil bilgi, ana gösterge şeridinde
    // ve paket etiketlerinde zaten aynı bilgi metin olarak mevcut; dekoratif
    <g transform={`translate(${x},${y})`} aria-hidden="true">
      <circle r={R} fill="#0a1120" stroke="#35485f" strokeWidth={1.5} />
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
        className="fill-tui-dim"
        fontFamily="ui-monospace, monospace"
        letterSpacing={1}
      >
        İNTERNET
      </text>
      {remote && (
        <text x={0} y={74} textAnchor="middle" fontSize={9} className="fill-tui-dim" fontFamily="ui-monospace, monospace">
          son: {remote}
        </text>
      )}
    </g>
  )
}

export function TrafficFlowDiagram({
  events,
  agents = [],
  devices = [],
  fill = false,
  editable = false,
  onAgentClick,
  onGroupClick,
  onAddGroup,
}: {
  events: TrafficEvent[]
  agents?: DiagramAgent[]
  /** gruplama için erişim katmanı cihazları (switch/AP/router/firewall) */
  devices?: DiagramDevice[]
  /** tam ekran: sahne genişler, daha çok sütun/satır + daha yoğun paket akışı */
  fill?: boolean
  /** düzenleme modu: agent/grup düğümleri tıklanabilir, "+ grup" görünür */
  editable?: boolean
  onAgentClick?: (name: string) => void
  onGroupClick?: (deviceId: number) => void
  onAddGroup?: () => void
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
  const bigFleet = count > 40

  // --- uplink gruplama: en az bir agent'ta uplinkId varsa gruplu düzen ---
  const devById = useMemo(() => {
    const m = new Map<number, DiagramDevice>()
    for (const d of devices) m.set(d.id, d)
    return m
  }, [devices])

  const bandsInput = useMemo(() => {
    const anyUplink = onlineAgents.some((a) => a.uplinkId != null && devById.has(a.uplinkId))
    if (!anyUplink) return null
    const byKey = new Map<string, number[]>()
    onlineAgents.forEach((a, i) => {
      const key = a.uplinkId != null && devById.has(a.uplinkId) ? String(a.uplinkId) : '__direct__'
      const arr = byKey.get(key)
      if (arr) arr.push(i)
      else byKey.set(key, [i])
    })
    type BandInput = {
      key: string
      label: string
      kind: string
      deviceId: number | null
      online: boolean
      members: number[]
    }
    const named: BandInput[] = [...byKey.entries()]
      .filter(([k]) => k !== '__direct__')
      .map(([k, members]) => {
        const d = devById.get(Number(k))!
        return { key: k, label: d.name, kind: d.kind, deviceId: d.id, online: d.online ?? false, members }
      })
      .sort((x, y) => x.label.localeCompare(y.label))
    const direct = byKey.get('__direct__')
    const out: BandInput[] = [...named]
    if (direct && direct.length)
      out.push({ key: '__direct__', label: 'Doğrudan', kind: '', deviceId: null, online: true, members: direct })
    return out
  }, [onlineAgents, devById])

  const s = useMemo(
    () => (bandsInput ? buildGroupedScene(bandsInput, fill) : buildScene(count, fill)),
    [bandsInput, count, fill],
  )
  const { W, FW_X, NET_X, H, midY, detail } = s
  const FW_IN: [number, number] = [FW_X - 44, midY]
  const FW_OUT: [number, number] = [FW_X + 44, midY]

  const resolveIdx = (name: string | undefined, hashKey: string): number => {
    if (count === 0) return -1
    if (name) return agentIndex.has(name) ? agentIndex.get(name)! : DROP
    return hashStr(hashKey) % count // yalnızca anonim uç (lan hedefi) — online havuzdan
  }

  // çok sütunda paket, düğümün kendisinden değil filonun SAĞ KENARINDAKİ dikey
  // "veri yolu"ndan çıkar/girer — düğüm hangi agent olduğunu flash ile gösterir.
  // Aksi halde paketler node ızgarasının üstünden çapraz geçip sahneyi
  // karıştırıyordu (çok-agent regresyonu).
  const busX = s.bandRight + 12
  const single = s.cols === 1 && !s.bands
  const pathFor = (dir: Dir, idxA: number, idxB: number): Array<[number, number]> => {
    const a = (i: number): [number, number] => {
      if (i < 0) return FW_IN
      const p = agentPos(i, s)
      return single ? [p.x + s.nodeW / 2, p.y] : [busX, p.y]
    }
    // çok sütunda ok önce veri yolu boyunca (x = busX) dikey ilerler, sonra
    // firewall'a girer — böylece üst/alt satırlardan çıkan oklar koridorda
    // çapraz kesişmez, "omurga → router" hissi verir. Gruplu düzende omurga
    // noktası, agent'ın grubunun switch/AP düğümüdür ("Doğrudan" → merkez).
    const spineOf = (i: number): [number, number] => {
      if (s.switchYByIdx && i >= 0) {
        const sy = s.switchYByIdx[i]
        if (sy != null) return [busX, sy]
      }
      return [busX, midY]
    }
    switch (dir) {
      case 'out':
        if (idxA < 0) return [FW_OUT, [(FW_X + NET_X) / 2, midY - 8], [NET_X - 36, midY]]
        return single
          ? [a(idxA), FW_IN, FW_OUT, [NET_X - 36, midY]]
          : [a(idxA), spineOf(idxA), FW_IN, FW_OUT, [NET_X - 36, midY]]
      case 'in':
        if (idxA < 0) return [[NET_X - 36, midY], [(FW_X + NET_X) / 2, midY + 8], FW_OUT]
        return single
          ? [[NET_X - 36, midY], FW_OUT, FW_IN, a(idxA)]
          : [[NET_X - 36, midY], FW_OUT, FW_IN, spineOf(idxA), a(idxA)]
      case 'lan': {
        const b = idxB >= 0 && idxB !== idxA ? idxB : (idxA + 1) % Math.max(1, count)
        if (single) return [a(idxA), [FW_X - 8, midY - 14], a(b)]
        // çok sütun: iki uç da veri yolunda — kenarda hafif bir kambur, firewall'a değmez
        const pa = a(idxA)
        const pb = a(b)
        return [pa, [busX - 30, (pa[1] + pb[1]) / 2], pb]
      }
      case 'log': {
        const to = idxA < 0 ? FW_IN : a(idxA)
        if (idxA < 0 || single)
          return [[FW_X, midY + 16], [(FW_X + to[0]) / 2, (midY + to[1]) / 2 + 8], to]
        return [[FW_X, midY + 16], spineOf(idxA), to]
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
    // kalabalık filoda paketler küçülür + hızlanır → "sürü" hissi, ekranda
    // daha az birikinti
    const rBase = bigFleet ? 3.6 : 4.5
    let r = rBase
    if (ev.weight && ev.weight > 0) r = Math.min(bigFleet ? 6.5 : 8, rBase + Math.log10(ev.weight) * 0.7)
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
    const spd = bigFleet ? 0.82 : 1
    const base = (dir === 'log' ? 1050 : dir === 'lan' ? 1750 : 2300) * spd
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
    // eşzamanlı ok tavanı filo büyüklüğüyle bir miktar artar — her ok artık
    // tam boy çizgi izi bıraktığı için nokta zamanına göre daha düşük tutulur
    const cap = Math.min(34, 18 + Math.floor(count / 7))
    if (packetsRef.current.length > cap) packetsRef.current.splice(0, packetsRef.current.length - cap)
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
      .slice(-(bigFleet ? 14 : 12))
      .reverse()
      .forEach((e, i) => spawnRef.current(e, now + Math.min(i, 14) * (bigFleet ? 130 : 180)))
  }, [events, agentIndex, bigFleet])

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
      .slice(0, bigFleet ? 2 : 4)
      .map((p) => p.id),
  )

  return (
    <div className={fill ? 'flex h-full flex-col' : undefined}>
      <style>{STYLE}</style>
      <div className={fill ? 'min-h-0 flex-1 overflow-hidden' : 'overflow-x-auto'}>
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className={fill ? 'h-full w-full' : 'w-full min-w-[680px]'}
          preserveAspectRatio={fill ? 'xMidYMid meet' : undefined}
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

          <rect aria-hidden="true" x={0} y={0} width={W} height={H} fill="url(#tfd-bg)" />

          {/* bölge başlıkları — dekoratif */}
          <g aria-hidden="true">
            <text x={s.headerX} y={28} textAnchor="middle" fontSize={11} className="fill-ink" fontFamily="ui-monospace, monospace" letterSpacing={1}>
              AGENT FİLOSU
            </text>
            <text x={s.headerX} y={H - 14} textAnchor="middle" fontSize={9} className="fill-tui-dim" fontFamily="ui-monospace, monospace">
              {count === 0
                ? 'aktif agent yok'
                : `${count} aktif${offlineCount > 0 ? ` · ${offlineCount} çevrimdışı gizli` : ''}${s.bands ? ` · ${s.bands.length} grup` : s.cols > 1 ? ` · ${s.cols} sütun` : ''}`}
            </text>
          </g>

          {/* "+ Switch / AP" — yalnızca düzenleme modunda */}
          {editable && onAddGroup && (
            <g
              transform={`translate(${s.headerX},44)`}
              role="button"
              tabIndex={0}
              aria-label="Switch / AP ekle"
              style={{ cursor: 'pointer' }}
              onClick={() => onAddGroup()}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onAddGroup()
                }
              }}
            >
              <rect x={-52} y={-9} width={104} height={16} fill="#0d1526" stroke="#38bdf8" strokeWidth={1} />
              <text x={0} y={2.5} textAnchor="middle" fontSize={9} fill="#38bdf8" fontFamily="ui-monospace, monospace">
                + Switch / AP
              </text>
            </g>
          )}

          {/* altyapı bağlantıları — dekoratif */}
          <g aria-hidden="true">
            {count === 0 ? (
              <line
                x1={s.headerX}
                y1={midY}
                x2={FW_IN[0]}
                y2={FW_IN[1]}
                stroke="#1e3a5f"
                strokeWidth={1.5}
                strokeDasharray="2 6"
                className={reduced ? '' : 'tfd-dash'}
              />
            ) : s.bands ? (
              // gruplu: her grubun switch/AP düğümünden (veya "Doğrudan" bant
              // merkezinden) güvenlik duvarına tek çizgi
              s.bands.map((b) => (
                <line
                  key={b.key}
                  x1={busX}
                  y1={b.switchY}
                  x2={FW_IN[0]}
                  y2={FW_IN[1]}
                  stroke="#17324f"
                  strokeWidth={1}
                  strokeDasharray="2 6"
                  className={reduced ? '' : 'tfd-dash'}
                />
              ))
            ) : s.cols === 1 ? (
              onlineAgents.map((_, i) => {
                const p = agentPos(i, s)
                return (
                  <line
                    key={i}
                    x1={p.x + s.nodeW / 2}
                    y1={p.y}
                    x2={FW_IN[0]}
                    y2={FW_IN[1]}
                    stroke="#17324f"
                    strokeWidth={1}
                    strokeDasharray="2 6"
                    className={reduced ? '' : 'tfd-dash'}
                  />
                )
              })
            ) : (
              Array.from({ length: s.cols }, (_, c) => {
                const n = colCount(c, count, s)
                const railX = s.bandLeft + s.colGap * c + s.colGap / 2 + s.nodeW / 2 + 5
                const y0 = TOP + s.rowH * 0.2
                const y1 = TOP + s.rowH * (n - 0.2)
                const cy = (y0 + y1) / 2
                return (
                  <g key={c}>
                    <line x1={railX} y1={y0} x2={railX} y2={y1} stroke="#17324f" strokeWidth={1} />
                    <line
                      x1={railX}
                      y1={cy}
                      x2={FW_IN[0]}
                      y2={FW_IN[1]}
                      stroke="#17324f"
                      strokeWidth={1}
                      strokeDasharray="2 6"
                      className={reduced ? '' : 'tfd-dash'}
                    />
                  </g>
                )
              })
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

          {/* grup başlıkları + switch/AP düğümleri (gruplu düzen) */}
          {s.bands?.map((b, gi) => (
            <g key={b.key}>
              {gi > 0 && (
                <line
                  x1={s.bandLeft - 6}
                  y1={b.y0}
                  x2={s.bandRight + 24}
                  y2={b.y0}
                  stroke="#232b3a"
                  strokeWidth={1}
                  aria-hidden="true"
                />
              )}
              <text
                x={s.bandLeft}
                y={b.y0 + 11}
                fontSize={9}
                className={b.deviceId === null ? 'fill-tui-dim' : 'fill-ink'}
                fontFamily="ui-monospace, monospace"
                letterSpacing={0.5}
              >
                {(b.deviceId === null ? '' : (b.kind || 'grup').toUpperCase() + ' ') + b.label}
                <tspan className="fill-tui-dim"> · {b.members.length}</tspan>
              </text>
              {b.deviceId !== null && (
                <GroupSwitch
                  x={busX}
                  y={b.switchY}
                  band={b}
                  onActivate={editable && onGroupClick ? () => onGroupClick(b.deviceId as number) : undefined}
                />
              )}
            </g>
          ))}

          {onlineAgents.map((a, i) => {
            const p = agentPos(i, s)
            return (
              <AgentNode
                key={a.name}
                x={p.x}
                y={p.y}
                agent={a}
                detail={detail}
                nodeW={s.nodeW}
                flash={!reduced && flashRef.current.has(i)}
                onActivate={editable && onAgentClick ? () => onAgentClick(a.name) : undefined}
              />
            )
          })}
          {count === 0 && (
            <text x={s.headerX} y={midY} textAnchor="middle" fontSize={10} className="fill-tui-dim" fontFamily="ui-monospace, monospace" aria-hidden="true">
              aktif agent bekleniyor
            </text>
          )}
          <Firewall x={FW_X} y={midY} reduced={reduced} />
          <Globe x={NET_X} y={midY} reduced={reduced} remote={netEnd} />

          {/* uçan "ok"lar — her olay için, olayın üretildiği agent'tan
              router/internet'e doğru yol boyunca çizilen bir çizgi + uçta ok
              başı. Çizgi, ok hedefe varana kadar arkada iz olarak kalır, sonra
              tümü sönerek kaybolur. Taşıdığı bilgi (yön/sayaç) alttaki gösterge
              şeridinde zaten metin olarak mevcut. */}
          <g aria-hidden="true">
          {live.map((p) => {
            const raw = (now - p.t0) / p.dur
            // ok hedefe raw≈0.82'de ulaşır; kalan süre tam çizili ok sönerek durur
            const prog = Math.min(1, raw / 0.82)
            const u = easeInOut(prog)
            const dist = u * p.total
            const head = sampleAt(p.pts, p.seg, p.total, dist)
            const fade = Math.min(1, raw / 0.05) * Math.min(1, (1 - raw) / 0.22)
            const col = DIR_COLOR[p.dir]
            const deg = (head.ang * 180) / Math.PI
            const trail = polySubPath(p.pts, p.seg, 0, dist) // olaydan buraya kadar izlenen yol
            const tip = polySubPath(p.pts, p.seg, Math.max(0, dist - 84), dist) // parlak baş parçası
            const ah = Math.max(5, p.r + 2) // ok başı boyu
            return (
              <g key={p.id} opacity={fade}>
                {trail && (
                  <path
                    d={trail}
                    fill="none"
                    stroke={col}
                    strokeWidth={1.2}
                    strokeOpacity={0.32}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                )}
                {tip && (
                  <path
                    d={tip}
                    fill="none"
                    stroke={col}
                    strokeWidth={2}
                    strokeOpacity={0.9}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                )}
                <g transform={`translate(${head.x},${head.y}) rotate(${deg})`}>
                  <path
                    d={`M ${ah + 3} 0 L ${ah - 4} ${ah * 0.6} L ${ah - 1.5} 0 L ${ah - 4} ${-ah * 0.6} Z`}
                    fill={col}
                    filter="url(#tfd-glow)"
                  />
                </g>
                {labelIds.has(p.id) && raw < 0.9 && (
                  <g transform={`translate(${head.x},${head.y - 14})`} opacity={Math.min(1, (0.9 - raw) / 0.22)}>
                    <rect
                      x={-p.label.length * 3.15 - 5}
                      y={-9}
                      width={p.label.length * 6.3 + 10}
                      height={15}

                      fill="#0b1220"
                      stroke={col}
                      strokeOpacity={0.4}
                    />
                    <text textAnchor="middle" y={2} fontSize={9} className="fill-ink" fontFamily="ui-monospace, monospace">
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
      <div className={`mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[11px]${fill ? ' shrink-0' : ''}`}>
        {(['out', 'in', 'lan', 'log'] as Dir[]).map((d) => (
          <span key={d} className="flex items-center gap-1.5">
            <span className="size-2 rounded-full" style={{ background: DIR_COLOR[d] }} />
            <span className="text-tui-dim">{DIR_LABEL[d]}</span>
            <span className="font-mono text-tui-dim">{tally[d]}</span>
          </span>
        ))}
        <span className="ml-auto font-mono text-tui-dim">
          {reduced ? 'hareket azaltma açık' : `${live.length} aktif paket`}
        </span>
      </div>
    </div>
  )
}
