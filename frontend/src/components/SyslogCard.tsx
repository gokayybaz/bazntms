import { useEffect, useState } from 'react'
import { SEV_NAMES, SEV_STYLES } from '../lib/syslogSeverity'

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
  // varsayılan "info": ağ cihazları çoğunlukla notice/info seviyesinde loglar;
  // 0 (yalnız emergency) çoğu kurulumda kartı boş gösteriyordu
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
  if (!loaded) return <p className="py-6 text-center text-sm text-slate-600">Yükleniyor…</p>

  return (
    <div>
      <div className="mb-2 flex items-center gap-2">
        <label className="flex items-center gap-2 text-xs text-slate-500">
          en az seviye:
          <input
            type="range"
            min={0}
            max={7}
            value={minSev}
            onChange={(e) => setMinSev(+e.target.value)}
            className="w-32 accent-cyan-500"
          />
          <span className="font-mono text-slate-300">{SEV_NAMES[minSev]}</span>
        </label>
        <span className="ml-auto font-mono text-[10px] text-slate-600">{shown.length}/{events.length} olay</span>
      </div>

      {shown.length === 0 ? (
        <p className="py-6 text-center text-sm text-slate-600">
          Olay yok — cihazları syslog'u hub'ın <code className="text-slate-400">-syslog-port</code> adresine gönderecek şekilde ayarlayın.
        </p>
      ) : (
        <ul className="max-h-72 space-y-1 overflow-y-auto pr-1">
          {shown.map((e) => (
            <li key={e.id} className="flex items-baseline gap-2 rounded px-2 py-1 font-mono text-[11px] hover:bg-slate-800/30">
              <span className="text-slate-600">{new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}</span>
              <span className="text-slate-400">{e.host}</span>
              <span className={`rounded px-1 ring-1 ${SEV_STYLES[e.severity]}`}>{SEV_NAMES[e.severity]}</span>
              <span className="truncate text-slate-300">{e.tag && <span className="text-slate-500">{e.tag}: </span>}{e.message}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
