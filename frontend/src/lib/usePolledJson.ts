import { useEffect, useRef, useState } from 'react'

// usePolledJson — bir JSON ucunu periyodik çeker. İlk veri gelene kadar geçici
// hata (hub yeniden başlarken 401/502, ağ kesintisi) durumunda hızlı yeniden
// dener; böylece kart "Yükleniyor…" ekranında takılı kalmaz. İlk başarılı
// yüklemeden sonra yalnızca normal aralıkta pollanır (hata sessizce yutulur,
// bir sonraki poll düzeltir).
export function usePolledJson<T>(
  url: string | null,
  intervalMs: number,
): { data: T | null; loaded: boolean } {
  const [data, setData] = useState<T | null>(null)
  const [loaded, setLoaded] = useState(false)
  const loadedRef = useRef(false)

  useEffect(() => {
    if (!url) return
    let stop = false
    let tries = 0
    const load = async () => {
      tries++
      try {
        const res = await fetch(url)
        if (!res.ok) throw new Error(String(res.status))
        const json = (await res.json()) as T
        if (!stop) {
          loadedRef.current = true
          setData(json)
          setLoaded(true)
        }
      } catch {
        // ilk veri gelene kadar hızlanan yeniden deneme: 400ms → 2s
        if (!stop && !loadedRef.current && tries < 25) {
          window.setTimeout(load, Math.min(2_000, 400 * tries))
        }
      }
    }
    load()
    const id = window.setInterval(load, intervalMs)
    return () => {
      stop = true
      window.clearInterval(id)
    }
  }, [url, intervalMs])

  return { data, loaded }
}
