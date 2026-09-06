import { render, screen } from '@testing-library/react'
import { useMemo } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { KeymapProvider, useKeymap, useRegisterKeys } from './KeymapContext'
import type { KeyAction } from './KeymapContext'

function press(key: string, init: KeyboardEventInit = {}) {
  window.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init }))
}

// kayıtlı eylemleri okuyup ekrana basan gözlemci
function ActionSpy() {
  const { actions } = useKeymap()
  return <div data-testid="keys">{actions.map((a) => `${a.key}:${a.label}`).join(',')}</div>
}

function Screen({ actions }: { actions: KeyAction[] }) {
  const memo = useMemo(() => actions, [actions])
  useRegisterKeys(memo)
  return null
}

describe('KeymapContext', () => {
  it('kayıtlı eylemi order sırasında listeler', () => {
    const acts: KeyAction[] = [
      { key: 'F5', label: 'Yenile', handler: vi.fn(), order: 100 },
      { key: 'F2', label: 'Setup', handler: vi.fn(), order: 10 },
    ]
    render(
      <KeymapProvider>
        <Screen actions={acts} />
        <ActionSpy />
      </KeymapProvider>,
    )
    expect(screen.getByTestId('keys').textContent).toBe('F2:Setup,F5:Yenile')
  })

  it('kayıtlı tuşa basınca handler tetiklenir', () => {
    const onSetup = vi.fn()
    render(
      <KeymapProvider>
        <Screen actions={[{ key: 'F2', label: 'Setup', handler: onSetup }]} />
      </KeymapProvider>,
    )
    press('F2')
    expect(onSetup).toHaveBeenCalledTimes(1)
  })

  it('unmount olan ekranın eylemleri kalkar', () => {
    const { rerender } = render(
      <KeymapProvider>
        <Screen actions={[{ key: 'F2', label: 'Setup', handler: vi.fn() }]} />
        <ActionSpy />
      </KeymapProvider>,
    )
    expect(screen.getByTestId('keys').textContent).toBe('F2:Setup')
    rerender(
      <KeymapProvider>
        <ActionSpy />
      </KeymapProvider>,
    )
    expect(screen.getByTestId('keys').textContent).toBe('')
  })

  it('aynı tuşu kaydeden son scope kazanır', () => {
    const early = vi.fn()
    const late = vi.fn()
    render(
      <KeymapProvider>
        <Screen actions={[{ key: 'F5', label: 'Genel', handler: early }]} />
        <Screen actions={[{ key: 'F5', label: 'Özel', handler: late }]} />
        <ActionSpy />
      </KeymapProvider>,
    )
    expect(screen.getByTestId('keys').textContent).toBe('F5:Özel')
    press('F5')
    expect(late).toHaveBeenCalledTimes(1)
    expect(early).not.toHaveBeenCalled()
  })

  it('useKeymap sağlayıcı dışında hata verir', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<ActionSpy />)).toThrow(/KeymapProvider/)
    spy.mockRestore()
  })
})
