import { useEffect, useState } from 'react'
import type { CompareResponse, DayTotal, HourAvg } from '../types'
import { formatBits, formatBytes } from '../lib/format'

const W = 520
const H = 200
const PAD = { top: 12, right: 8, bottom: 26, left: 48 }

function totalGB(d: DayTotal): number {
  return ((d.avg_bps_in + d.avg_bps_out) / 8) * d.samples / 1e9
}

function dayLabel(ts: number): string {
  return new Date(ts * 1000).toLocaleDateString('tr-TR', { day: '2-digit', month: '2-digit' })
}

function DayBars({ days }: { days: DayTotal[] }) {
  const shown = days.slice(-7)
  if (shown.length === 0) {
    return <p className="py-14 text-center text-sm text-slate-600">Günlük veri yok.</p>
  }
  const maxGB = Math.max(0.01, ...shown.map(totalGB))
  const iw = W - PAD.left - PAD.right
  const ih = H - PAD.top - PAD.bottom
  const groupW = iw / shown.length
  const barW = Math.min(18, groupW * 0.32)

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Günlük toplam transfer">
      {[0, 0.5, 1].map((f) => {
        const y = PAD.top + ih - f * ih
        return (
          <g key={f}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y} y2={y} stroke="#1e293b" />
            <text x={PAD.left - 6} y={y + 4} textAnchor="end" className="fill-slate-500" fontSize="10">
              {formatBytes(maxGB * f)}
            </text>
          </g>
        )
      })}
      {shown.map((d, i) => {
        const cx = PAD.left + groupW * i + groupW / 2
        const gbIn = (d.avg_bps_in / 8) * d.samples / 1e9
        const gbOut = (d.avg_bps_out / 8) * d.samples / 1e9
        const hIn = (gbIn / maxGB) * ih
        const hOut = (gbOut / maxGB) * ih
        const yBase = PAD.top + ih
        return (
          <g key={d.day}>
            <rect
              x={cx - barW - 1} y={yBase - hIn} width={barW} height={hIn} rx={2}
              fill="#22d3ee" opacity={0.9}
            >
              <title>indirme: {formatBytes(gbIn * 1e9)}</title>
            </rect>
            <rect
              x={cx + 1} y={yBase - hOut} width={barW} height={hOut} rx={2}
              fill="#a78bfa" opacity={0.9}
            >
              <title>gönderme: {formatBytes(gbOut * 1e9)}</title>
            </rect>
            <text x={cx} y={H - 8} textAnchor="middle" className="fill-slate-500" fontSize="10">
              {dayLabel(d.day)}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

function HourOverlay({ today, yesterday }: { today: HourAvg[]; yesterday: HourAvg[] }) {
  const hasData = today.some((h) => h.bps_in > 0 || h.bps_out > 0) || yesterday.some((h) => h.bps_in > 0 || h.bps_out > 0)
  if (!hasData) {
    return <p className="py-14 text-center text-sm text-slate-600">Saatlik veri yok.</p>
  }
  const maxBps = Math.max(
    1,
    ...today.map((h) => Math.max(h.bps_in, h.bps_out)),
    ...yesterday.map((h) => Math.max(h.bps_in, h.bps_out)),
  )
  const iw = W - PAD.left - PAD.right
  const ih = H - PAD.top - PAD.bottom
  const x = (hour: number) => PAD.left + (hour / 23) * iw
  const y = (v: number) => PAD.top + ih - (v / maxBps) * ih

  const path = (series: HourAvg[], get: (h: HourAvg) => number) =>
    series.map((h, i) => `${i === 0 ? 'M' : 'L'}${x(h.hour).toFixed(1)},${y(get(h)).toFixed(1)}`).join(' ')

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="w-full" role="img" aria-label="Bugün vs dün saatlik verim">
      {[0, 0.5, 1].map((f) => {
        const yy = PAD.top + ih - f * ih
        return (
          <g key={f}>
            <line x1={PAD.left} x2={W - PAD.right} y1={yy} y2={yy} stroke="#1e293b" />
            <text x={PAD.left - 6} y={yy + 4} textAnchor="end" className="fill-slate-500" fontSize="10">
              {formatBits(maxBps * f)}
            </text>
          </g>
        )
      })}
      {[0, 6, 12, 18, 23].map((h) => (
        <text key={h} x={x(h)} y={H - 8} textAnchor={h === 0 ? 'start' : h === 23 ? 'end' : 'middle'} className="fill-slate-500" fontSize="10">
          {h}:00
        </text>
      ))}
      <path d={path(yesterday, (h) => h.bps_in)} fill="none" stroke="#a78bfa" strokeWidth="1.5" strokeDasharray="4 3" opacity={0.8} />
      <path d={path(today, (h) => h.bps_in)} fill="none" stroke="#22d3ee" strokeWidth="2" />
    </svg>
  )
}

export function CompareCard() {
  const [data, setData] = useState<CompareResponse | null>(null)

  useEffect(() => {
    const load = () =>
      fetch('/api/compare?days=7')
        .then((r) => r.json())
        .then(setData)
        .catch(() => {})
    load()
    const id = window.setInterval(load, 60_000)
    return () => window.clearInterval(id)
  }, [])

  const todayTotal = data?.days.find((d) => d.day === new Date().setHours(0, 0, 0, 0) / 1000)
  const yesterdayTotal = data?.days.find(
    (d) => d.day === new Date(Date.now() - 86400_000).setHours(0, 0, 0, 0) / 1000,
  )
  let change: number | null = null
  if (todayTotal && yesterdayTotal && yesterdayTotal.samples > 0) {
    change = ((totalGB(todayTotal) - totalGB(yesterdayTotal)) / totalGB(yesterdayTotal)) * 100
  }

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-4 text-xs">
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm bg-cyan-400" /> indirme
        </span>
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm bg-violet-400" /> gönderme
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-0 w-4 border-t-2 border-dashed border-violet-400" /> dün
        </span>
        {change !== null && (
          <span
            className={`ml-auto rounded px-2.5 py-1 font-mono ring-1 ${
              change >= 0
                ? 'bg-amber-500/10 text-amber-400 ring-amber-500/30'
                : 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/30'
            }`}
          >
            bugün düne göre: {change >= 0 ? '+' : ''}{change.toFixed(0)}%
          </span>
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-slate-500">Son 7 Gün (toplam transfer)</p>
          {data ? <DayBars days={data.days} /> : <p className="py-14 text-center text-sm text-slate-600">Yükleniyor…</p>}
        </div>
        <div>
          <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-slate-500">Bugün vs Dün (saatlik indirme ortalaması)</p>
          {data ? (
            <HourOverlay today={data.today_hours} yesterday={data.yesterday_hours} />
          ) : (
            <p className="py-14 text-center text-sm text-slate-600">Yükleniyor…</p>
          )}
        </div>
      </div>
    </div>
  )
}
