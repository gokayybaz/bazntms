import type { Bucket } from '../types'
import { formatBits } from '../lib/format'

interface Props {
  history: Bucket[]
  running?: boolean
  /** gercek zaman penceresi (dakika) — canli gorunum icin 2 */
  rangeMinutes?: number
  subtitle?: string
}

// TUI: basamaklı (step) çizgi, alan dolgusu / gradyan YOK, mono tick etiketleri,
// karakter-ızgara zemin (dikey + yatay grid), son noktada vurgu.
export function ThroughputChart({ history, running = false, rangeMinutes = 2, subtitle }: Props) {
  const W = 1000
  const H = 240
  const PAD = { top: 14, right: 12, bottom: 22, left: 58 }
  const iw = W - PAD.left - PAD.right
  const ih = H - PAD.top - PAD.bottom

  const data = history.slice(-120)
  const maxBps = Math.max(1, ...data.map((b) => Math.max(b.in, b.out, b.local))) * 8
  const ticks = 4

  const x = (i: number, n: number) => PAD.left + (n <= 1 ? iw : (i / (n - 1)) * iw)
  const y = (v: number) => PAD.top + ih - (v / maxBps) * ih

  // basamaklı çizgi: her noktaya yatay git, sonra dikey (step-after)
  const stepLine = (get: (b: Bucket) => number) => {
    if (data.length === 0) return ''
    let d = `M${x(0, data.length).toFixed(1)},${y(get(data[0]) * 8).toFixed(1)}`
    for (let i = 1; i < data.length; i++) {
      const px = x(i, data.length).toFixed(1)
      d += ` L${px},${y(get(data[i - 1]) * 8).toFixed(1)} L${px},${y(get(data[i]) * 8).toFixed(1)}`
    }
    return d
  }

  const last = data[data.length - 1]
  const lastIn = (last?.in ?? 0) * 8
  const lastOut = (last?.out ?? 0) * 8
  const lastX = x(data.length - 1, data.length)

  const yTicks = Array.from({ length: ticks + 1 }, (_, i) => (maxBps / ticks) * i)
  const windowSecs = rangeMinutes * 60
  const xTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => f * windowSecs)

  const xLabel = (secs: number) => {
    if (secs === 0) return 'şimdi'
    if (secs < 90) return `-${secs}s`
    return `-${Math.round(secs / 60)}d`
  }

  return (
    <div className="font-mono">
      <div className="mb-2 flex flex-wrap items-baseline gap-x-5 gap-y-1 text-[11px]">
        <span className="flex items-center gap-1.5">
          <span className="text-rx">▉</span>
          <span className="text-tui-dim">İndirilen</span>
          <span className="font-semibold text-rx">{formatBits(lastIn)}</span>
        </span>
        <span className="flex items-center gap-1.5">
          <span className="text-tx">▉</span>
          <span className="text-tui-dim">Gönderilen</span>
          <span className="font-semibold text-tx">{formatBits(lastOut)}</span>
        </span>
        <span className="ml-auto text-[10px] text-tui-dim">
          {subtitle ?? (running ? 'son 2 dakika · saniyelik örnekleme' : 'yakalama durduruldu')}
        </span>
      </div>

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Trafik grafiği">
        {/* yatay grid + y ekseni etiketleri */}
        {yTicks.map((v, i) => (
          <g key={`y${i}`}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} stroke="#232b3a" strokeWidth="1" />
            <text x={PAD.left - 8} y={y(v) + 3.5} textAnchor="end" fill="#8794a8" fontSize="9" fontFamily="ui-monospace, monospace">
              {formatBits(v).replace(/ ?bit\/s/, 'b').replace(/ ?Kbit\/s/, 'K').replace(/ ?Mbit\/s/, 'M').replace(/ ?Gbit\/s/, 'G')}
            </text>
          </g>
        ))}
        {/* dikey grid (karakter-ızgara hissi) */}
        {xTicks.map((s) => {
          const px = PAD.left + (s / windowSecs) * iw
          if (px > W - PAD.right + 1) return null
          return <line key={`vg${s}`} x1={px} x2={px} y1={PAD.top} y2={PAD.top + ih} stroke="#232b3a" strokeWidth="1" />
        })}
        {xTicks.map((s) => {
          const px = PAD.left + (s / windowSecs) * iw
          if (px > W - PAD.right + 1) return null
          return (
            <text key={`x${s}`} x={px} y={H - 5} textAnchor={s === 0 ? 'start' : 'middle'} fill="#8794a8" fontSize="9" fontFamily="ui-monospace, monospace">
              {xLabel(s)}
            </text>
          )
        })}

        {data.length > 1 && (
          <>
            <path d={stepLine((b) => b.out)} fill="none" stroke="#a78bfa" strokeWidth="1.5" />
            <path d={stepLine((b) => b.in)} fill="none" stroke="#22d3ee" strokeWidth="1.5" />
            {/* son nokta vurgusu */}
            <rect x={lastX - 2.5} y={y(lastOut) - 2.5} width="5" height="5" fill="#a78bfa" />
            <rect x={lastX - 2.5} y={y(lastIn) - 2.5} width="5" height="5" fill="#22d3ee" />
          </>
        )}

        {data.length <= 1 && (
          <text x={W / 2} y={H / 2} textAnchor="middle" fill="#8794a8" fontSize="11" fontFamily="ui-monospace, monospace">
            Veri bekleniyor…
          </text>
        )}
      </svg>
    </div>
  )
}
