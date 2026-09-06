import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { normalizeKey, useHotkeys } from './useHotkeys'

function press(key: string, opts: KeyboardEventInit & { target?: Element } = {}) {
  const { target, ...init } = opts
  const ev = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init })
  ;(target ?? window).dispatchEvent(ev)
  return ev
}

describe('normalizeKey', () => {
  it('yalın tuşu olduğu gibi verir', () => {
    expect(normalizeKey(new KeyboardEvent('keydown', { key: 'j' }))).toBe('j')
    expect(normalizeKey(new KeyboardEvent('keydown', { key: '1' }))).toBe('1')
  })
  it('modifier öneklerini ekler', () => {
    expect(normalizeKey(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))).toBe('ctrl+k')
    expect(normalizeKey(new KeyboardEvent('keydown', { key: 'F6', shiftKey: true }))).toBe('shift+F6')
  })
  it('harf tuşunda Shift öneki eklemez (büyük harf zaten gelir)', () => {
    expect(normalizeKey(new KeyboardEvent('keydown', { key: 'S', shiftKey: true }))).toBe('S')
  })
})

describe('useHotkeys', () => {
  it('eşleşen tuşta handler çağırır ve preventDefault yapar', () => {
    const handler = vi.fn()
    renderHook(() => useHotkeys([{ key: 'j', handler }]))
    const ev = press('j')
    expect(handler).toHaveBeenCalledTimes(1)
    expect(ev.defaultPrevented).toBe(true)
  })

  it('eşleşmeyen tuşta hiçbir şey yapmaz', () => {
    const handler = vi.fn()
    renderHook(() => useHotkeys([{ key: 'j', handler }]))
    press('k')
    expect(handler).not.toHaveBeenCalled()
  })

  it('input odaktayken tek-harf kısayolunu bastırır', () => {
    const handler = vi.fn()
    renderHook(() => useHotkeys([{ key: 'j', handler }]))
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    press('j', { target: input })
    expect(handler).not.toHaveBeenCalled()
    input.remove()
  })

  it('allowInField ile input odaktayken de çalışır', () => {
    const handler = vi.fn()
    renderHook(() => useHotkeys([{ key: 'Escape', handler, allowInField: true }]))
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    press('Escape', { target: input })
    expect(handler).toHaveBeenCalledTimes(1)
    input.remove()
  })

  it('preventDefault:false ile olayı iptal etmez', () => {
    renderHook(() => useHotkeys([{ key: 'g', handler: () => {}, preventDefault: false }]))
    const ev = press('g')
    expect(ev.defaultPrevented).toBe(false)
  })

  it('unmount sonrası dinlemeyi bırakır', () => {
    const handler = vi.fn()
    const { unmount } = renderHook(() => useHotkeys([{ key: 'j', handler }]))
    unmount()
    press('j')
    expect(handler).not.toHaveBeenCalled()
  })

  it('yeniden abone olmadan taze kapanış değeri kullanır', () => {
    let seen = 0
    const { rerender } = renderHook(({ n }: { n: number }) => useHotkeys([{ key: 'j', handler: () => (seen = n) }]), {
      initialProps: { n: 1 },
    })
    rerender({ n: 2 })
    press('j')
    expect(seen).toBe(2)
  })
})
