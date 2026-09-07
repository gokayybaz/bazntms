import { act, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { KeymapProvider } from '../lib/KeymapContext'
import { FullscreenCard } from './FullscreenCard'

function press(key: string) {
  act(() => {
    window.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }))
  })
}

function wrap(ui: React.ReactNode) {
  return render(<KeymapProvider>{ui}</KeymapProvider>)
}

describe('FullscreenCard', () => {
  it('F4 tam ekranı açıp kapatır, çocuğa full bayrağını geçirir', () => {
    wrap(
      <FullscreenCard title="Canlı Akış" hint="ipucu">
        {(full) => <div data-testid="child">{full ? 'TAM' : 'PANEL'}</div>}
      </FullscreenCard>,
    )
    expect(screen.getByTestId('child').textContent).toBe('PANEL')

    press('F4')
    expect(screen.getByTestId('child').textContent).toBe('TAM')
    expect(screen.getByText('Canlı Akış')).toBeInTheDocument()

    press('F4')
    expect(screen.getByTestId('child').textContent).toBe('PANEL')
  })

  it('tam ekranda Escape kapatır', () => {
    wrap(<FullscreenCard title="Coğrafi">{(full) => <span>{String(full)}</span>}</FullscreenCard>)
    press('F4')
    expect(screen.getByText('true')).toBeInTheDocument()
    press('Escape')
    expect(screen.getByText('false')).toBeInTheDocument()
  })

  it('editable: F3 düzenle modunu açıp kapatır', () => {
    wrap(
      <FullscreenCard title="Canlı Akış" editable>
        {(_full, editing) => <div data-testid="e">{editing ? 'DÜZEN' : 'İZLE'}</div>}
      </FullscreenCard>,
    )
    expect(screen.getByTestId('e').textContent).toBe('İZLE')
    press('F3')
    expect(screen.getByTestId('e').textContent).toBe('DÜZEN')
    press('F3')
    expect(screen.getByTestId('e').textContent).toBe('İZLE')
  })

  it('editable verilmezse F3 bir şey yapmaz', () => {
    wrap(
      <FullscreenCard title="Coğrafi">
        {(_full, editing) => <div data-testid="e">{String(editing)}</div>}
      </FullscreenCard>,
    )
    press('F3')
    expect(screen.getByTestId('e').textContent).toBe('false')
  })
})
