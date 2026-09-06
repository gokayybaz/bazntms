import { useEffect, useRef, useState } from 'react'
import type { AlertEvent } from '../types'
import type { FleetSummary } from '../lib/useLive'
import { formatBits, formatNum } from '../lib/format'
import { Meter } from './Meter'
import { StatusPill } from './StatusPill'

// TuiHeader — kabuğun üst şeridi: marka + WS durumu + filo Meter bandı
// (RX/TX/PPS) + olay/uyarı sayaçları + kimlik + canlı saat. htop'un üst
// meter panelinin dashboard karşılığı. useLive fleet verisini App'ten alır.

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

  return (
    <header className="sticky top-0 z-20 border-b border-rule bg-ground">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-1.5 font-mono text-[11px]">
        <span className="font-bold tracking-[0.06em] text-ink-hi">bazNTMS</span>
        <StatusPill tone={connected ? 'emerald' : 'amber'} label={connected ? 'ws canlı' : 'ws yoklama'} />

        <Meter
          className="w-40"
          label="RX"
          value={rx}
          max={peaks.rx}
          accent="rx"
          display={formatBits(rx)}
          width={10}
        />
        <Meter
          className="w-40"
          label="TX"
          value={tx}
          max={peaks.tx}
          accent="tx"
          display={formatBits(tx)}
          width={10}
        />
        <Meter
          className="w-36"
          label="PPS"
          value={pps}
          max={peaks.pps}
          display={`${formatNum(Math.round(pps))}`}
          width={8}
        />

        <span className="text-tui-dim">
          EVT <span className="font-semibold text-ink">{evtRate.toFixed(1)}/s</span>
        </span>
        <span className="text-tui-dim">
          ALRT{' '}
          <span className={`font-semibold ${alerts > 0 ? 'text-rose-400' : 'text-ink'}`}>{formatNum(alerts)}</span>
        </span>

        <span className="ml-auto text-tui-dim">
          <span className="text-ink">{online}</span>/{total} agent
        </span>

        {identity && (
          <span className="border border-rule-hi px-1.5 py-0.5 text-tui-dim">
            {identity.username} · {identity.role}
            {identity.site ? ` · ${identity.site}` : ''}
          </span>
        )}

        <span className="tabular-nums text-ink">
          {now.toLocaleTimeString('tr-TR')}
          <span className="tui-cursor ml-0.5 !h-[0.9em] align-baseline" />
        </span>

        <button
          onClick={onLogout}
          title="Oturumu kapat (F10)"
          className="border border-rule-hi px-1.5 py-0.5 text-tui-dim uppercase tracking-[0.04em] transition hover:border-rose-400 hover:text-rose-400"
        >
          [F10] çıkış
        </button>
      </div>
    </header>
  )
}
