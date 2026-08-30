import { describe, expect, it } from 'vitest'
import { flagEmoji, formatBits, formatBytes, formatDuration, formatNum } from './format'

describe('formatBytes', () => {
  it('sıfır ve negatif için 0 B döner', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(-5)).toBe('0 B')
  })

  it('birimler arasında doğru ölçeklenir', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB')
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.0 GB')
  })

  it('ondalık basamak sayısı ayarlanabilir', () => {
    expect(formatBytes(1536, 2)).toBe('1.50 KB')
  })
})

describe('formatBits', () => {
  it('sıfır ve negatif için 0 bit/s döner', () => {
    expect(formatBits(0)).toBe('0 bit/s')
    expect(formatBits(-1)).toBe('0 bit/s')
  })

  it('1000 tabanında ölçeklenir (bayt değil bit)', () => {
    expect(formatBits(500)).toBe('500 bit/s')
    expect(formatBits(1000)).toBe('1.0 Kbit/s')
    expect(formatBits(1_000_000)).toBe('1.0 Mbit/s')
    expect(formatBits(1_000_000_000)).toBe('1.0 Gbit/s')
  })
})

describe('formatNum', () => {
  it('tr-TR yerel ayarına göre binlik ayraç kullanır', () => {
    expect(formatNum(1000)).toBe('1.000')
    expect(formatNum(1234567)).toBe('1.234.567')
    expect(formatNum(0)).toBe('0')
  })
})

describe('flagEmoji', () => {
  it('geçersiz ülke kodları için boş string döner', () => {
    expect(flagEmoji(undefined)).toBe('')
    expect(flagEmoji('')).toBe('')
    expect(flagEmoji('TUR')).toBe('') // 3 harfli kod gecersiz
    expect(flagEmoji('1r')).toBe('')
  })

  it('geçerli 2 harfli kod için bayrak emojisi üretir', () => {
    expect(flagEmoji('TR')).toBe('🇹🇷')
    expect(flagEmoji('us')).toBe('🇺🇸') // kucuk harf de kabul edilir
  })
})

describe('formatDuration', () => {
  it('tarih verilmemişse veya geçersizse "-" döner', () => {
    expect(formatDuration(undefined)).toBe('-')
    expect(formatDuration('gecersiz-tarih')).toBe('-')
  })

  it('bir saatten kısa sürede dakika:saniye formatı kullanır', () => {
    const iso = new Date(Date.now() - 90_000).toISOString() // 90 sn once
    expect(formatDuration(iso)).toMatch(/^1d \d{1,2}sn$/)
  })

  it('bir saatten uzun sürede saat:dakika formatı kullanır', () => {
    const iso = new Date(Date.now() - 2 * 3600_000 - 5 * 60_000).toISOString() // 2sa 5dk once
    expect(formatDuration(iso)).toMatch(/^2s \d{1,2}d$/)
  })
})
