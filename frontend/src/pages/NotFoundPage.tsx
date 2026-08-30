import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="mx-auto flex max-w-7xl flex-col items-center justify-center gap-3 px-4 py-24 text-center">
      <span className="font-mono text-5xl font-bold text-slate-700">404</span>
      <h1 className="text-lg font-semibold text-slate-300">Sayfa bulunamadı</h1>
      <p className="max-w-sm text-sm text-slate-500">Aradığınız adres yok ya da taşındı.</p>
      <Link
        to="/"
        className="mt-2 rounded-md border border-cyan-500/40 bg-cyan-500/10 px-4 py-1.5 text-sm font-medium text-cyan-300 transition hover:bg-cyan-500/20"
      >
        Dashboard'a dön
      </Link>
    </div>
  )
}
