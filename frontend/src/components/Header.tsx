import type { NetInterface } from '../types'

interface Props {
  interfaces: NetInterface[]
  selected: string
  onSelect: (name: string) => void
  running: boolean
  error?: string
  connected: boolean
  onStart: () => void
  onStop: () => void
  starting: boolean
  onLogout: () => void
}

export function Header({
  interfaces,
  selected,
  onSelect,
  running,
  error,
  connected,
  onStart,
  onStop,
  starting,
  onLogout,
}: Props) {
  const snifable = interfaces.filter((i) => i.up && !i.loopback)
  const options = snifable.length > 0 ? snifable : interfaces
  const activeIface = interfaces.find((i) => i.name === selected)

  return (
    <header className="sticky top-0 z-20 border-b border-slate-800 bg-slate-950/95 backdrop-blur">
      <div className="mx-auto max-w-7xl px-4 py-2.5 flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2.5 mr-2">
          <div className="grid size-9 place-items-center rounded-md border border-cyan-500/50 bg-slate-900">
            <svg viewBox="0 0 24 24" className="size-5 text-cyan-400" fill="none" stroke="currentColor" strokeWidth="1.8">
              <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
              <circle cx="5" cy="5" r="1.6" />
              <circle cx="19" cy="5" r="1.6" />
              <circle cx="5" cy="19" r="1.6" />
              <circle cx="19" cy="19" r="1.6" />
              <path d="M6.2 6.2 10.6 10.6m6.8-4.4-4.4 4.4M6.2 17.8l4.4-4.4m6.8 4.4-4.4-4.4" strokeLinecap="round" />
            </svg>
          </div>
          <div>
            <h1 className="font-mono text-[15px] font-bold leading-tight tracking-tight text-white">bazNTMS</h1>
            <p className="text-[9.5px] uppercase leading-tight tracking-[0.14em] text-slate-500">
              Network Traffic Monitoring System
            </p>
          </div>
        </div>

        <span
          className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
            connected
              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
              : 'border-amber-500/30 bg-amber-500/10 text-amber-400'
          }`}
        >
          <span className={`size-1.5 ${connected ? 'bg-emerald-400' : 'bg-amber-400'}`} />
          {connected ? 'ws: canlı' : 'ws: yoklama'}
        </span>

        {running && (
          <span className="inline-flex items-center gap-1.5 rounded border border-cyan-500/30 bg-cyan-500/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-cyan-400">
            <span className="size-1.5 animate-pulse bg-cyan-400" />
            capture: aktif
          </span>
        )}

        <div className="flex-1" />

        {error && (
          <span className="max-w-md truncate rounded border border-rose-500/30 bg-rose-500/10 px-2 py-0.5 font-mono text-[10px] text-rose-400" title={error}>
            {error}
          </span>
        )}

        <select
          value={selected}
          onChange={(e) => onSelect(e.target.value)}
          disabled={running}
          className="rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 font-mono text-sm text-slate-200 outline-none focus:border-cyan-500/60 disabled:opacity-50"
        >
          <option value="">Arayüz seçin…</option>
          {options.map((i) => (
            <option key={i.name} value={i.name}>
              {i.name}
              {i.addresses?.length ? ` — ${i.addresses[0].split('/')[0]}` : ''}
            </option>
          ))}
        </select>

        {running ? (
          <button
            onClick={onStop}
            className="rounded-md bg-rose-600 px-3.5 py-1.5 text-sm font-semibold text-white transition hover:bg-rose-500"
          >
            Durdur
          </button>
        ) : (
          <button
            onClick={onStart}
            disabled={!selected || starting}
            className="rounded-md bg-cyan-600 px-3.5 py-1.5 text-sm font-semibold text-white transition hover:bg-cyan-500 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {starting ? 'Başlatılıyor…' : 'Yakalamayı Başlat'}
          </button>
        )}

        <button
          onClick={onLogout}
          title="Oturumu kapat"
          className="inline-flex items-center gap-1.5 rounded-md border border-slate-700 px-2.5 py-1.5 text-sm text-slate-400 transition hover:border-slate-500 hover:text-slate-200"
        >
          <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M15 12H4m0 0 3.5-3.5M4 12l3.5 3.5M11 4h6a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Çıkış
        </button>
      </div>

      {/* NOC durum şeridi */}
      <div className="border-t border-slate-800/70 bg-slate-900/60">
        <div className="mx-auto max-w-7xl px-4 py-1 flex flex-wrap items-center gap-x-5 gap-y-0.5 font-mono text-[10px] uppercase tracking-wider text-slate-500">
          <span>arayüz: <span className={selected ? 'text-slate-300' : 'text-slate-600'}>{selected || 'seçilmedi'}</span></span>
          {activeIface?.addresses?.length ? (
            <span>ip: <span className="text-slate-300">{activeIface.addresses[0].split('/')[0]}</span></span>
          ) : null}
          {activeIface && <span>mtu: <span className="text-slate-300">{activeIface.mtu}</span></span>}
          {activeIface?.hw_addr && <span className="hidden md:inline">mac: <span className="text-slate-300">{activeIface.hw_addr}</span></span>}
          <span>durum: <span className={running ? 'text-cyan-400' : 'text-slate-600'}>{running ? 'yakalama' : 'beklemede'}</span></span>
          {!running && selected && (
            <span className="normal-case text-slate-600">paket yakalama için sunucuyu sudo/admin ile çalıştırın</span>
          )}
        </div>
      </div>
    </header>
  )
}
