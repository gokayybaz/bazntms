import { NavLink, useNavigate } from 'react-router-dom'
import { useHotkeys } from '../lib/useHotkeys'

// TabBar — kabuğun numaralı yatay navigasyonu (Sidebar yerine). 1-9 tuşları
// görünür sekmelere sırayla eşlenir. Aktif sekme reverse-video. Dar ekranda
// yatay kaydırır.

type Tab = { to: string; label: string; end: boolean; govern?: boolean; admin?: boolean }

const TABS: Tab[] = [
  { to: '/', label: 'Pano', end: true },
  { to: '/agentlar', label: 'Agent', end: false },
  { to: '/cihazlar', label: 'Cihaz', end: false },
  { to: '/akis', label: 'Akış', end: false },
  { to: '/cografi', label: 'Harita', end: false },
  { to: '/topoloji', label: 'Topo', end: false },
  { to: '/uyarilar', label: 'Uyarı', end: false },
  { to: '/raporlar', label: 'Rapor', end: false },
  { to: '/uyumluluk', label: 'Uyumluluk', end: false, govern: true },
  { to: '/yonetim', label: 'Yönetim', end: false, admin: true },
]

export function TabBar({ isAdmin = false, canGovern = true }: { isAdmin?: boolean; canGovern?: boolean }) {
  const navigate = useNavigate()
  const items = TABS.filter((t) => (!t.govern || canGovern) && (!t.admin || isAdmin))

  useHotkeys(
    items.slice(0, 9).map((t, i) => ({
      key: String(i + 1),
      handler: () => navigate(t.to),
    })),
  )

  return (
    <nav className="border-b border-rule bg-ground font-mono text-[11px]">
      <div className="mx-auto flex w-full max-w-[1600px] overflow-x-auto">
        {items.map((t, i) => (
          <NavLink
            key={t.to}
            to={t.to}
            end={t.end}
            className={({ isActive }) =>
              `flex shrink-0 items-center gap-1.5 border-r border-rule px-2.5 py-1.5 uppercase tracking-[0.04em] transition first:pl-4 ${
                isActive ? 'bg-rx text-ground' : 'text-tui-dim hover:bg-panel-2 hover:text-ink-hi'
              }`
            }
          >
            {({ isActive }) => (
              <>
                {i < 9 && <span className={isActive ? 'font-bold' : 'text-rx'}>{i + 1}</span>}
                {t.label}
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  )
}
