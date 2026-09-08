import { useEffect, useState } from 'react'
import { SEV_NAMES, SEV_STYLES } from '../lib/syslogSeverity'
import { PanelState } from './PanelState'

interface SyslogEvent {
  id: number
  ts: number
  host: string
  severity: number
  tag: string
  message: string
}

export function SyslogCard() {
  const [events, setEvents] = useState<SyslogEvent[]>([])
  // varsayılan "info": ağ cihazları çoğunlukla notice/info seviyesinde loglar
  const [minSev, setMinSev] = useState(6)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/syslog?limit=200')
        if (res.status === 401) return
        const data: SyslogEvent[] = await res.json()
        if (!stop) {
          setEvents(data)
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const id = window.setInterval(load, 5_000)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [])

  const shown = events.filter((e) => e.severity <= minSev)
  if (!loaded) return <PanelState kind="loading" />

  return (
    <div>
      <div className="mb-2 flex items-center gap-2 font-mono text-[11px]">
        <label className="flex items-center gap-2 text-tui-dim">
          en az seviye:
          <input
            type="range"
            min={0}
            max={7}
            value={minSev}
            onChange={(e) => setMinSev(+e.target.value)}
            className="w-32 accent-cyan-500"
          />
          <span className="text-ink-hi">{SEV_NAMES[minSev]}</span>
        </label>
        <span className="ml-auto text-tui-dim">
          {shown.length}/{events.length} olay
        </span>
      </div>

      {shown.length === 0 ? (
        <PanelState
          kind="empty"
          message="Olay yok."
          hint={
            <>
              cihazları syslog'u hub'ın <code className="text-tui-dim">-syslog-port</code> adresine gönderecek şekilde
              ayarlayın
            </>
          }
        />
      ) : (
        <div role="log" aria-live="polite" className="max-h-72 overflow-y-auto font-mono text-[11px]">
          {shown.map((e, i) => (
            <div key={e.id} className={`flex items-baseline gap-2 px-2 py-0.5 ${i % 2 ? 'bg-panel-2/40' : ''}`}>
              <span className="shrink-0 text-tui-dim">{new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}</span>
              <span className="shrink-0 text-tui-dim">{e.host}</span>
              <span className={`shrink-0 px-1 ${SEV_STYLES[e.severity]}`}>{SEV_NAMES[e.severity]}</span>
              <span className="min-w-0 flex-1 truncate text-ink">
                {e.tag && <span className="text-tui-dim">{e.tag}: </span>}
                {e.message}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
