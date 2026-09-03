// Trafik yönü sınıflandırması — TrafficFlowDiagram bileşeni ve testi arasında
// paylaşılır. Ayrı dosyada tutulması Fast Refresh'in bileşen dosyalarında
// düzgün çalışmasını sağlar (oxlint: only-export-components).

export type TrafficDir = 'out' | 'in' | 'lan' | 'log'

/** "1.2.3.4:443" → "1.2.3.4", "[::1]:53" → "::1", çıplak ipv6/hostname → aynen */
export function stripPort(a: string): string {
  const s = (a || '').trim()
  if (s.startsWith('[')) {
    const e = s.indexOf(']')
    return e > 0 ? s.slice(1, e) : s.slice(1)
  }
  const parts = s.split(':')
  return parts.length === 2 ? parts[0] : s
}

/** RFC1918 / loopback / link-local / CGNAT ve ip olmayan (hostname) → yerel say */
export function isPrivate(raw: string): boolean {
  const h = stripPort(raw).toLowerCase()
  if (!h) return true
  if (h === 'localhost' || h === '::1' || h.startsWith('127.')) return true
  if (h.startsWith('10.') || h.startsWith('192.168.') || h.startsWith('169.254.')) return true
  const m = h.match(/^172\.(\d{1,3})\./)
  if (m && +m[1] >= 16 && +m[1] <= 31) return true
  const c = h.match(/^100\.(\d{1,3})\./)
  if (c && +c[1] >= 64 && +c[1] <= 127) return true
  if (h.startsWith('fe80:') || h.startsWith('fc') || h.startsWith('fd')) return true
  if (!/^[0-9a-f.:]+$/.test(h)) return true // ip değil → cihaz adı, yerel varsay
  return false
}

/** kaynak/hedef özel-genel eksenine göre paketin gideceği yön */
export function classifyDir(ev: { kind: 'flow' | 'agent' | 'syslog'; from: string; to?: string }): TrafficDir {
  if (ev.kind === 'syslog') return 'log'
  const fromPriv = isPrivate(ev.from)
  if (!ev.to) return 'lan'
  const toPriv = isPrivate(ev.to)
  if (fromPriv && !toPriv) return 'out'
  if (!fromPriv && toPriv) return 'in'
  if (fromPriv && toPriv) return 'lan'
  return 'in' // her iki uç da public — dışarıdan geliyormuş gibi göster
}

function hashStr(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

/** bir IP'yi 3 istemci şeridinden birine kararlı biçimde eşle */
export function laneFor(ip: string): number {
  return hashStr(stripPort(ip)) % 3
}
