import { StatusPill } from './StatusPill'

interface Props {
  connected: boolean
  onLogout: () => void
  identity: { username: string; role: string } | null
}

export function Header({ connected, onLogout, identity }: Props) {
  return (
    <header className="sticky top-0 z-20 border-b border-slate-800 bg-slate-950/95 backdrop-blur">
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
        <StatusPill tone={connected ? 'emerald' : 'amber'} label={connected ? 'ws: canlı' : 'ws: yoklama'} />

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
