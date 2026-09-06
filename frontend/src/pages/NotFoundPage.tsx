import { Link, useLocation } from 'react-router-dom'

export function NotFoundPage() {
  const { pathname } = useLocation()
  return (
    <div className="mx-auto max-w-[900px] px-4 py-16 font-mono text-[13px] leading-relaxed">
      <p className="text-tui-dim">
        <span className="text-rx">bazntms</span>:<span className="text-tx">~</span>$ open <span className="text-ink-hi">{pathname}</span>
      </p>
      <p className="mt-1 text-rose-400">bazntms: {pathname}: böyle bir sayfa yok (404)</p>
      <p className="mt-4 text-tui-dim">
        Geçerli rotalar için <Link to="/" className="text-rx hover:underline">1:Pano</Link>'ya dönün ya da{' '}
        <span className="border border-rule-hi bg-panel-2 px-1 text-ink-hi">F1</span> ile klavye yardımını açın.
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
