import { useState } from 'react'

// CopyButton, verilen metni panoya kopyalar ve kısa süre "kopyalandı" gösterir.
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
      className={`rounded-md border border-cyan-500/40 bg-cyan-500/10 px-2.5 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20 ${className}`}
    >
      {done ? '✓ kopyalandı' : label}
    </button>
  )
}
