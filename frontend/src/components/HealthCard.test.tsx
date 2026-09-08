import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { HealthCard } from './HealthCard'

function mock(score: number, deductions: { reason: string; points: number }[]) {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve({ ok: true, json: async () => ({ score, deductions }) } as Response)),
  )
}

describe('HealthCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('skoru ve açıklanabilir kesintileri gösterir', async () => {
    mock(72, [
      { reason: '3 açık kritik uyarı', points: 15 },
      { reason: '2 açık olay', points: 13 },
    ])
    render(<HealthCard />)
    await waitFor(() => expect(screen.getByText('72')).toBeInTheDocument())
    expect(screen.getByText('3 açık kritik uyarı')).toBeInTheDocument()
    expect(screen.getByText('−15')).toBeInTheDocument()
  })

  it('sorun yoksa olumlu mesaj', async () => {
    mock(100, [])
    render(<HealthCard />)
    await waitFor(() => expect(screen.getByText('Tespit edilen sorun yok.')).toBeInTheDocument())
  })
})
