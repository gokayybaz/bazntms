// StatusPill — küçük durum rozeti (nokta + etiket), online/offline veya
// sağlıklı/bekliyor/sorunlu gibi 2-3 durumlu göstergeler için ortak kabuk.
// Aynı satır-içi kalıp Header/AgentDetailPage/AgentsListPage/DeviceDetailPage'de
// elle kopyalanıyordu (impeccable Faz 6, extract) — renk-anlam sözleşmesi
// (DESIGN.md) burada tek yerden korunuyor: emerald=sağlıklı, amber=uyarı/
// bekleme, rose=sorunlu, slate=çevrimdışı/nötr.
export type StatusTone = 'emerald' | 'amber' | 'rose' | 'slate'

const TONE_STYLES: Record<StatusTone, string> = {
  emerald: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400',
  amber: 'border-amber-500/30 bg-amber-500/10 text-amber-400',
  rose: 'border-rose-500/30 bg-rose-500/10 text-rose-400',
  slate: 'border-slate-600 bg-slate-800 text-slate-500',
}

const DOT_STYLES: Record<StatusTone, string> = {
  emerald: 'bg-emerald-400',
  amber: 'bg-amber-400',
  rose: 'bg-rose-400',
  slate: 'bg-slate-500',
}

export function StatusPill({ tone, label, dot = true }: { tone: StatusTone; label: string; dot?: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${TONE_STYLES[tone]}`}
    >
      {dot && <span className={`size-1.5 rounded-full ${DOT_STYLES[tone]}`} />}
      {label}
    </span>
  )
}
