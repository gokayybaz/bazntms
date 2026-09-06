import type { ReactNode } from 'react'

// Panel — projedeki tek paylaşılan konteyner (eski Card.tsx bunun alias'ı).
// TUI: kare köşe, 1px kenarlık, OPAK zemin, gölge YOK. Başlık şeridi box-title
// aksanı taşır (┤ BAŞLIK ├) — bkz. DESIGN.md → Components → Panel.
export function Panel({
  title,
  right,
  children,
  className = '',
  bodyClassName = 'p-4',
}: {
  title?: string
  right?: ReactNode
  children: ReactNode
  className?: string
  bodyClassName?: string
}) {
  return (
    <section className={`border border-rule bg-panel ${className}`}>
      {(title || right) && (
        <header className="flex items-center gap-2 border-b border-rule px-3 py-1.5">
          {title && (
            <h2 className="flex min-w-0 items-center gap-1.5 text-[11px] font-medium uppercase tracking-[0.06em] text-ink-hi">
              <span aria-hidden className="text-rule-hi">┤</span>
              <span className="truncate">{title}</span>
              <span aria-hidden className="text-rule-hi">├</span>
            </h2>
          )}
          {right && <div className="ml-auto flex min-w-0 items-center gap-2">{right}</div>}
        </header>
      )}
      <div className={bodyClassName}>{children}</div>
    </section>
  )
}
