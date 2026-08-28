import { useEffect, useRef, useState, useCallback } from 'react'
import type { AlertEvent, Connection, RecordInfo, Snapshot } from '../types'

const EMPTY: Snapshot = {
  running: false,
  total_packets: 0,
  total_bytes: 0,
  dropped: 0,
  bps_in: 0,
  bps_out: 0,
  bps_local: 0,
  pps: 0,
  protocols: {},
  top_endpoints: [],
  top_ports: [],
  top_domains: [],
  history: [],
  local_ip_count: 0,
}

export function useLive(onAuthRequired?: () => void) {
  const [stats, setStats] = useState<Snapshot>(EMPTY)
  const [connections, setConnections] = useState<Connection[]>([])
  const [alertEvents, setAlertEvents] = useState<AlertEvent[]>([])
  const [record, setRecord] = useState<RecordInfo>({ recording: false, packets: 0, bytes: 0 })
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const pollRef = useRef<number | null>(null)
  const retryRef = useRef<number | null>(null)

  const startPolling = useCallback(() => {
    if (pollRef.current !== null) return
    const poll = async () => {
      try {
        const res = await fetch('/api/status')
        if (res.status === 401) {
          onAuthRequired?.()
          return
        }
        const s = await res.json()
        const [c, a] = await Promise.all([
          fetch('/api/connections').then((r) => r.json()),
          fetch('/api/alerts/events?limit=20').then((r) => r.json()),
        ])
        setStats(s)
        setConnections(Array.isArray(c) ? c : [])
        setAlertEvents(Array.isArray(a) ? a : [])
        fetch('/api/record/status').then((r) => r.json()).then(setRecord).catch(() => {})
        setConnected(false)
      } catch {
        /* yoksay */
      }
    }
    void poll()
    pollRef.current = window.setInterval(poll, 2000)
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
        const msg = JSON.parse(ev.data) as {
          type?: string
          stats?: Snapshot
          connections?: Connection[]
          alert_events?: AlertEvent[]
          record?: RecordInfo
        }
        if (msg.type === 'tick' && msg.stats) {
          setStats(msg.stats)
          setConnections(msg.connections ?? [])
          setAlertEvents(msg.alert_events ?? [])
          if (msg.record) setRecord(msg.record)
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

  return { stats, connections, alertEvents, record, connected, reconnect }
}
