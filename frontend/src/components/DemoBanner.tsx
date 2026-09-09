// Demo build'i (VITE_DEMO) — kabuğun en üstünde kalıcı şerit. Kullanıcı
// verinin sentetik olduğunu ve mutasyonların kaydedilmediğini bilmeli.
// Yalnız App.tsx içinde, import.meta.env.VITE_DEMO açıkken render edilir.

const REPO = 'https://github.com/gokayybaz/bazntms'

export function DemoBanner() {
  return (
    <div className="flex items-center gap-2 overflow-x-auto border-b border-rule bg-rx/10 px-4 py-1 font-mono text-[10px] whitespace-nowrap text-rx">
      <span className="shrink-0 bg-rx px-1 font-bold uppercase tracking-[0.08em] text-ground">demo</span>
      <span className="text-ink">sentetik veri · gerçek bir kurulum değil · değişiklikler kaydedilmez</span>
      <a
        href={`${REPO}#kurulum`}
        target="_blank"
        rel="noreferrer"
        className="ml-auto shrink-0 border border-rx/40 px-1.5 py-0.5 uppercase tracking-[0.04em] transition hover:bg-rx/15"
      >
        gerçek kurulum ↗
      </a>
    </div>
  )
}
