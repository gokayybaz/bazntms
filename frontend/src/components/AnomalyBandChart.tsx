import { formatBits } from '../lib/format'

// Anomali "beklenen bant" grafiği (S22.5). Baseline bir zaman serisi DEĞİL —
// mevsimsel kova başına (mean ± Nσ) profil. Grafik günün/haftanın profilini
// çizer; içinde bulunulan kova vurgulanır ve o an ölçülen değer üstüne konur.
// TUI: basamaklı çizgi + ince bant dolgusu, mono tick, box-drawing hissi.

export interface BaselineRow {
  bucket: number
  n: number
  mean: number
  m2: number
}

export interface ActivePoint {
  bucket: number
  cur: number
  z: number
}

const SIGMA = 2 // bant yarı-genişliği

function std(r: BaselineRow): number {
  return r.n < 2 ? 0 : Math.sqrt(Math.max(0, r.m2 / r.n))
}

function bucketCount(seasonality: string): number {
  if (seasonality === 'dow') return 168
  if (seasonality === 'weekday') return 48
  return 24
}

function bucketLabel(b: number, seasonality: string): string {
  if (seasonality === 'hourly') return `${b}`
  if (seasonality === 'weekday') return b < 24 ? `Hİ ${b}` : `HS ${b - 24}`
  const day = ['Pz', 'Pt', 'Sa', 'Ça', 'Pe', 'Cu', 'Ct'][Math.floor(b / 24)]
  return `${day} ${b % 24}`
}

export function AnomalyBandChart({
  rows,
  active,
  seasonality,
  metric,
  currentBucket,
}: {
  rows: BaselineRow[]
  active: ActivePoint[]
  seasonality: string
  metric: string
  currentBucket: number
}) {
  const W = 1000
  const H = 260
  const PAD = { top: 14, right: 12, bottom: 26, left: 62 }
  const iw = W - PAD.left - PAD.right
  const ih = H - PAD.top - PAD.bottom

  const N = bucketCount(seasonality)
  const byBucket = new Map(rows.map((r) => [r.bucket, r]))
  const fmt =
    metric === 'dns_qps'
      ? (v: number) => `${v.toFixed(v < 10 ? 1 : 0)}`
      : (v: number) => formatBits(v).replace(/ ?bit\/s/, '').replace(/ ?Kbit\/s/, 'K').replace(/ ?Mbit\/s/, 'M').replace(/ ?Gbit\/s/, 'G')
  const unit = metric === 'dns_qps' ? 'sorgu/sn' : 'bps'

  const upper = (r: BaselineRow) => r.mean + SIGMA * std(r)
  let maxV = 1
  for (const r of rows) maxV = Math.max(maxV, upper(r))
  for (const a of active) maxV = Math.max(maxV, a.cur)

  const x = (b: number) => PAD.left + (N <= 1 ? iw / 2 : (b / (N - 1)) * iw)
  const y = (v: number) => PAD.top + ih - (Math.min(v, maxV) / maxV) * ih

  // bant çokgeni (üst kenar ileri, alt kenar geri)
  const present = [...byBucket.keys()].sort((a, b) => a - b)
  let bandPath = ''
  if (present.length > 0) {
    bandPath = 'M' + present.map((b) => `${x(b).toFixed(1)},${y(upper(byBucket.get(b)!)).toFixed(1)}`).join(' L')
    for (let i = present.length - 1; i >= 0; i--) {
      const b = present[i]
      const r = byBucket.get(b)!
      const lo = Math.max(0, r.mean - SIGMA * std(r))
      bandPath += ` L${x(b).toFixed(1)},${y(lo).toFixed(1)}`
    }
    bandPath += ' Z'
  }
  const meanPath =
    present.length > 0
      ? 'M' + present.map((b) => `${x(b).toFixed(1)},${y(byBucket.get(b)!.mean).toFixed(1)}`).join(' L')
      : ''

  const yTicks = Array.from({ length: 5 }, (_, i) => (maxV / 4) * i)
  const xTickStep = seasonality === 'dow' ? 24 : seasonality === 'weekday' ? 6 : 3
  const xTicks: number[] = []
  for (let b = 0; b < N; b += xTickStep) xTicks.push(b)


  return (
    <div className="font-mono">
      <div className="mb-2 flex flex-wrap items-baseline gap-x-5 gap-y-1 text-[11px]">
        <span className="flex items-center gap-1.5">
          <span className="text-tx">▬</span>
          <span className="text-tui-dim">beklenen ±{SIGMA}σ bandı</span>
        </span>
        <span className="flex items-center gap-1.5">
          <span className="text-rule-hi">┊</span>
          <span className="text-tui-dim">şu anki kova</span>
        </span>
        {active.length > 0 && (
          <span className="flex items-center gap-1.5">
            <span className="text-amber-400">■</span>
            <span className="text-tui-dim">aktif sapma (ölçülen)</span>
          </span>
        )}
        <span className="ml-auto text-[10px] text-tui-dim">
          {seasonality === 'hourly' ? 'saat-of-day' : seasonality === 'weekday' ? 'hafta içi/sonu × saat' : 'haftanın günü × saat'} · {unit}
        </span>
      </div>

      <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Anomali beklenen bant grafiği">
        {yTicks.map((v, i) => (
          <g key={`y${i}`}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} stroke="#232b3a" strokeWidth="1" />
            <text x={PAD.left - 8} y={y(v) + 3.5} textAnchor="end" fill="#8794a8" fontSize="9" fontFamily="ui-monospace, monospace">
              {fmt(v)}
            </text>
          </g>
        ))}
        {xTicks.map((b) => (
          <g key={`x${b}`}>
            <line x1={x(b)} x2={x(b)} y1={PAD.top} y2={PAD.top + ih} stroke="#232b3a" strokeWidth="1" />
            <text x={x(b)} y={H - 8} textAnchor="middle" fill="#8794a8" fontSize="9" fontFamily="ui-monospace, monospace">
              {bucketLabel(b, seasonality)}
            </text>
          </g>
        ))}

        {/* şu anki kova sütunu */}
        {currentBucket >= 0 && currentBucket < N && (
          <line x1={x(currentBucket)} x2={x(currentBucket)} y1={PAD.top} y2={PAD.top + ih} stroke="#35485f" strokeWidth="1.5" strokeDasharray="3 3" />
        )}

        {bandPath && <path d={bandPath} fill="#a78bfa" fillOpacity="0.14" stroke="none" />}
        {meanPath && <path d={meanPath} fill="none" stroke="#a78bfa" strokeWidth="1.4" />}

        {/* aktif sapma noktaları */}
        {active.map((a) => (
          <rect
            key={`a${a.bucket}`}
            x={x(a.bucket) - 3}
            y={y(a.cur) - 3}
            width="6"
            height="6"
            fill={Math.abs(a.z) >= 4 ? '#fb7185' : '#fbbf24'}
          />
        ))}

        {present.length === 0 && (
          <text x={W / 2} y={H / 2} textAnchor="middle" fill="#8794a8" fontSize="11" fontFamily="ui-monospace, monospace">
            Baseline henüz ısınıyor — yeterli geçmiş veri yok
          </text>
        )}
      </svg>
    </div>
  )
}
