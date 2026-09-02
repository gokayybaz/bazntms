import { useEffect, useRef, useState, useCallback } from 'react'
import type { AlertEvent } from '../types'

// useLive, uyarı olay akışını canlı tutar: /ws üzerinden 'tick' mesajları,
// bağlantı yoksa /api/alerts/events polling'i. WS handshake aynı zamanda
// oturum denetimidir (401 → onAuthRequired). Hub'ın kendi yerel yakalama
// snapshot'ı (top talkers / DNS / protokoller) ve bağlantı listesi bu akıştan
// çıkarıldı — onları gösteren Trafik sayfası kaldırılmıştı (bkz. commit
// "Trafik sayfasını kaldır"); sunucu da artık tick'te yalnızca alert_events
// gönderiyor.
export function useLive(onAuthRequired?: () => void) {
  const [alertEvents, setAlertEvents] = useState<AlertEvent[]>([])
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const pollRef = useRef<number | null>(null)
  const retryRef = useRef<number | null>(null)

  const startPolling = useCallback(() => {
    if (pollRef.current !== null) return
    const poll = async () => {
      try {
        const res = await fetch('/api/alerts/events?limit=20')
        if (res.status === 401) {
          onAuthRequired?.()
          return
        }
        const a = await res.json()
        setAlertEvents(Array.isArray(a) ? a : [])
        setConnected(false)
      } catch {
        /* yoksay */
      }
    }
    void poll()
    pollRef.current = window.setInterval(poll, 5000)
  }, [onAuthRequired])

  const stopPolling = useCallback(() => {
    if (pollRef.current !== null) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  const connect = useCallback(() => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/ws`)
    wsRef.current = ws
    ws.onopen = () => {
      setConnected(true)
      stopPolling()
    }
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as { type?: string; alert_events?: AlertEvent[] }
        if (msg.type === 'tick') {
          setAlertEvents(msg.alert_events ?? [])
        }
      } catch {
        /* yoksay */
      }
    }
    ws.onclose = () => {
      setConnected(false)
      startPolling()
      if (retryRef.current === null) {
        retryRef.current = window.setTimeout(() => {
          retryRef.current = null
          connect()
        }, 3000)
      }
    }
    ws.onerror = () => ws.close()
  }, [startPolling, stopPolling])

  const reconnect = useCallback(() => {
    if (retryRef.current !== null) {
      clearTimeout(retryRef.current)
      retryRef.current = null
    }
    const ws = wsRef.current
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return // zaten bagli; oturum sona erse sunucu tarafindan fark edilir
    }
    connect()
  }, [connect])

  useEffect(() => {
    connect()
    return () => {
      if (retryRef.current !== null) {
        clearTimeout(retryRef.current)
        retryRef.current = null
      }
      stopPolling()
      wsRef.current?.close()
    }
  }, [connect, stopPolling])

  return { alertEvents, connected, reconnect }
}
