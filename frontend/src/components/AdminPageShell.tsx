import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

const ITEMS = [
  { to: '/yonetim/kullanicilar', label: 'Kullanıcılar' },
  { to: '/yonetim/tokenlar', label: 'API Token’ları' },
  { to: '/yonetim/agent-ekle', label: 'Agent Ekle' },
  { to: '/yonetim/denetim', label: 'Denetim Kaydı' },
]

// AdminPageShell, tüm /yonetim/* sayfaları için ortak başlık + alt gezinme.
export function AdminPageShell({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Yönetim</h1>
        <span className="text-xs text-slate-500">{title}</span>
      </div>

      <div className="flex flex-wrap gap-1.5 border-b border-slate-800 pb-3">
        {ITEMS.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end
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

      {hint && <p className="text-xs text-slate-500">{hint}</p>}
      {children}
    </div>
  )
}

// Placeholder, henüz bağlanmamış yönetim sayfaları için geçici kabuk (Faz 12
// S12.2–S12.5 doldurur).
export function Placeholder({ note }: { note: string }) {
  return (
    <div className="rounded-lg border border-dashed border-slate-800 bg-slate-950/40 px-4 py-10 text-center">
      <p className="text-sm text-slate-500">{note}</p>
    </div>
  )
}
