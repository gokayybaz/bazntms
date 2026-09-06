import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useHotkeys } from './useHotkeys'
import type { HotkeyBinding } from './useHotkeys'

// KeymapContext — aktif ekranın F-tuşu eylemlerini toplayıp hem global klavye
// dinleyicisine hem de alt `FnKeyBar` şeridine besler. Ekranlar `useRegisterKeys`
// ile kendi eylemlerini kaydeder; unmount'ta otomatik silinir. Aynı tuşu birden
// fazla scope kaydederse SON kaydeden kazanır (daha derin/özgül ekran genel
// varsayılanı ezer).

export interface KeyAction {
  /** "F1".."F10", "?" — normalizeKey ile aynı biçim (bkz. useHotkeys). */
  key: string
  /** FnKeyBar'da gösterilen kısa etiket. */
  label: string
  handler: () => void
  /** Küçük sayı solda. Global varsayılanlar 100+, ekran eylemleri <100. */
  order?: number
  /** <input> odaktayken de çalışsın mı (varsayılan false). */
  allowInField?: boolean
}

interface KeymapValue {
  /** Görüntü için sıralı + tuş-bazında tekilleştirilmiş eylem listesi. */
  actions: KeyAction[]
  register: (scope: symbol, actions: KeyAction[]) => void
  unregister: (scope: symbol) => void
}

const KeymapCtx = createContext<KeymapValue | null>(null)

export function KeymapProvider({ children }: { children: ReactNode }) {
  // scope → o scope'un eylemleri. Kayıt sırası Map'te korunur (son = en özgül).
  const registryRef = useRef<Map<symbol, KeyAction[]>>(new Map())
  const [version, setVersion] = useState(0)
  const bump = useCallback(() => setVersion((v) => v + 1), [])

  const register = useCallback(
    (scope: symbol, actions: KeyAction[]) => {
      registryRef.current.set(scope, actions)
      bump()
    },
    [bump],
  )
  const unregister = useCallback(
    (scope: symbol) => {
      if (registryRef.current.delete(scope)) bump()
    },
    [bump],
  )

  const actions = useMemo<KeyAction[]>(() => {
    // Son kaydeden kazanır: ters sırada gez, ilk görülen tuşu al.
    const seen = new Map<string, KeyAction>()
    const scopes = [...registryRef.current.values()].reverse()
    for (const list of scopes) {
      for (const a of list) {
        if (!seen.has(a.key)) seen.set(a.key, a)
      }
    }
    return [...seen.values()].sort((x, y) => (x.order ?? 50) - (y.order ?? 50))
    // version değişince yeniden hesapla
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version])

  // Global klavye: kayıtlı her eylemin tuşunu handler'ına bağla.
  const bindings = useMemo<HotkeyBinding[]>(
    () =>
      actions.map((a) => ({
        key: a.key,
        handler: () => a.handler(),
        allowInField: a.allowInField,
      })),
    [actions],
  )
  useHotkeys(bindings)

  const value = useMemo<KeymapValue>(() => ({ actions, register, unregister }), [actions, register, unregister])

  return <KeymapCtx.Provider value={value}>{children}</KeymapCtx.Provider>
}

export function useKeymap(): KeymapValue {
  const ctx = useContext(KeymapCtx)
  if (!ctx) throw new Error('useKeymap must be used within <KeymapProvider>')
  return ctx
}

// useRegisterKeys — bir ekran/bileşen kendi F-tuşu eylemlerini kaydeder.
// `actions` dizisini useMemo ile sabitleyin (handler'lar taze state'e ihtiyaç
// duyuyorsa useMemo bağımlılıklarına ekleyin) — dizi kimliği değişince yeniden
// kaydedilir.
export function useRegisterKeys(actions: KeyAction[]): void {
  const { register, unregister } = useKeymap()
  const scopeRef = useRef<symbol | null>(null)
  if (scopeRef.current === null) scopeRef.current = Symbol('keymap-scope')

  useEffect(() => {
    const scope = scopeRef.current
    if (!scope) return
    register(scope, actions)
    return () => unregister(scope)
  }, [register, unregister, actions])
}
