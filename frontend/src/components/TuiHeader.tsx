import { useEffect, useRef, useState } from 'react'
import type { AlertEvent } from '../types'
import type { FleetSummary } from '../lib/useLive'
import { formatBits, formatNum } from '../lib/format'
import { StatusPill } from './StatusPill'

// TuiHeader — kabuğun üst şeridi: logo + marka + WS durumu + filo meter bandı
// (RX/TX/PPS) + olay/uyarı sayaçları + kimlik + canlı saat. htop'un üst
// meter panelinin dashboard karşılığı. useLive fleet verisini App'ten alır.
// Mobilde filo şeridi ayrı bir yatay-kaydırma satırına iner (üst satır sade kalır).

interface Props {
  connected: boolean
  fleet: FleetSummary | null
  alertEvents: AlertEvent[]
  identity: { username: string; role: string; site?: string } | null
  onLogout: () => void
}

function usePeaks(rx: number, tx: number, pps: number) {
  const ref = useRef({ rx: 1, tx: 1, pps: 1 })
  useEffect(() => {
    ref.current = {
      rx: Math.max(ref.current.rx, Number.isFinite(rx) ? rx : 0),
      tx: Math.max(ref.current.tx, Number.isFinite(tx) ? tx : 0),
      pps: Math.max(ref.current.pps, Number.isFinite(pps) ? pps : 0),
    }
  }, [rx, tx, pps])
  return ref.current
}

// Kompakt satır-içi meter — sabit-genişlik, taşmayan.
function HMeter({
  label,
  value,
  max,
  display,
  accent,
}: {
  label: string
  value: number
  max: number
  display: string
  accent: 'rx' | 'tx' | 'threshold'
}) {
  const w = 7
  const frac = max > 0 && Number.isFinite(value) ? Math.min(1, Math.max(0, value / max)) : 0
  const filled = Math.round(frac * w)
  const cls =
    accent === 'rx'
      ? 'text-rx'
      : accent === 'tx'
        ? 'text-tx'
        : frac < 0.6
          ? 'text-emerald-400'
          : frac < 0.85
            ? 'text-amber-400'
            : 'text-rose-400'
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      <span className="text-tui-dim">{label}</span>
      <span aria-hidden className="whitespace-pre text-rule-hi">
        [<span className={cls}>{'█'.repeat(filled)}</span>
        <span className="text-rule">{'·'.repeat(w - filled)}</span>]
      </span>
      <span className={`w-[5.5rem] shrink-0 tabular-nums ${cls}`}>{display}</span>
    </span>
  )
}

export function TuiHeader({ connected, fleet, alertEvents, identity, onLogout }: Props) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(id)
  }, [])

  const rx = fleet?.rx_bps ?? 0
  const tx = fleet?.tx_bps ?? 0
  const pps = fleet?.pps ?? 0
  const evtRate = (fleet?.flows_per_min ?? 0) / 60
  const peaks = usePeaks(rx, tx, pps)

  const alerts = alertEvents.length
  const online = fleet?.agents_online ?? 0
  const total = fleet?.agents_total ?? 0

  const stats = (
    <>
      <HMeter label="RX" value={rx} max={peaks.rx} accent="rx" display={formatBits(rx)} />
      <HMeter label="TX" value={tx} max={peaks.tx} accent="tx" display={formatBits(tx)} />
      <HMeter label="PPS" value={pps} max={peaks.pps} accent="threshold" display={`${formatNum(Math.round(pps))} pps`} />
      <span className="shrink-0 text-tui-dim">
        EVT <span className="font-semibold text-ink">{evtRate.toFixed(1)}/s</span>
      </span>
      <span className="shrink-0 text-tui-dim">
        ALRT <span className={`font-semibold ${alerts > 0 ? 'text-rose-400' : 'text-ink'}`}>{formatNum(alerts)}</span>
      </span>
      <span className="shrink-0 text-tui-dim">
        <span className="text-ink">{online}</span>/{total} agent
      </span>
    </>
  )

  return (
    <header className="border-b border-rule bg-ground">
      <div className="mx-auto w-full max-w-[1600px] px-4 py-1.5 font-mono text-[11px]">
        {/* üst satır */}
        <div className="flex items-center gap-x-3 gap-y-1">
          <span className="flex shrink-0 items-center gap-1.5">
            <svg viewBox="0 0 24 24" className="size-3.5 text-rx" fill="none" stroke="currentColor" strokeWidth="1.9">
              <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
              <circle cx="5" cy="5" r="1.6" />
              <circle cx="19" cy="5" r="1.6" />
              <circle cx="5" cy="19" r="1.6" />
              <circle cx="19" cy="19" r="1.6" />
              <path d="M6.2 6.2 10.6 10.6m6.8-4.4-4.4 4.4M6.2 17.8l4.4-4.4m6.8 4.4-4.4-4.4" strokeLinecap="round" />
            </svg>
            <span className="font-bold tracking-[0.06em] text-ink-hi">bazNTMS</span>
          </span>
          <StatusPill tone={connected ? 'emerald' : 'amber'} label={connected ? 'ws canlı' : 'ws yoklama'} />

          {/* sm+ : filo istatistikleri burada; mobilde alt şeritte. Ara
              genişliklerde taşarsa kendi içinde kayar, kabuğu bozmaz. */}
          <div className="hidden min-w-0 items-center gap-x-4 overflow-x-auto sm:flex">{stats}</div>

          <span className="ml-auto shrink-0" />

          {identity && (
            <span className="hidden shrink-0 border border-rule-hi px-1.5 py-0.5 text-tui-dim md:inline">
              {identity.username} · {identity.role}
              {identity.site ? ` · ${identity.site}` : ''}
            </span>
          )}

          <span className="shrink-0 tabular-nums text-ink">
            {now.toLocaleTimeString('tr-TR')}
            <span className="tui-cursor ml-0.5 !h-[0.9em] align-baseline" />
          </span>

          <button
            onClick={onLogout}
            title="Oturumu kapat (F10)"
            className="shrink-0 border border-rule-hi px-1.5 py-0.5 uppercase tracking-[0.04em] text-tui-dim transition hover:border-rose-400 hover:text-rose-400"
          >
            <span className="hidden sm:inline">[F10] çıkış</span>
            <span className="sm:hidden">F10</span>
          </button>
        </div>

        {/* mobil filo şeridi — yatay kaydırma */}
        <div className="mt-1 flex items-center gap-x-4 overflow-x-auto pb-0.5 sm:hidden">{stats}</div>
      </div>
    </header>
  )
}
