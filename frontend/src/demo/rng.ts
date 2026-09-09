// Demo katmanı — tohumlanabilir RNG. İlk dünya kurulumu deterministik olsun
// diye (SSR yok ama ekran görüntüleri kararlı kalsın); tick mutasyonları
// Math.random kullanır (canlılık hissi).

export function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** rnd'ye bağlı, çağrı başına jenerik seçici: `const pick = makePicker(rnd)` */
export function makePicker(rnd: () => number) {
  return <T>(arr: readonly T[]): T => arr[Math.floor(rnd() * arr.length)]
}

/** [min, max] arası tamsayı */
export function ri(rnd: () => number, min: number, max: number): number {
  return Math.floor(rnd() * (max - min + 1)) + min
}

/** basit string hash — sorgu parametrelerinden deterministik tohum üretmek için */
export function hashStr(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}
