import { useState } from 'react'

// CopyButton, verilen metni panoya kopyalar ve kısa süre "kopyalandı" gösterir.
// TUI: kare, mono, küçük — cyan (bilgi/nötr eylem) rozeti.
export function CopyButton({ text, label = 'Kopyala', className = '' }: { text: string; label?: string; className?: string }) {
  const [done, setDone] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setDone(true)
      window.setTimeout(() => setDone(false), 1800)
    } catch {
      setDone(false)
    }
  }
  return (
    <button
      onClick={copy}
      className={`border border-rx/40 bg-rx/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20 ${className}`}
    >
      {done ? '✓ kopyalandı' : label}
    </button>
  )
}
