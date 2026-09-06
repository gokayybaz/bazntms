// StatusPill — küçük durum rozeti (glyph + etiket). TUI: kare, dolgu yok,
// ● (aktif) / ○ (çevrimdışı) glyph + kenarlık + anlam-rengi metin.
// Renk-anlam sözleşmesi (DESIGN.md): emerald=sağlıklı, amber=uyarı/bekleme,
// rose=sorunlu, cyan=bilgi/nötr-aktif, slate=çevrimdışı/nötr.
export type StatusTone = 'emerald' | 'amber' | 'rose' | 'cyan' | 'slate'

const TONE_STYLES: Record<StatusTone, string> = {
  emerald: 'border-emerald-500/40 text-emerald-400',
  amber: 'border-amber-500/40 text-amber-400',
  rose: 'border-rose-500/40 text-rose-400',
  cyan: 'border-rx/40 text-rx',
  slate: 'border-rule-hi text-tui-dim',
}

// reverse-video (seçili/vurgulu satır) — tam sınıf adları (Tailwind JIT görebilsin)
const TONE_REVERSE: Record<StatusTone, string> = {
  emerald: 'border-emerald-400 bg-emerald-400 text-ground',
  amber: 'border-amber-400 bg-amber-400 text-ground',
  rose: 'border-rose-400 bg-rose-400 text-ground',
  cyan: 'border-rx bg-rx text-ground',
  slate: 'border-rule-hi bg-rule-hi text-ground',
}

export function StatusPill({
  tone,
  label,
  dot = true,
  reverse = false,
}: {
  tone: StatusTone
  label: string
  dot?: boolean
  /** Seçili/vurgulu bağlamda reverse-video (zemin = ton rengi). */
  reverse?: boolean
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 border px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-[0.06em] ${
        reverse ? TONE_REVERSE[tone] : TONE_STYLES[tone]
      }`}
    >
      {dot && <span aria-hidden>{tone === 'slate' ? '○' : '●'}</span>}
      {label}
    </span>
  )
}
