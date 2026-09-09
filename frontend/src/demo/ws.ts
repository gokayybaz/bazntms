// Demo katmanı — sahte WebSocket. useLive.ts yalnızca `/ws` adresine bağlanır
// ve saniyede bir `{type:'tick', fleet, alert_events}` bekler. Gerçek
// WebSocket'i patch'lemek yerine, yalnız `/ws` isteklerini bu sınıfa
// yönlendiririz (install.ts).

import { fleetSummary, legacyAlerts } from './world'

type Listener = (ev: any) => void

export class DemoWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3
  readonly CONNECTING = 0
  readonly OPEN = 1
  readonly CLOSING = 2
  readonly CLOSED = 3

  url: string
  readyState = 0
  onopen: Listener | null = null
  onmessage: Listener | null = null
  onclose: Listener | null = null
  onerror: Listener | null = null
  private timer: number | undefined
  private killed = false
  private listeners: Record<string, Listener[]> = {}

  constructor(url: string) {
    this.url = url
    setTimeout(() => this.open(), 40)
  }

  private emit(type: string, ev: any) {
    const cb = (this as any)['on' + type] as Listener | null
    if (cb) cb(ev)
    for (const l of this.listeners[type] ?? []) l(ev)
  }

  private open() {
    if (this.killed || this.readyState === 3) return
    this.readyState = 1
    this.emit('open', { type: 'open' })
    const push = () => {
      if (this.readyState !== 1) return
      const payload = JSON.stringify({
        type: 'tick',
        fleet: fleetSummary(),
        alert_events: legacyAlerts(20),
      })
      this.emit('message', { type: 'message', data: payload })
    }
    push()
    this.timer = window.setInterval(push, 1000)
  }

  send() {
    /* demo: sunucuya gönderilecek bir şey yok */
  }

  close() {
    this.killed = true
    this.readyState = 3
    if (this.timer) window.clearInterval(this.timer)
    this.emit('close', { type: 'close', code: 1000, wasClean: true })
  }

  addEventListener(type: string, cb: Listener) {
    ;(this.listeners[type] ??= []).push(cb)
  }
  removeEventListener(type: string, cb: Listener) {
    this.listeners[type] = (this.listeners[type] ?? []).filter((l) => l !== cb)
  }
}
