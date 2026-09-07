import { FullscreenCard } from '../components/FullscreenCard'
import { GeoMapCard } from '../components/GeoMapCard'

const HINT = 'NetFlow + agent uç noktaları · GeoIP ile ülke merkezine'

export function GeoPage() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Coğrafi Trafik</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">{HINT}</span>
      </div>

      <FullscreenCard title="Coğrafi Trafik" hint={HINT}>
        {(full) => <GeoMapCard fill={full} />}
      </FullscreenCard>
    </div>
  )
}
