export function formatBytes(n: number, digits = 1): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : digits)} ${units[i]}`
}

export function formatBits(bps: number): string {
  if (!Number.isFinite(bps) || bps <= 0) return '0 bit/s'
  const units = ['bit/s', 'Kbit/s', 'Mbit/s', 'Gbit/s']
  let i = 0
  let v = bps
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatNum(n: number): string {
  return new Intl.NumberFormat('tr-TR').format(n)
}

export function flagEmoji(cc?: string): string {
  if (!cc || cc.length !== 2 || !/^[a-zA-Z]{2}$/.test(cc)) return ''
  return String.fromCodePoint(
    ...cc
      .toUpperCase()
      .split('')
      .map((c) => 0x1f1a5 + c.charCodeAt(0)),
  )
}

export function formatDuration(iso?: string): string {
  if (!iso) return '-'
  const start = new Date(iso).getTime()
  if (Number.isNaN(start)) return '-'
  const secs = Math.max(0, Math.floor((Date.now() - start) / 1000))
  const m = Math.floor(secs / 60)
  const s = secs % 60
  const h = Math.floor(m / 60)
  if (h > 0) return `${h}s ${m % 60}d`
  return `${m}d ${s}sn`
}
