import { describe, expect, it } from 'vitest'
import { classifyDir, isPrivate, laneFor, stripPort } from './traffic'

describe('stripPort', () => {
  it('ipv4:port → ip', () => expect(stripPort('10.0.0.5:443')).toBe('10.0.0.5'))
  it('[ipv6]:port → ipv6', () => expect(stripPort('[2001:db8::1]:53')).toBe('2001:db8::1'))
  it('çıplak ipv6 aynen kalır', () => expect(stripPort('fe80::1')).toBe('fe80::1'))
  it('hostname aynen kalır', () => expect(stripPort('switch-1')).toBe('switch-1'))
})

describe('isPrivate', () => {
  it('RFC1918 blokları', () => {
    expect(isPrivate('10.1.2.3')).toBe(true)
    expect(isPrivate('192.168.1.1')).toBe(true)
    expect(isPrivate('172.16.0.1')).toBe(true)
    expect(isPrivate('172.31.255.1')).toBe(true)
  })
  it('172.32.x genel', () => expect(isPrivate('172.32.0.1')).toBe(false))
  it('loopback / link-local / CGNAT', () => {
    expect(isPrivate('127.0.0.1')).toBe(true)
    expect(isPrivate('169.254.10.1')).toBe(true)
    expect(isPrivate('100.100.0.1')).toBe(true)
  })
  it('genel IP', () => {
    expect(isPrivate('8.8.8.8')).toBe(false)
    expect(isPrivate('1.1.1.1:443')).toBe(false)
  })
  it('hostname → yerel varsay', () => expect(isPrivate('fw-ofis')).toBe(true))
})

describe('classifyDir', () => {
  it('özel → genel = giden (out)', () =>
    expect(classifyDir({ kind: 'flow', from: '10.0.0.5', to: '8.8.8.8:53' })).toBe('out'))
  it('genel → özel = gelen (in)', () =>
    expect(classifyDir({ kind: 'flow', from: '8.8.8.8', to: '10.0.0.5:44000' })).toBe('in'))
  it('özel → özel = yerel (lan)', () =>
    expect(classifyDir({ kind: 'agent', from: '10.0.0.1:1000', to: '10.0.0.2:22' })).toBe('lan'))
  it('hedefsiz soket = yerel (lan)', () =>
    expect(classifyDir({ kind: 'agent', from: '0.0.0.0:8080' })).toBe('lan'))
  it('syslog daima log', () =>
    expect(classifyDir({ kind: 'syslog', from: 'switch-1' })).toBe('log'))
  it('genel → genel dışarıdan gelir gibi (in)', () =>
    expect(classifyDir({ kind: 'flow', from: '8.8.8.8', to: '1.1.1.1:443' })).toBe('in'))
})

describe('laneFor', () => {
  it('0-2 aralığında ve port değişiminden bağımsız kararlı', () => {
    const a = laneFor('10.0.0.42:1234')
    expect(a).toBeGreaterThanOrEqual(0)
    expect(a).toBeLessThan(3)
    expect(laneFor('10.0.0.42:9999')).toBe(a)
  })
})
