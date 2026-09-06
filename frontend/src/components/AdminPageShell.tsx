import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

const ITEMS = [
  { to: '/yonetim/kullanicilar', label: 'Kullanıcılar' },
  { to: '/yonetim/tokenlar', label: 'API Token’ları' },
  { to: '/yonetim/agent-ekle', label: 'Agent Ekle' },
  { to: '/yonetim/denetim', label: 'Denetim Kaydı' },
]

// AdminPageShell, tüm /yonetim/* sayfaları için ortak başlık + alt gezinme (TUI).
export function AdminPageShell({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Yönetim</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">{title}</span>
      </div>

      <nav className="flex flex-wrap border border-rule bg-ground text-[11px]">
        {ITEMS.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end
            className={({ isActive }) =>
              `border-r border-rule px-3 py-1 uppercase tracking-[0.04em] transition ${
                isActive ? 'bg-rx text-ground' : 'text-tui-dim hover:bg-panel-2 hover:text-ink-hi'
              }`
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>

      {hint && <p className="text-[11px] text-tui-dim">{hint}</p>}
      {children}
    </div>
  )
}

// Placeholder, henüz bağlanmamış yönetim sayfaları için geçici kabuk.
export function Placeholder({ note }: { note: string }) {
  return (
    <div className="border border-dashed border-rule-hi bg-panel-2/40 px-4 py-10 text-center">
      <p className="font-mono text-[11px] text-tui-dim">{note}</p>
    </div>
  )
}
