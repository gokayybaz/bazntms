// Sparkline — satır-içi mini trend. Terminal sparkline konvansiyonu: 8 seviyeli
// blok karakter rampası (▁▂▃▄▅▆▇█), veri noktası başına bir karakter. Harici
// kütüphane yok. Liste satırlarında (agent en-yoğun arayüz, cihaz sayacı) kullanılır.

const RAMP = ['▁', '▂', '▃', '▄', '▅', '▆', '▇', '█']

export interface SparklineProps {
  data: number[]
  /** Gösterilecek son N nokta (varsayılan 24). */
  points?: number
  /** Renk sınıfı (varsayılan text-rx). */
  className?: string
  /** Ekran okuyucu etiketi (yoksa gizlenir). */
  label?: string
}

export function toSparkChars(data: number[]): string {
  if (data.length === 0) return ''
  const min = Math.min(...data)
  const max = Math.max(...data)
  const span = max - min
  return data
    .map((v) => {
      if (!Number.isFinite(v)) return RAMP[0]
      if (span === 0) return RAMP[max > 0 ? 4 : 0]
      const lvl = Math.round(((v - min) / span) * (RAMP.length - 1))
      return RAMP[Math.max(0, Math.min(RAMP.length - 1, lvl))]
    })
    .join('')
}

export function Sparkline({ data, points = 24, className = 'text-rx', label }: SparklineProps) {
  const slice = data.slice(-points)
  const chars = toSparkChars(slice)
  return (
    <span
      className={`font-mono text-[11px] leading-none tracking-tight ${className}`}
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      {chars || '·'}
    </span>
  )
}
