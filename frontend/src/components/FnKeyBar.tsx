import { useKeymap } from '../lib/KeymapContext'

// FnKeyBar — kabuğun alt sabit F-tuşu şeridi (htop dili). KeymapContext'e
// kayıtlı aktif eylemleri reverse-video çizer. Tıklanabilir (fare karşılığı).
export function FnKeyBar() {
  const { actions } = useKeymap()

  return (
    <footer className="border-t border-rule bg-ground font-mono text-[10px]">
      <div className="mx-auto flex w-full max-w-[1600px] overflow-x-auto px-4">
        {actions.length === 0 ? (
          <span className="py-1 text-tui-dim">—</span>
        ) : (
          actions.map((a) => (
            <button
              key={a.key}
              onClick={a.handler}
              className="flex shrink-0 items-center gap-1 py-1 pr-3 text-tui-dim transition hover:text-ink-hi"
            >
              <span className="bg-rx px-1 font-bold text-ground">{a.key}</span>
              <span className="uppercase tracking-[0.04em]">{a.label}</span>
            </button>
          ))
        )}
      </div>
    </footer>
  )
}
