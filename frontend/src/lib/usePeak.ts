import { useEffect, useRef } from 'react'

// usePeak — bir metriğin oturum-içi tepe değerini izler. Meter'lara ölçek
// (max) verir: throughput/pps'in sabit bir üst sınırı yok, "bugünün tepesine
// göre %" en anlamlı gösterge. Bileşen zaten saniyede yeniden render olduğu
// için ref güncellemesi effect'te yeterli.
export function usePeak(value: number): number {
  const ref = useRef(1)
  useEffect(() => {
    if (Number.isFinite(value) && value > ref.current) ref.current = value
  }, [value])
  return Math.max(ref.current, 1)
}
