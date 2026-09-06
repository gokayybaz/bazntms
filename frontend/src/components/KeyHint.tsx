import type { ReactNode } from 'react'

// KeyHint — metin içi tuş ipucu rozeti: <KeyHint>F5</KeyHint> → kutulu [F5].
// FnKeyBar'dan farklı; yardım metinlerinde ve ipuçlarında kullanılır.
export function KeyHint({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <kbd
      className={`inline-block border border-rule-hi bg-panel-2 px-1 font-mono text-[10px] leading-[1.4] text-ink-hi ${className}`}
    >
      {children}
    </kbd>
  )
}
