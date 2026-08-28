import type { ReactNode } from 'react'

export function Card({
  title,
  right,
  children,
  className = '',
}: {
  title?: string
  right?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section
      className={`rounded-md border border-slate-800 bg-slate-900/70 ${className}`}
    >
      {(title || right) && (
        <header className="flex items-center justify-between px-4 py-2.5 border-b border-slate-800">
          <h2 className="text-[11px] font-semibold tracking-widest text-slate-400 uppercase">{title}</h2>
          {right}
        </header>
      )}
      <div className="p-4">{children}</div>
    </section>
  )
}
