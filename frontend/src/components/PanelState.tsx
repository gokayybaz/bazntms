import type { ReactNode } from 'react'

// PanelState — Panel/kart içi tekil durum bandı: yükleniyor / boş / hata.
// Faz 25-E: elle yazılmış ~70 "py-6 text-center text-tui-dim Yükleniyor…"
// tekrarını tek bir tutarlı bileşende toplar. TUI: ortalanmış, sönük, tek
// satır (+ opsiyonel ipucu ve "Yeniden dene").
//
//   {!loaded && <PanelState kind="loading" />}
//   {loaded && rows.length === 0 && <PanelState kind="empty" message="Cihaz yok." hint="SNMP poller ekleyin." />}
//   {error && <PanelState kind="error" message={error} onRetry={load} />}

const DEFAULTS: Record<Kind, string> = {
  loading: 'Yükleniyor…',
  empty: 'Kayıt yok.',
  error: 'Veri alınamadı.',
}

type Kind = 'loading' | 'empty' | 'error'

export function PanelState({
  kind,
  message,
  hint,
  onRetry,
  className = '',
}: {
  kind: Kind
  /** Ana satır. Verilmezse türe göre varsayılan metin. */
  message?: ReactNode
  /** İkinci satır — ne yapılacağına dair kısa ipucu. */
  hint?: ReactNode
  /** kind="error" ile birlikte "Yeniden dene" düğmesi. */
  onRetry?: () => void
  className?: string
}) {
  const tone = kind === 'error' ? 'text-rose-400' : 'text-tui-dim'
  return (
    <div
      role={kind === 'error' ? 'alert' : 'status'}
      className={`flex flex-col items-center justify-center gap-1 px-3 py-8 text-center font-mono text-[11px] ${className}`}
    >
      <p className={tone}>
        {kind === 'loading' && (
          <span aria-hidden className="mr-1.5 inline-block animate-pulse motion-reduce:animate-none">
            ▪
          </span>
        )}
        {kind === 'error' && <span aria-hidden className="mr-1.5">⚠</span>}
        {message ?? DEFAULTS[kind]}
      </p>
      {hint && <p className="text-[10px] text-tui-dim">{hint}</p>}
      {kind === 'error' && onRetry && (
        <button
          onClick={onRetry}
          className="mt-1 border border-rule-hi px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] text-tui-dim transition hover:border-ink-hi hover:text-ink-hi"
        >
          Yeniden dene
        </button>
      )}
    </div>
  )
}
