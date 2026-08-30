import { NavLink } from 'react-router-dom'

/* --- ikonlar (24px, stroke tabanlı — Header logosuyla aynı dil) --- */

const IconGrid = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="8" height="8" rx="1.5" />
    <rect x="13" y="3" width="8" height="8" rx="1.5" />
    <rect x="3" y="13" width="8" height="8" rx="1.5" />
    <rect x="13" y="13" width="8" height="8" rx="1.5" />
  </svg>
)

const IconLayers = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 3 2 8l10 5 10-5-10-5Z" />
    <path d="m2 13 10 5 10-5" />
  </svg>
)

const IconServer = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="7" rx="1.5" />
    <rect x="2" y="13" width="20" height="7" rx="1.5" />
    <path d="M6 7.5h.01M6 16.5h.01" />
  </svg>
)

const NAV_ITEMS = [
  { to: '/', label: 'Dashboard', icon: <IconGrid />, end: true },
  { to: '/agentlar', label: "Agent'lar", icon: <IconServer />, end: false },
  { to: '/tum-kartlar', label: 'Tüm Kartlar', icon: <IconLayers />, end: false },
]

export function Sidebar() {
  return (
    <aside className="flex w-56 flex-shrink-0 flex-col border-r border-slate-800 bg-slate-950/60">
      <div className="flex h-[57px] items-center gap-2.5 border-b border-slate-800 px-4">
        <div className="grid size-8 flex-shrink-0 place-items-center rounded-md border border-cyan-500/50 bg-slate-900">
          <svg viewBox="0 0 24 24" className="size-4.5 text-cyan-400" fill="none" stroke="currentColor" strokeWidth="1.8">
            <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
            <circle cx="5" cy="5" r="1.6" />
            <circle cx="19" cy="5" r="1.6" />
            <circle cx="5" cy="19" r="1.6" />
            <circle cx="19" cy="19" r="1.6" />
            <path d="M6.2 6.2 10.6 10.6m6.8-4.4-4.4 4.4M6.2 17.8l4.4-4.4m6.8 4.4-4.4-4.4" strokeLinecap="round" />
          </svg>
        </div>
        <span className="font-mono text-sm font-bold tracking-tight text-white">bazNTMS</span>
      </div>

      <nav className="flex-1 space-y-0.5 px-2.5 py-3">
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={({ isActive }) =>
              `flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition ${
                isActive
                  ? 'bg-cyan-500/10 text-cyan-300 border border-cyan-500/20'
                  : 'border border-transparent text-slate-400 hover:bg-slate-900 hover:text-slate-200'
              }`
            }
          >
            <span className="size-4.5 flex-shrink-0">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-slate-800 px-4 py-3">
        <p className="text-[10px] leading-relaxed text-slate-600">
          Sayfalar aşamalı olarak ayrılıyor — şimdilik detaylı kartların çoğu
          "Tüm Kartlar" altında.
        </p>
      </div>
    </aside>
  )
}
