import { usePolledJson } from '../lib/usePolledJson'
import { PanelState } from './PanelState'

// Faz 25-A: ağ sağlık skoru — deterministik, her kesinti açıklanabilir.

interface Deduction {
  reason: string
  points: number
}
interface Score {
  score: number
  deductions: Deduction[]
}

export function HealthCard() {
  const { data, loaded } = usePolledJson<Score>('/api/v1/health', 30_000)

  if (!loaded || !data) {
    return <PanelState kind="loading" />
  }

  const tone = data.score >= 85 ? 'text-emerald-400' : data.score >= 60 ? 'text-amber-400' : 'text-rose-400'
  const barTone = data.score >= 85 ? 'bg-emerald-500' : data.score >= 60 ? 'bg-amber-500' : 'bg-rose-500'

  return (
    <div className="font-mono">
      <div className="flex items-baseline gap-2">
        <span className={`text-2xl font-bold ${tone}`}>{data.score}</span>
        <span className="text-[11px] text-tui-dim">/ 100</span>
      </div>
      <div className="mt-1.5 h-1.5 w-full bg-panel-2">
        <span className={`block h-full ${barTone}`} style={{ width: `${Math.max(2, data.score)}%` }} />
      </div>
      {data.deductions.length === 0 ? (
        <p className="mt-3 text-[11px] text-emerald-400">Tespit edilen sorun yok.</p>
      ) : (
        <ul className="mt-3 space-y-1 text-[11px]">
          {data.deductions.map((d, i) => (
            <li key={`${d.reason}-${i}`} className="flex justify-between gap-2">
              <span className="text-ink">{d.reason}</span>
              <span className="shrink-0 text-rose-400">−{d.points}</span>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-3 text-[10px] text-tui-dim">deterministik ağırlıklı — opak AI skoru değil</p>
    </div>
  )
}
