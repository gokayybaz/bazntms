import { Link, Outlet } from 'react-router-dom'

// AdminGuard, /yonetim/* alt rotalarını sarar. Erişim: RBAC admin rolü VEYA
// kimlik doğrulama tamamen kapalıysa (dev modu — sunucuda requirePerm da
// aynı şekilde herkesi geçirir). Aksi halde "yetkiniz yok" paneli.
export function AdminGuard({ isAdmin }: { isAdmin: boolean }) {
  if (isAdmin) return <Outlet />
  return (
    <div className="mx-auto flex max-w-7xl flex-col items-center justify-center gap-3 px-4 py-24 text-center">
      <svg viewBox="0 0 24 24" className="size-10 text-slate-700" fill="none" stroke="currentColor" strokeWidth="1.7">
        <rect x="4" y="10" width="16" height="10" rx="2" />
        <path d="M8 10V7a4 4 0 0 1 8 0v3" strokeLinecap="round" />
      </svg>
      <h1 className="text-lg font-semibold text-slate-300">Yetkiniz yok</h1>
      <p className="max-w-sm text-sm text-slate-500">
        Yönetim bölümü yalnızca <span className="text-slate-300">yönetici</span> rolündeki hesaplara açıktır.
      </p>
      <Link
        to="/"
        className="mt-2 rounded-md border border-cyan-500/40 bg-cyan-500/10 px-4 py-1.5 text-sm font-medium text-cyan-300 transition hover:bg-cyan-500/20"
      >
        Dashboard'a dön
      </Link>
    </div>
  )
}
