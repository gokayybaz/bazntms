import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Sparkline, toSparkChars } from './Sparkline'

describe('toSparkChars', () => {
  it('artan diziyi düşükten yükseğe rampa eşler', () => {
    const s = toSparkChars([0, 1, 2, 3, 4, 5, 6, 7])
    expect(s[0]).toBe('▁')
    expect(s[s.length - 1]).toBe('█')
  })
  it('düz dizi orta seviye verir (sıfır değilse)', () => {
    expect(toSparkChars([5, 5, 5])).toBe('▅▅▅')
  })
  it('tümü sıfır → taban seviye', () => {
    expect(toSparkChars([0, 0, 0])).toBe('▁▁▁')
  })
  it('boş dizi → boş string', () => {
    expect(toSparkChars([])).toBe('')
  })
  it('NaN güvenli', () => {
    expect(() => toSparkChars([1, NaN, 3])).not.toThrow()
  })
})

describe('Sparkline', () => {
  it('son N noktayı çizer', () => {
    render(<Sparkline data={[1, 2, 3, 4, 5]} points={3} label="trend" />)
    expect(screen.getByRole('img', { name: 'trend' }).textContent).toHaveLength(3)
  })
  it('veri yoksa nokta gösterir', () => {
    render(<Sparkline data={[]} label="trend" />)
    expect(screen.getByRole('img', { name: 'trend' }).textContent).toBe('·')
  })
})
