import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Meter } from './Meter'

describe('Meter', () => {
  it('düşük yükte değer emerald, yüksekte rose', () => {
    const { rerender } = render(<Meter label="CPU" value={20} max={100} display="20%" />)
    expect(screen.getByText('20%')).toHaveClass('text-emerald-400')
    rerender(<Meter label="CPU" value={70} max={100} display="70%" />)
    expect(screen.getByText('70%')).toHaveClass('text-amber-400')
    rerender(<Meter label="CPU" value={95} max={100} display="95%" />)
    expect(screen.getByText('95%')).toHaveClass('text-rose-400')
  })

  it('value > max iken çubuk tamamen dolar (clamp)', () => {
    render(<Meter label="X" value={500} max={100} width={10} display="500" />)
    const meter = screen.getByRole('meter')
    // 10 hücre dolu → 10 █, 0 nokta
    expect(meter.textContent).toContain('█'.repeat(10))
    expect(meter.textContent).not.toContain('·')
    expect(meter).toHaveAttribute('aria-valuenow', '500')
  })

  it('value = 0 iken çubuk tamamen boş', () => {
    render(<Meter label="X" value={0} max={100} width={8} display="0" />)
    const meter = screen.getByRole('meter')
    expect(meter.textContent).toContain('·'.repeat(8))
    expect(meter.textContent).not.toContain('█')
  })

  it('max = 0 / NaN güvenli (boş çubuk, patlamaz)', () => {
    render(<Meter label="X" value={5} max={0} width={6} display="—" />)
    const meter = screen.getByRole('meter')
    expect(meter.textContent).toContain('·'.repeat(6))
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('accent="rx" throughput ölçeri cyan, eşik rengi kullanmaz', () => {
    render(<Meter label="RX" value={95} max={100} accent="rx" display="9.5G" />)
    expect(screen.getByText('9.5G')).toHaveClass('text-rx')
    expect(screen.getByText('9.5G')).not.toHaveClass('text-rose-400')
  })

  it('erişilebilir meter rolü + aria-value* taşır', () => {
    render(<Meter label="Bellek" value={42} max={64} display="42 GB" />)
    const meter = screen.getByRole('meter', { name: 'Bellek' })
    expect(meter).toHaveAttribute('aria-valuenow', '42')
    expect(meter).toHaveAttribute('aria-valuemax', '64')
    expect(meter).toHaveAttribute('aria-valuemin', '0')
  })
})
