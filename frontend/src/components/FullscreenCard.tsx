import { useMemo, useState, type ReactNode } from 'react'
import { useRegisterKeys } from '../lib/KeymapContext'
import type { KeyAction } from '../lib/KeymapContext'
import { useHotkeys } from '../lib/useHotkeys'
import { Panel } from './Panel'

// FullscreenCard — canlı görselleri (Akış şeması, Coğrafi harita) kabuğun
// üstüne tam ekran açar. Çok agent / çok trafik senaryosunda dar panel içinde
// sıkışan SVG'lere nefes alanı verir. F4 (FnKeyBar'a da düşer) veya sağ üst
// düğme açıp kapatır; tam ekranda Esc kapatır.
//
// `editable` verildiğinde ikinci bir "Düzenle" düğmesi (F3) çıkar — yöneticinin
// şema üzerinde satır içi düzenleme (agent gruplama) yapmasını açar/kapatır.
//
// `children` bir render-prop: `full` ve `editing` bayraklarını alt bileşene
// geçirir ki diyagram kendi ölçeğini/etkileşimini buna göre seçebilsin.
export function FullscreenCard({
  title,
  hint,
  editable = false,
  children,
}: {
  title: string
  hint?: string
  editable?: boolean
  children: (full: boolean, editing: boolean) => ReactNode
}) {
  const [full, setFull] = useState(false)
  const [editing, setEditing] = useState(false)

  const actions = useMemo<KeyAction[]>(() => {
    const list: KeyAction[] = [
      { key: 'F4', label: full ? 'Küçült' : 'Tam Ekran', order: 20, handler: () => setFull((f) => !f) },
    ]
    if (editable) {
      list.push({ key: 'F3', label: editing ? 'Bitir' : 'Düzenle', order: 19, handler: () => setEditing((e) => !e) })
    }
    return list
  }, [full, editing, editable])
  useRegisterKeys(actions)
  // Esc yalnızca tam ekranda bağlanır; açık bir diyalog varsa ona öncelik ver.
  useHotkeys(
    full
      ? [
          {
            key: 'Escape',
            allowInField: true,
            handler: () => {
              if (!document.querySelector('[role="dialog"]')) setFull(false)
            },
          },
        ]
      : [],
  )

  const btn = (onClick: () => void, key: string, label: string, pressed: boolean) => (
    <button
      onClick={onClick}
      aria-pressed={pressed}
      className="flex shrink-0 items-center gap-1 font-mono text-[10px] text-tui-dim transition hover:text-ink-hi"
    >
      <span className={`px-1 font-bold text-ground ${pressed ? 'bg-tx' : 'bg-rx'}`}>{key}</span>
      <span className="uppercase tracking-[0.04em]">{label}</span>
    </button>
  )

  const controls = (
    <>
      {editable && btn(() => setEditing((e) => !e), 'F3', editing ? 'Bitir' : 'Düzenle', editing)}
      {btn(() => setFull((f) => !f), 'F4', full ? 'Küçült' : 'Tam ekran', full)}
    </>
  )

  if (full) {
    return (
      <div className="fixed inset-0 z-40 flex flex-col bg-ground font-mono">
        <header className="flex items-center gap-2 border-b border-rule px-4 py-1.5">
          <h2 className="flex min-w-0 items-center gap-1.5 text-[11px] font-medium uppercase tracking-[0.06em] text-ink-hi">
            <span aria-hidden className="text-rule-hi">┤</span>
            <span className="truncate">{title}</span>
            <span aria-hidden className="text-rule-hi">├</span>
          </h2>
          {hint && <span className="hidden truncate text-[10px] text-tui-dim md:inline">{hint}</span>}
          <div className="ml-auto flex items-center gap-3">
            <span className="hidden text-[10px] text-tui-dim sm:inline">Esc çıkış</span>
            {controls}
          </div>
        </header>
        <div className="min-h-0 flex-1 overflow-auto p-4">{children(true, editing)}</div>
      </div>
    )
  }

  return (
    <Panel right={<div className="flex items-center gap-3">{controls}</div>}>{children(false, editing)}</Panel>
  )
}
