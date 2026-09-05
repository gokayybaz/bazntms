import { useEffect, type ReactNode } from 'react'

// Modal, hafif merkezi diyalog: Escape ve arka-plan tıklaması kapatır.
// Harici kütüphane yok — sabit konumlu overlay + panel.
export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-slate-950/70 p-4 backdrop-blur-sm"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="w-full max-w-md rounded-lg border border-slate-700 bg-slate-900 shadow-2xl shadow-black/50"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header className="flex items-center justify-between border-b border-slate-800 px-4 py-2.5">
          <h2 className="text-[11px] font-semibold uppercase tracking-widest text-slate-400">{title}</h2>
          <button
            onClick={onClose}
            aria-label="Kapat"
            className="text-slate-500 transition hover:text-slate-200"
          >
            <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M18 6 6 18M6 6l12 12" strokeLinecap="round" />
            </svg>
          </button>
        </header>
        <div className="p-4">{children}</div>
      </div>
    </div>
  )
}
