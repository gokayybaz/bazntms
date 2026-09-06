import { useEffect, useRef } from 'react'

// useHotkeys — tek global `keydown` dinleyicisi üzerinden klavye kısayolu
// bağlar. TUI kabuğunun (TabBar, FnKeyBar, HelpOverlay) ve klavye-öncelikli
// listelerin temel yapı taşı. Callback'ler bir ref'te tutulur; hook her
// render'da yeniden abone OLMAZ, o yüzden `bindings` dizisini memo'lamak
// gerekmez — ama handler kapanış değerleri her zaman tazedir.

export type HotkeyHandler = (e: KeyboardEvent) => void

export interface HotkeyBinding {
  /**
   * Normalize edilmiş tuş. Örnekler:
   *   "1", "j", "/", "?", "Enter", "Escape", "ArrowDown"
   *   "F5", "shift+F6", "ctrl+k"
   * Harf tuşlarında Shift ayrı yazılmaz (zaten büyük harf gelir: "S").
   */
  key: string
  handler: HotkeyHandler
  /**
   * <input>/<textarea>/<select>/[contenteditable] odaktayken de tetiklensin mi.
   * Varsayılan false — tek-harf kısayolları yazarken tetiklenmesin diye.
   * "Escape" gibi alandan çıkış tuşlarında true verilir.
   */
  allowInField?: boolean
  /** Eşleşince e.preventDefault() çağrılsın mı (varsayılan true). */
  preventDefault?: boolean
}

export function isEditableTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

export function normalizeKey(e: KeyboardEvent): string {
  const parts: string[] = []
  if (e.ctrlKey) parts.push('ctrl')
  if (e.altKey) parts.push('alt')
  if (e.metaKey) parts.push('meta')
  // Shift yalnızca yazdırılamayan tuşlarda anlamlı (harf zaten büyük harf verir).
  if (e.shiftKey && e.key.length > 1 && e.key !== 'Shift') parts.push('shift')
  parts.push(e.key)
  return parts.join('+')
}

export function useHotkeys(bindings: HotkeyBinding[]): void {
  const ref = useRef(bindings)
  // Her render sonrası taze bindings'i sakla — dinleyici tek sefer abone olur,
  // handler kapanış değerleri yine güncel kalır.
  useEffect(() => {
    ref.current = bindings
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const inField = isEditableTarget(e.target ?? document.activeElement)
      const norm = normalizeKey(e)
      for (const b of ref.current) {
        if (b.key !== norm) continue
        if (inField && !b.allowInField) continue
        if (b.preventDefault !== false) e.preventDefault()
        b.handler(e)
        return
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
