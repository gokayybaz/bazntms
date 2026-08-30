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

const IconServer = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="7" rx="1.5" />
    <rect x="2" y="13" width="20" height="7" rx="1.5" />
    <path d="M6 7.5h.01M6 16.5h.01" />
  </svg>
)

const IconRouter = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="10" width="20" height="8" rx="1.5" />
    <path d="M7 10V6a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v4" />
    <path d="M6 14h.01M10 14h.01" />
  </svg>
)

const IconTopo = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="5" cy="5" r="2" />
    <circle cx="19" cy="5" r="2" />
    <circle cx="12" cy="12" r="2.3" />
    <circle cx="5" cy="19" r="2" />
    <circle cx="19" cy="19" r="2" />
    <path d="M6.5 6.5 10.2 10.2M17.5 6.5 13.8 10.2M6.5 17.5 10.2 13.8M17.5 17.5 13.8 13.8" />
  </svg>
)

const IconBell = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M6 8a6 6 0 1 1 12 0c0 5 2 6 2 6H4s2-1 2-6Z" />
    <path d="M10 20a2 2 0 0 0 4 0" />
  </svg>
)

const IconDoc = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M6 2h9l5 5v15a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1Z" />
    <path d="M14 2v5h5M8 12h8M8 16h8M8 8h3" />
  </svg>
)

const IconShield = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2l8 3.5v5.2c0 5-3.4 9.3-8 11.3-4.6-2-8-6.3-8-11.3V5.5L12 2z" />
    <path d="M9 12l2 2 4-4.5" />
  </svg>
)

const NAV_ITEMS = [
  { to: '/', label: 'Dashboard', icon: <IconGrid />, end: true },
  { to: '/agentlar', label: "Agent'lar", icon: <IconServer />, end: false },
  { to: '/cihazlar', label: 'Cihazlar', icon: <IconRouter />, end: false },
  { to: '/topoloji', label: 'Ağ Topolojisi', icon: <IconTopo />, end: false },
  { to: '/uyarilar', label: 'Uyarılar', icon: <IconBell />, end: false },
  { to: '/raporlar', label: 'Raporlar', icon: <IconDoc />, end: false },
  { to: '/uyumluluk', label: 'Uyumluluk', icon: <IconShield />, end: false },
]

export function Sidebar() {
  return (
    <aside className="sticky top-0 flex h-screen w-56 flex-shrink-0 flex-col border-r border-slate-800 bg-slate-950/60">
      <div className="flex items-center gap-3 border-b border-slate-800 px-4 py-4">
        <div className="grid size-10 flex-shrink-0 place-items-center rounded-md border border-cyan-500/50 bg-slate-900">
          <svg viewBox="0 0 24 24" className="size-6 text-cyan-400" fill="none" stroke="currentColor" strokeWidth="1.8">
            <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
            <circle cx="5" cy="5" r="1.6" />
            <circle cx="19" cy="5" r="1.6" />
            <circle cx="5" cy="19" r="1.6" />
            <circle cx="19" cy="19" r="1.6" />
            <path d="M6.2 6.2 10.6 10.6m6.8-4.4-4.4 4.4M6.2 17.8l4.4-4.4m6.8 4.4-4.4-4.4" strokeLinecap="round" />
          </svg>
        </div>
        <div className="min-w-0">
          <p className="font-mono text-base font-bold leading-tight tracking-tight text-white">bazNTMS</p>
          <p className="text-[8.5px] uppercase leading-[1.35] tracking-[0.1em] text-slate-500">
            Network Traffic
            <br />
            Monitoring System
          </p>
        </div>
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
    </aside>
  )
}
