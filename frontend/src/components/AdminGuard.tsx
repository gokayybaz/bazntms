import { Link, Outlet } from 'react-router-dom'

// AdminGuard, /yonetim/* alt rotalarını sarar. Erişim: RBAC admin VEYA
// site-admin (kendi sahası, S14.B) rolü VEYA kimlik doğrulama tamamen kapalıysa
// (dev modu — sunucuda requirePerm da aynı şekilde davranır). Aksi halde
// "yetkiniz yok" paneli.
export function AdminGuard({ isAdmin }: { isAdmin: boolean }) {
  if (isAdmin) return <Outlet />
  return (
    <div className="mx-auto max-w-[900px] px-4 py-16 font-mono text-[13px] leading-relaxed">
      <p className="text-rose-400">bazntms: /yonetim: erişim reddedildi (403)</p>
      <p className="mt-2 text-tui-dim">
        Yönetim bölümü <span className="text-ink-hi">yönetici</span> ve{' '}
        <span className="text-ink-hi">saha yöneticisi</span> rolündeki hesaplara açıktır.
      </p>
      <p className="mt-6">
        <Link
          to="/"
          className="inline-block border border-rx/40 bg-rx/10 px-2.5 py-0.5 text-[11px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20"
        >
          cd ~ (Pano)
        </Link>
      </p>
    </div>
  )
}
