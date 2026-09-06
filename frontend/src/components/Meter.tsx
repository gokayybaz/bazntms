import type { ReactNode } from 'react'

// Meter — imza bileşen. htop CPU/Mem çubuğunun dili: köşeli parantezli bar,
// dolu segmentler eşik rengiyle (emerald→amber→rose), boş `·`, sağ-hizalı değer.
// accent="rx"/"tx" throughput ölçerleri için (yük değil → tek renk cyan/violet).

export type MeterAccent = 'threshold' | 'rx' | 'tx'

export interface MeterProps {
  label: string
  value: number
  max: number
  /** Sağda gösterilen biçimlenmiş değer metni (yoksa yuvarlanmış value). */
  display?: string
  /** Çubuk hücre sayısı (varsayılan 20). */
  width?: number
  /** Uyarı/kritik eşikleri, 0..1 kesir (varsayılan 0.6 / 0.85). */
  warn?: number
  crit?: number
  accent?: MeterAccent
  right?: ReactNode
  className?: string
}

const THRESH_CLS = (frac: number, warn: number, crit: number) =>
  frac < warn ? 'text-emerald-400' : frac < crit ? 'text-amber-400' : 'text-rose-400'

export function Meter({
  label,
  value,
  max,
  display,
  width = 20,
  warn = 0.6,
  crit = 0.85,
  accent = 'threshold',
  right,
  className = '',
}: MeterProps) {
  const frac = max > 0 && Number.isFinite(value) ? Math.min(1, Math.max(0, value / max)) : 0
  const filled = Math.round(frac * width)
  const empty = width - filled
  const text = display ?? String(Math.round(Number.isFinite(value) ? value : 0))

  let bar: ReactNode
  let valueCls: string
  if (accent === 'rx' || accent === 'tx') {
    const cls = accent === 'rx' ? 'text-rx' : 'text-tx'
    bar = <span className={cls}>{'█'.repeat(filled)}</span>
    valueCls = cls
  } else {
    const warnAt = Math.round(warn * width)
    const critAt = Math.round(crit * width)
    const nOf = (from: number, to: number) => Math.max(0, Math.min(filled, to) - Math.max(0, from))
    bar = (
      <>
        <span className="text-emerald-400">{'█'.repeat(nOf(0, warnAt))}</span>
        <span className="text-amber-400">{'█'.repeat(nOf(warnAt, critAt))}</span>
        <span className="text-rose-400">{'█'.repeat(nOf(critAt, width))}</span>
      </>
    )
    valueCls = THRESH_CLS(frac, warn, crit)
  }

  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuenow={Math.round(Number.isFinite(value) ? value : 0)}
      aria-valuemin={0}
      aria-valuemax={Math.round(Number.isFinite(max) ? max : 0)}
      aria-valuetext={`${text} / ${label}`}
      className={`flex items-center gap-2 font-mono text-[11px] leading-tight ${className}`}
    >
      <span className="w-12 shrink-0 truncate uppercase tracking-[0.04em] text-tui-dim">{label}</span>
      {/* çubuk: dar ekranda boş `·` kuyruğu kırpılır, dolgu + değer korunur */}
      <span aria-hidden className="flex min-w-0 shrink whitespace-pre">
        <span className="shrink-0 text-rule-hi">[</span>
        <span className="min-w-0 overflow-hidden">
          {bar}
          <span className="text-rule">{'·'.repeat(empty)}</span>
        </span>
        <span className="shrink-0 text-rule-hi">]</span>
      </span>
      <span className={`shrink-0 font-semibold tabular-nums ${valueCls}`}>{text}</span>
      {right}
    </div>
  )
}
