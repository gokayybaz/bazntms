// Demo katmanı kurulumu — App mount olmadan ÖNCE çağrılır (main.tsx).
// window.fetch + WebSocket'i sentetik veriye bağlar, tick döngüsünü başlatır,
// backend gerektiren indirme linklerini (rapor/delil paketi) nazikçe engeller.

import { initWorld, tick } from './world'
import { demoHandle } from './http'
import { DemoWebSocket } from './ws'

let installed = false

export function installDemo(): void {
  if (installed) return
  installed = true

  initWorld()
  window.setInterval(tick, 1000)

  // --- fetch ---
  const realFetch = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    let rawUrl: string
    let method = init?.method || 'GET'
    if (typeof input === 'string') rawUrl = input
    else if (input instanceof URL) rawUrl = input.href
    else {
      rawUrl = input.url
      method = input.method || method
    }
    let url: URL
    try {
      url = new URL(rawUrl, window.location.origin)
    } catch {
      return realFetch(input as RequestInfo, init)
    }
    if (!/\/api(\/|$)/.test(url.pathname)) return realFetch(input as RequestInfo, init)

    // hafif yapay gecikme — "Yükleniyor…" durumları bir an görünsün
    await new Promise((r) => setTimeout(r, 45 + Math.random() * 90))
    try {
      const res = await demoHandle(method.toUpperCase(), url, init)
      if (res) return res
    } catch (e) {
      return new Response(JSON.stringify({ error: String(e) }), { status: 500, headers: { 'content-type': 'application/json' } })
    }
    return realFetch(input as RequestInfo, init)
  }

  // --- WebSocket (yalnız /ws) ---
  const RealWS = window.WebSocket
  function PatchedWS(this: unknown, url: string | URL, protocols?: string | string[]) {
    const u = String(url)
    if (/\/ws(\?|$)/.test(u)) return new DemoWebSocket(u) as unknown as WebSocket
    return new RealWS(url, protocols)
  }
  PatchedWS.prototype = RealWS.prototype
  Object.assign(PatchedWS, {
    CONNECTING: RealWS.CONNECTING, OPEN: RealWS.OPEN, CLOSING: RealWS.CLOSING, CLOSED: RealWS.CLOSED,
  })
  window.WebSocket = PatchedWS as unknown as typeof WebSocket

  // --- <a href="/api/..."> (rapor / denetçi paketi / delil) ve düz app-route linkleri ---
  document.addEventListener(
    'click',
    (e) => {
      const t = e.target as HTMLElement | null
      const a = t?.closest?.('a')
      if (!a) return
      const href = a.getAttribute('href') || ''
      if (/^(\/bazntms\/demo)?\/api\//.test(href)) {
        e.preventDefault()
        showToast('Demo modunda dosya indirme / rapor görüntüleme devre dışı.')
        return
      }
      // href="/uyumluluk" gibi düz app linkleri — HashRouter'a çevir
      if (/^\/(?!\/|bazntms\/demo)/.test(href) && a.getAttribute('target') !== '_blank') {
        e.preventDefault()
        window.location.hash = '#' + href
      }
    },
    true,
  )

  // eslint-disable-next-line no-console
  console.info('%cbazNTMS DEMO', 'color:#22d3ee;font-weight:bold', '— sentetik veri, backend yok. window.fetch + WebSocket taklit ediliyor.')
}

let toastEl: HTMLDivElement | null = null
let toastTimer: number | undefined
function showToast(msg: string): void {
  if (!toastEl) {
    toastEl = document.createElement('div')
    toastEl.style.cssText =
      'position:fixed;left:50%;bottom:2.5rem;transform:translateX(-50%);z-index:9999;' +
      'background:#10141d;color:#ced7e3;border:1px solid #35485f;padding:.55rem .9rem;' +
      'font:12px ui-monospace,monospace;max-width:90vw;text-align:center;pointer-events:none'
    document.body.appendChild(toastEl)
  }
  toastEl.textContent = msg
  toastEl.style.opacity = '1'
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    if (toastEl) toastEl.style.opacity = '0'
  }, 2600)
  toastEl.style.transition = 'opacity .3s'
}
