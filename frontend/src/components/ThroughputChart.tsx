import type { Bucket } from '../types'
import { formatBits } from '../lib/format'

interface Props {
  history: Bucket[]
  running?: boolean
  /** gercek zaman penceresi (dakika) — canli gorunum icin 2 */
  rangeMinutes?: number
  subtitle?: string
}

export function ThroughputChart({ history, running = false, rangeMinutes = 2, subtitle }: Props) {
  const W = 1000
  const H = 260
  const PAD = { top: 16, right: 12, bottom: 24, left: 56 }
  const iw = W - PAD.left - PAD.right
  const ih = H - PAD.top - PAD.bottom

  const data = history.slice(-120)
  const maxBps = Math.max(1, ...data.map((b) => Math.max(b.in, b.out, b.local))) * 8
  const ticks = 4

  const x = (i: number, n: number) => PAD.left + (n <= 1 ? iw : (i / (n - 1)) * iw)
  const y = (v: number) => PAD.top + ih - (v / maxBps) * ih

  const line = (get: (b: Bucket) => number) =>
    data.map((b, i) => `${i === 0 ? 'M' : 'L'}${x(i, data.length).toFixed(1)},${y(get(b) * 8).toFixed(1)}`).join(' ')

  const area = (get: (b: Bucket) => number) => {
    if (data.length === 0) return ''
    return `${line(get)} L${x(data.length - 1, data.length).toFixed(1)},${(PAD.top + ih).toFixed(1)} L${x(0, data.length).toFixed(1)},${(PAD.top + ih).toFixed(1)} Z`
  }

  const last = data[data.length - 1]
  const lastIn = (last?.in ?? 0) * 8
  const lastOut = (last?.out ?? 0) * 8

  const yTicks = Array.from({ length: ticks + 1 }, (_, i) => (maxBps / ticks) * i)
  const windowSecs = rangeMinutes * 60
  const xTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => f * windowSecs)

  const xLabel = (secs: number) => {
    if (secs === 0) return 'şimdi'
    if (secs < 90) return `-${secs} sn`
    return `-${Math.round(secs / 60)} dk`
  }

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-baseline gap-x-6 gap-y-1">
        <div className="flex items-center gap-2">
          <span className="size-2.5 rounded-sm bg-cyan-400" />
          <span className="text-xs text-slate-400">İndirilen</span>
          <span className="font-mono text-sm font-semibold text-cyan-300">{formatBits(lastIn)}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="size-2.5 rounded-sm bg-violet-400" />
          <span className="text-xs text-slate-400">Gönderilen</span>
          <span className="font-mono text-sm font-semibold text-violet-300">{formatBits(lastOut)}</span>
        </div>
        <span className="ml-auto text-xs text-slate-500">
          {subtitle ?? (running ? 'son 2 dakika · saniyelik örnekleme' : 'yakalama durduruldu')}
        </span>
      </div>

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Trafik grafiği">
        {yTicks.map((v, i) => (
          <g key={i}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} stroke="#1e293b" strokeWidth="1" />
            <text x={PAD.left - 8} y={y(v) + 4} textAnchor="end" className="fill-slate-500" fontSize="10">
              {formatBits(v).replace(' bit/s', 'b').replace('bit/s', 'b')}
            </text>
          </g>
        ))}

        {xTicks.map((s) => {
          const px = PAD.left + (s / windowSecs) * iw
          if (px > W - PAD.right + 1) return null
          return (
            <text key={s} x={px} y={H - 6} textAnchor={s === 0 ? 'start' : 'middle'} className="fill-slate-500" fontSize="10">
              {xLabel(s)}
            </text>
          )
        })}

        {data.length > 1 && (
          <>
            <path d={area((b) => b.local + b.in + b.out)} fill="#a78bfa" fillOpacity="0.08" />
            <path d={area((b) => b.in)} fill="#22d3ee" fillOpacity="0.10" />
            <path d={line((b) => b.out)} fill="none" stroke="#a78bfa" strokeWidth="1.5" />
            <path d={line((b) => b.in)} fill="none" stroke="#22d3ee" strokeWidth="2" />
          </>
        )}

        {data.length <= 1 && (
          <text x={W / 2} y={H / 2} textAnchor="middle" className="fill-slate-600" fontSize="13">
            Veri bekleniyor…
          </text>
        )}
      </svg>
    </div>
  )
}
