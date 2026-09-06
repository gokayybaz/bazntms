// RangeTabs — kartlarda tekrarlanan segment seçici ("15 dk | 1 saat | 6 saat",
// "7 gün | 30 gün" vb.). TUI: kare, reverse-video aktif.
export function RangeTabs<T extends string | number>({
  ranges,
  value,
  onChange,
  className = '',
}: {
  ranges: readonly { label: string; value: T }[]
  value: T
  onChange: (v: T) => void
  className?: string
}) {
  return (
    <div className={`flex border border-rule ${className}`} role="group" aria-label="Aralık seçici">
      {ranges.map((r) => (
        <button
          key={String(r.value)}
          type="button"
          onClick={() => onChange(r.value)}
          aria-pressed={value === r.value}
          className={`px-2 py-0.5 font-mono text-[11px] transition ${
            value === r.value ? 'bg-rx text-ground' : 'text-tui-dim hover:text-ink-hi'
          }`}
        >
          {r.label}
        </button>
      ))}
    </div>
  )
}
