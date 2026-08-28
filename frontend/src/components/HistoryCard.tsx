import { useCallback, useEffect, useState } from 'react'
import type { HistoryResponse } from '../types'
import { formatBits, formatBytes } from '../lib/format'
import { ThroughputChart } from './ThroughputChart'

const RANGES = [
  { label: '15 dk', minutes: 15 },
  { label: '1 saat', minutes: 60 },
  { label: '6 saat', minutes: 360 },
  { label: '24 saat', minutes: 1440 },
]

export function HistoryCard({ refreshKey }: { refreshKey: number }) {
  const [minutes, setMinutes] = useState(60)
  const [data, setData] = useState<HistoryResponse | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async (m: number) => {
    setLoading(true)
    try {
      const res = await fetch(`/api/history?minutes=${m}`)
      setData(await res.json())
    } catch {
      /* yoksay */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load(minutes)
  }, [minutes, refreshKey, load])

  const t = data?.totals
  const chartBuckets = (data?.buckets ?? []).map((b) => ({
    ts: b.ts,
    in: b.in,
    out: b.out,
    local: b.local,
    packets: Math.round(b.pps),
  }))

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="flex rounded-lg border border-slate-700/80 p-0.5">
          {RANGES.map((r) => (
            <button
              key={r.minutes}
              onClick={() => setMinutes(r.minutes)}
              className={`rounded-md px-3 py-1 text-xs font-medium transition ${
                minutes === r.minutes ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
        <span className="ml-auto text-xs text-slate-500">
          {t
            ? `ort ↓ ${formatBits(t.avg_bps_in)} · ↑ ${formatBits(t.avg_bps_out)} | zirve ↓ ${formatBits(t.peak_bps_in)} · ↑ ${formatBits(t.peak_bps_out)} | toplam ${formatBytes(((t.avg_bps_in + t.avg_bps_out) / 8) * t.seconds)}`
            : ''}
        </span>
      </div>

      {loading && !data ? (
        <p className="py-16 text-center text-sm text-slate-600">Yükleniyor…</p>
      ) : data && data.buckets.length > 0 ? (
        <ThroughputChart
          history={chartBuckets}
          rangeMinutes={data.range_minutes}
          subtitle={`veritabanı kaydı · ${data.buckets.length} örnek · ${formatBytes(data.db_bytes)}`}
        />
      ) : (
        <p className="py-16 text-center text-sm text-slate-600">
          Bu aralıkta kayıt yok — yakalama açıkken veriler her saniye kaydedilir.
        </p>
      )}
    </div>
  )
}
