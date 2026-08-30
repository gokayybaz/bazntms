interface Props {
  connected: boolean
  onLogout: () => void
  identity: { username: string; role: string } | null
}

export function Header({ connected, onLogout, identity }: Props) {
  return (
    <header className="sticky top-0 z-20 border-b border-slate-800 bg-slate-950/95 backdrop-blur">
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
        <div className="flex items-center gap-2.5">
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

        <div className="flex-1" />

        {identity && (
          <span
            title={identity.username}
            className="rounded border border-violet-500/30 bg-violet-500/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-violet-300"
          >
            {identity.username} · {identity.role}
          </span>
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
    </header>
  )
}
