import { NavLink } from 'react-router-dom'

const ITEMS = [
  { to: '/uyumluluk', label: 'Genel Bakış', end: true },
  { to: '/uyumluluk/risk', label: 'Risk Defteri', end: true },
  { to: '/uyumluluk/soa', label: 'SoA', end: true },
  { to: '/uyumluluk/politikalar', label: 'Politikalar', end: true },
  { to: '/uyumluluk/denetimler', label: 'Denetimler', end: true },
  { to: '/uyumluluk/yonetisim', label: 'Yönetişim', end: true },
]

export function ComplianceSubNav() {
  return (
    <div className="flex flex-wrap gap-1.5 border-b border-slate-800 pb-3">
      {ITEMS.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive }) =>
            `rounded-md px-3 py-1.5 text-xs font-medium transition ${
              isActive ? 'bg-cyan-500/15 text-cyan-300' : 'text-slate-500 hover:bg-slate-900 hover:text-slate-300'
            }`
          }
        >
          {item.label}
        </NavLink>
      ))}
    </div>
  )
}
