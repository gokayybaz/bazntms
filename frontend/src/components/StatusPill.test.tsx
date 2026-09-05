import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StatusPill } from './StatusPill'

describe('StatusPill', () => {
  it('etiketi ve rengi doğru render eder', () => {
    render(<StatusPill tone="emerald" label="online" />)
    const pill = screen.getByText('online')
    expect(pill).toHaveClass('text-emerald-400')
  })

  it('dot={false} ile noktayı gizler', () => {
    render(<StatusPill tone="rose" label="sorunlu" dot={false} />)
    const pill = screen.getByText('sorunlu')
    expect(pill.querySelector('span')).not.toBeInTheDocument()
  })

  it('her tone için farklı bir sınıf üretir (renk-anlam sözleşmesi karışmıyor)', () => {
    const { rerender } = render(<StatusPill tone="amber" label="x" />)
    expect(screen.getByText('x')).toHaveClass('text-amber-400')
    rerender(<StatusPill tone="slate" label="x" />)
    expect(screen.getByText('x')).toHaveClass('text-slate-500')
  })
})
