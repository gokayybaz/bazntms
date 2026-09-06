import { useEffect, useRef, type ReactNode } from 'react'

// Modal — ncurses tarzı merkezi diyalog kutusu. Escape ve arka-plan tıklaması
// kapatır. Gölge/blur yok (TUI). Açılınca odak kutuya taşınır, Tab kutu içinde
// döner (focus trap), kapanınca önceki odak geri gelir.
export function Modal({
  title,
  onClose,
  children,
  width = 'max-w-md',
}: {
  title: string
  onClose: () => void
  children: ReactNode
  width?: string
}) {
  const boxRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null
    boxRef.current?.focus()
    return () => prev?.focus?.()
  }, [])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      onClose()
      return
    }
    if (e.key !== 'Tab') return
    const f = boxRef.current?.querySelectorAll<HTMLElement>(
      'a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])',
    )
    if (!f || f.length === 0) return
    const first = f[0]
    const last = f[f.length - 1]
    if (e.shiftKey && document.activeElement === first) {
      last.focus()
      e.preventDefault()
    } else if (!e.shiftKey && document.activeElement === last) {
      first.focus()
      e.preventDefault()
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4"
      onClick={onClose}
      onKeyDown={onKeyDown}
      role="presentation"
    >
      <div
        ref={boxRef}
        tabIndex={-1}
        className={`w-full ${width} border border-rule-hi bg-panel outline-none`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header className="flex items-center gap-2 border-b border-rule px-3 py-1.5">
          <h2 className="flex min-w-0 flex-1 items-center gap-1.5 font-mono text-[11px] font-medium uppercase tracking-[0.06em] text-ink-hi">
            <span aria-hidden className="text-rule-hi">┤</span>
            <span className="truncate">{title}</span>
            <span aria-hidden className="text-rule-hi">├</span>
          </h2>
          <button
            onClick={onClose}
            aria-label="Kapat"
            className="shrink-0 font-mono text-[13px] text-tui-dim transition hover:text-ink-hi"
          >
            ✕
          </button>
        </header>
        <div className="p-4">{children}</div>
      </div>
    </div>
  )
}
