import { useMemo, useState } from 'react'
import { Panel } from '../components/Panel'
import { TuiTable, type TuiColumn } from '../components/TuiTable'
import { AnomalyBandChart, type BaselineRow, type ActivePoint } from '../components/AnomalyBandChart'
import { usePolledJson } from '../lib/usePolledJson'
import { formatBits, formatNum } from '../lib/format'

// GET /api/v1/anomaly/* yanıt tipleri (yerel — CLAUDE.md konvansiyonu).
interface BaselineResp {
  dim: string
  metric: string
  seasonality: string
  rows: BaselineRow[]
}
interface Deviation {
  metric: string
  dim: string
  key: string
  scope: string
  bucket: number
  mean: number
  std: number
  cur: number
  z: number
  n: number
}
interface ActiveResp {
  deviations: Deviation[] | null
}

const DIMS = [
  { id: 'fleet', label: 'Filo' },
  { id: 'local', label: 'Hub yerel' },
] as const
const METRICS = [
  { id: 'bps', label: 'Bant genişliği' },
  { id: 'dns_qps', label: 'DNS sorgu hızı' },
  { id: 'proc_bps', label: 'Süreç trafiği' },
] as const

function fmtVal(metric: string, v: number): string {
  return metric === 'dns_qps' ? `${v.toFixed(v < 10 ? 1 : 0)} sorgu/sn` : formatBits(v)
}

function seasonalBucketNow(seasonality: string): number {
  const d = new Date()
  const h = d.getHours()
  if (seasonality === 'dow') return d.getDay() * 24 + h
  if (seasonality === 'weekday') {
    const wd = d.getDay()
    return wd === 0 || wd === 6 ? 24 + h : h
  }
  return h
}

export function AnomalyPage() {
  const [dim, setDim] = useState<string>('fleet')
  const [metric, setMetric] = useState<string>('bps')

  const { data: base } = usePolledJson<BaselineResp>(
    `/api/v1/anomaly/baseline?dim=${dim}&metric=${metric}`,
    30_000,
  )
  const { data: act } = usePolledJson<ActiveResp>('/api/v1/anomaly/active', 15_000)

  const seasonality = base?.seasonality ?? 'weekday'
  const curBucket = seasonalBucketNow(seasonality)
  const rows = base?.rows ?? []
  const deviations = useMemo(() => act?.deviations ?? [], [act])

  // grafiğe geçen aktif noktalar: yalnız seçili dim+metric, mevcut kova
  const chartActive: ActivePoint[] = useMemo(
    () =>
      deviations
        .filter((d) => d.dim === dim && d.metric === metric)
        .map((d) => ({ bucket: d.bucket, cur: d.cur, z: d.z })),
    [deviations, dim, metric],
  )

  const columns: TuiColumn<Deviation>[] = [
    { key: 'scope', header: 'Kapsam', width: '14rem', render: (d) => d.scope },
    {
      key: 'metric',
      header: 'Metrik',
      width: '10rem',
      render: (d) => METRICS.find((m) => m.id === d.metric)?.label ?? d.metric,
    },
    {
      key: 'z',
      header: 'z',
      align: 'right',
      width: '5rem',
      sortValue: (d) => Math.abs(d.z),
      render: (d) => (
        <span className={Math.abs(d.z) >= 4 ? 'text-rose-400' : 'text-amber-400'}>
          {d.z > 0 ? '+' : ''}
          {d.z.toFixed(1)}
        </span>
      ),
    },
    {
      key: 'cur',
      header: 'Şu an',
      align: 'right',
      width: '9rem',
      sortValue: (d) => d.cur,
      render: (d) => fmtVal(d.metric, d.cur),
    },
    {
      key: 'exp',
      header: 'Beklenen',
      align: 'right',
      width: '12rem',
      sortValue: (d) => d.mean,
      render: (d) => (
        <span className="text-tui-dim">
          {fmtVal(d.metric, d.mean)} ± {fmtVal(d.metric, d.std)}
        </span>
      ),
    },
    { key: 'n', header: 'Örnek', align: 'right', width: '6rem', sortValue: (d) => d.n, render: (d) => formatNum(d.n) },
  ]

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex flex-wrap items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Anomali</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">
          mevsimsel z-skoru baseline · filo / saha / agent · bant genişliği + DNS + süreç trafiği
        </span>
      </div>

      <Panel
        title="Beklenen Bant"
        right={
          <div className="flex items-center gap-1 overflow-x-auto">
            {DIMS.map((d) => (
              <button
                key={d.id}
                onClick={() => setDim(d.id)}
                className={`shrink-0 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] transition ${
                  dim === d.id ? 'border-rx bg-rx/10 text-rx' : 'border-rule text-tui-dim hover:text-ink-hi'
                }`}
              >
                {d.label}
              </button>
            ))}
            <span className="mx-1 text-rule-hi">│</span>
            {METRICS.map((m) => (
              <button
                key={m.id}
                onClick={() => setMetric(m.id)}
                className={`shrink-0 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] transition ${
                  metric === m.id ? 'border-rx bg-rx/10 text-rx' : 'border-rule text-tui-dim hover:text-ink-hi'
                }`}
              >
                {m.label}
              </button>
            ))}
          </div>
        }
      >
        <AnomalyBandChart
          rows={rows}
          active={chartActive}
          seasonality={seasonality}
          metric={metric}
          currentBucket={curBucket}
        />
      </Panel>

      <Panel
        title="Aktif Sapmalar"
        right={<span className="text-[10px] text-tui-dim">{formatNum(deviations.length)} sapma · z ≥ eşik</span>}
      >
        <TuiTable
          columns={columns}
          rows={deviations}
          getKey={(d) => `${d.metric}:${d.dim}:${d.key}`}
          initialSort={{ key: 'z', dir: 'desc' }}
          filterText={(d) => `${d.scope} ${d.metric}`}
          filterLabel="kapsam / metrik"
          empty={<span className="text-tui-dim">Şu an eşik aşan sapma yok — filo normal seyrediyor.</span>}
        />
      </Panel>
    </div>
  )
}
