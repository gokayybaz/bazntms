// Demo katmanı — /api/* isteklerini karşılayan yönlendirici. install.ts
// window.fetch'i buraya bağlar; /api dışı ve /ws istekleri gerçek fetch'e döner.

import { world, now, agentJSON, geoRows, topologyGraph, legacyAlerts, type DemoAgent } from './world'
import * as fx from './fixtures'
import * as d from './derive'
import { sseResponse } from './sse'
import { ri } from './rng'

const rr = () => Math.random()

function json(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), { status, headers: { 'content-type': 'application/json' } })
}
const OK = () => json({ ok: true })

function num(v: string | null, def: number): number {
  const n = Number(v)
  return Number.isFinite(n) && n > 0 ? n : def
}

async function readBody(init?: RequestInit): Promise<any> {
  try {
    if (init?.body && typeof init.body === 'string') return JSON.parse(init.body)
  } catch { /* yoksay */ }
  return {}
}

function agentById(id: number): DemoAgent | undefined {
  return world.agents.find((a) => a.id === id)
}

function deviceJSON(dev: (typeof world.devices)[number]) {
  return {
    id: dev.id, name: dev.name, host: dev.host, kind: dev.kind, site: dev.site,
    vendor: dev.vendor, snmp_version: dev.snmp_version, api_url: dev.api_url,
    api_verify_tls: dev.api_verify_tls, vdom: dev.vdom, poll_seconds: dev.poll_seconds,
    enabled: dev.enabled, sys_name: dev.sys_name, sys_descr: dev.sys_descr,
    added_at: dev.added_at, last_poll: dev.last_poll, last_error: dev.last_error,
  }
}

function lifecycleEvents(q: URLSearchParams) {
  const sev = q.get('severity') || ''
  const state = q.get('state') || ''
  const kind = q.get('kind') || ''
  const since = Number(q.get('since') || 0)
  const limit = num(q.get('limit'), 100)
  const cursor = Number(q.get('cursor') || 0)
  let rows = world.alerts.slice().sort((a, b) => b.last_ts - a.last_ts)
  if (sev) rows = rows.filter((e) => e.severity === sev)
  if (state) rows = rows.filter((e) => e.state === state)
  if (kind) rows = rows.filter((e) => e.kind === kind)
  if (since) rows = rows.filter((e) => e.last_ts >= since)
  const start = cursor || 0
  const page = rows.slice(start, start + limit)
  const next = start + limit < rows.length ? start + limit : 0
  return { events: page, next_cursor: next }
}

function incidentDetail(id: number) {
  const inc = world.incidents.find((i) => i.id === id)
  if (!inc) return null
  const evidence = [
    { kind: 'alert', ref: `alert#${1000 + id}`, ts: inc.first_seen, summary: inc.correlation_reason.split('+')[0].trim() },
    { kind: 'event', ref: `ev#${2000 + id}`, ts: inc.first_seen + 60, summary: 'DNS beacon deseni gözlendi' },
    { kind: 'alert', ref: `alert#${1001 + id}`, ts: inc.first_seen + 180, summary: 'yeni hedefe hacimli trafik' },
    { kind: 'event', ref: `ev#${2001 + id}`, ts: inc.last_seen, summary: 'korelasyon penceresi güncellendi' },
  ]
  return { incident: inc, evidence }
}

export async function demoHandle(method: string, url: URL, init?: RequestInit): Promise<Response | null> {
  const p = url.pathname.replace(/^\/bazntms\/demo/, '')
  const q = url.searchParams
  const seg = p.split('/').filter(Boolean) // ['api','v1',...]

  // --- auth / alerts (legacy) ---
  if (p === '/api/auth/status') return json(fx.authStatus)
  if (p === '/api/login') return json({ ok: true, username: 'demo', role: 'viewer' })
  if (p === '/api/logout') return OK()
  if (p === '/api/alerts/events') return json(legacyAlerts(num(q.get('limit'), 20)))
  if (p === '/api/alerts' && method === 'GET') return json(fx.alertConfig)
  if (p === '/api/alerts' && method === 'PUT') return OK()
  if (p === '/api/alerts/status') return json(fx.alertStatus)
  if (p === '/api/alerts/test') return json({ channels: { slack: { last_attempt: now(), ok: true }, siem: { last_attempt: now(), ok: true } } })
  if (p === '/api/report' || p.startsWith('/api/report')) return new Response('<!doctype html><meta charset=utf-8><title>Demo</title><body style="font:14px system-ui;padding:2rem;background:#0a0d13;color:#ced7e3"><h1>Demo modu</h1><p>Rapor üretimi demoda devre dışı. Gerçek kurulumda burada HTML/PDF rapor açılır.</p>', { headers: { 'content-type': 'text/html' } })

  if (seg[0] !== 'api') return null
  const v1 = seg[1] === 'v1'
  const r = v1 ? seg.slice(2) : seg.slice(1)

  // ================= v1 =================
  if (v1) {
    // --- agents ---
    if (r[0] === 'agents') {
      if (r.length === 1 && method === 'GET') return json(world.agents.map(agentJSON))
      const id = Number(r[1])
      const a = agentById(id)
      if (r.length === 2) {
        if (method === 'GET') {
          if (!a) return json({ error: 'not found' }, 404)
          return json({ agent: agentJSON(a), connections: a.connSamples })
        }
        if (method === 'PATCH') {
          const b = await readBody(init)
          if (a && b.name) a.name = String(b.name)
          return OK()
        }
        if (method === 'DELETE') {
          world.agents = world.agents.filter((x) => x.id !== id)
          return OK()
        }
      }
      if (r[2] === 'history') return json(d.agentHistory(num(q.get('minutes'), 60)))
      if (r[2] === 'uplink' && method === 'PUT') {
        const b = await readBody(init)
        if (a) a.uplink_device_id = b.device_id ?? undefined
        return OK()
      }
      if (r[2] === 'processes' && r[3]) {
        if (!a) return json({ error: 'not found' }, 404)
        return json(d.processDetail(a, decodeURIComponent(r[3]), num(q.get('minutes'), 60)))
      }
    }

    // --- devices ---
    if (r[0] === 'devices') {
      if (r.length === 1) {
        if (method === 'GET') return json(world.devices.map(deviceJSON))
        if (method === 'POST') {
          const b = await readBody(init)
          const id = world.devices.length + 1
          world.devices.push({
            id, name: b.name || `cihaz-${id}`, host: b.host || '', kind: b.kind || 'switch',
            site: b.site || '', vendor: b.vendor || (b.host ? 'snmp' : 'virtual'),
            snmp_version: b.snmp_version || 0, api_url: b.api_url || '', api_verify_tls: false,
            vdom: b.vdom || '', poll_seconds: 60, enabled: true, sys_name: b.name || `cihaz-${id}`,
            sys_descr: b.host ? 'demo cihazı' : 'sanal düğüm (poll yok)', added_at: now(),
            last_poll: b.host ? 0 : 0, last_error: '', ifCount: 8,
          })
          return OK()
        }
      }
      const id = Number(r[1])
      if (r.length === 2 && method === 'DELETE') {
        world.devices = world.devices.filter((x) => x.id !== id)
        for (const a of world.agents) if (a.uplink_device_id === id) a.uplink_device_id = undefined
        return OK()
      }
      if (r[2] === 'interfaces') return json(d.deviceInterfaces(id))
      if (r[2] === 'resources') return json(d.fortiResources(num(q.get('minutes'), 180)))
      if (r[2] === 'vpn') return json(d.fortiVpn())
      if (r[2] === 'sdwan') return json(d.fortiSdwan(num(q.get('minutes'), 30)))
      if (r[2] === 'policies') return json(d.fortiPolicies())
    }

    // --- flows ---
    if (r[0] === 'flows') {
      if (r[1] === 'conversations') return json(d.flowConversations(q.get('window') || '15m', q.get('by') || 'pair', q.get('sort') || 'octets', num(q.get('limit'), 25)))
      if (r[1] === 'conversation') return json(d.flowConversationDrill(q.get('window') || '15m', q.get('src') || '', q.get('dst') || '', q.get('proto') || undefined))
      const minutes = num(q.get('minutes'), 15)
      const limit = num(q.get('limit'), 20)
      const horizon = now() - minutes * 60
      const rows = world.flows.filter((f) => f.ts >= horizon).sort((a, b) => b.ts - a.ts).slice(0, limit)
      return json(rows)
    }

    if (r[0] === 'syslog') {
      const limit = num(q.get('limit'), 20)
      return json(world.syslog.slice().sort((a, b) => b.ts - a.ts).slice(0, limit))
    }
    if (r[0] === 'topology') return json(topologyGraph())
    if (r[0] === 'geo') return json(geoRows(num(q.get('minutes'), 60)))
    if (r[0] === 'health') return json(fx.health())
    if (r[0] === 'processes') return json(d.processes(num(q.get('minutes'), 60), num(q.get('limit'), 20), q.get('agent_id') ? Number(q.get('agent_id')) : undefined))
    if (r[0] === 'l7') return json(d.l7(num(q.get('minutes'), 60), num(q.get('limit'), 30), q.get('agent_id') ? Number(q.get('agent_id')) : undefined))
    if (r[0] === 'dns') return json(d.dns(num(q.get('minutes'), 60), num(q.get('limit'), 30), q.get('agent_id') ? Number(q.get('agent_id')) : undefined))
    if (r[0] === 'events') return json(d.events(num(q.get('since_min'), 180), num(q.get('limit'), 100), q.get('before') ? Number(q.get('before')) : undefined, (q.get('type') || '').split(',').filter(Boolean), q.get('agent_id') ? Number(q.get('agent_id')) : undefined))
    if (r[0] === 'enrich') return json({})
    if (r[0] === 'threatintel') return json({ enabled: true, feeds: [{ name: 'demo-abuse.ch', entries: 41823, updated_ts: now() - 3600 }], matches_24h: 3 })

    // --- anomaly ---
    if (r[0] === 'anomaly') {
      if (r[1] === 'baseline') return json(d.anomalyBaseline(q.get('dim') || 'fleet', q.get('metric') || 'bps'))
      if (r[1] === 'active') return json(d.anomalyActive())
    }

    // --- alerts (lifecycle) ---
    if (r[0] === 'alerts') {
      if (r[1] === 'events' && r.length === 2) return json(lifecycleEvents(q))
      if (r[1] === 'events' && r[3]) {
        const id = Number(r[2])
        const ev = world.alerts.find((e) => e.id === id)
        const b = await readBody(init)
        if (ev) {
          if (r[3] === 'ack') { ev.state = 'ack'; ev.ack_by = 'demo'; if (b.note) ev.note = b.note }
          else if (r[3] === 'resolve') ev.state = 'resolved'
          else if (r[3] === 'note') ev.note = b.note || ev.note
        }
        return OK()
      }
      if (r[1] === 'silences') {
        if (method === 'GET') return json({ silences: world.silences })
        if (method === 'POST') {
          const b = await readBody(init)
          world.silences.push({
            id: world.silences.length + 1, match_kind: b.match_kind || '', match_site: b.match_site || '',
            match_key: b.match_key || '', starts_ts: now(), ends_ts: now() + (Number(b.duration_min) || 60) * 60,
            reason: b.reason || '', created_by: 'demo',
          })
          return OK()
        }
        if (method === 'DELETE') {
          const id = Number(r[2])
          world.silences = world.silences.filter((s: any) => s.id !== id)
          return OK()
        }
      }
    }

    // --- incidents ---
    if (r[0] === 'incidents') {
      if (r.length === 1) {
        const status = q.get('status') || ''
        const limit = num(q.get('limit'), 100)
        let rows = world.incidents.slice()
        if (status === 'open') rows = rows.filter((i) => i.status === 'open' || i.status === 'investigating')
        else if (status) rows = rows.filter((i) => i.status === status)
        return json({ incidents: rows.slice(0, limit) })
      }
      const id = Number(r[1])
      if (r.length === 2) {
        const det = incidentDetail(id)
        return det ? json(det) : json({ error: 'not found' }, 404)
      }
      if (r[2] && method === 'POST') {
        const inc = world.incidents.find((i) => i.id === id)
        if (inc) {
          const map: Record<string, Incident['status']> = { ack: 'investigating', investigate: 'investigating', resolve: 'resolved', close: 'closed' }
          if (map[r[2]]) inc.status = map[r[2]]
          if (r[2] === 'ack') inc.ack_by = 'demo'
          if (r[2] === 'resolve' || r[2] === 'close') inc.resolved_ts = now()
          inc.updated_ts = now()
        }
        return OK()
      }
    }

    // --- audit ---
    if (r[0] === 'audit') {
      if (r[1] === 'verify') return json(fx.auditVerify)
      return json(fx.auditEvents(num(q.get('limit'), 100)))
    }

    // --- isms ---
    if (r[0] === 'isms') {
      if (method !== 'GET') return OK()
      switch (r[1]) {
        case 'summary': return json(fx.ismsSummary)
        case 'assets': return json(fx.ismsAssets)
        case 'risks': return json(fx.ismsRisks)
        case 'soa': return json(fx.ismsSoa)
        case 'policies': return json(fx.ismsPolicies)
        case 'audits':
          if (r[2] && r[3] === 'findings') return json(fx.ismsFindings[Number(r[2])] ?? [])
          return json(fx.ismsAudits)
        case 'mgmt-reviews': return json(fx.ismsReviews)
        case 'suppliers': return json(fx.ismsSuppliers)
        case 'continuity': return json(fx.ismsContinuity)
      }
      if (r[1] === 'auditor-package') return json({ note: 'demo — denetçi paketi devre dışı' })
      return json([])
    }

    // --- compliance ---
    if (r[0] === 'compliance') {
      if (r[1] === 'status') return json(fx.complianceStatus)
      if (r[1] === 'reviews' && method === 'GET') return json(fx.complianceReviews)
      if (r[1] === 'reviews' && method === 'POST') return OK()
      if (r[1] === 'evidence') return json({ note: 'demo — delil paketi devre dışı' })
    }

    // --- sla / reports ---
    if (r[0] === 'sla' && r[1] === 'targets') return method === 'GET' ? json(fx.slaTargets) : OK()
    if (r[0] === 'reports') {
      if (r[1] === 'schedules' && method === 'GET') return json(fx.reportSchedules)
      if (r[1] === 'archive' && method === 'GET') return json(fx.reportArchive)
      if (r[1] === 'archive' && r[2]) return new Response('demo raporu (içerik yok)', { headers: { 'content-type': 'text/plain' } })
      return OK()
    }

    // --- users / tokens / enroll ---
    if (r[0] === 'users') {
      if (r.length === 1 && method === 'GET') return json(fx.users)
      return OK()
    }
    if (r[0] === 'tokens') {
      if (r.length === 1 && method === 'GET') return json(fx.tokens)
      if (method === 'POST') return json({ token: `bnt_demo_${Math.random().toString(36).slice(2, 14)}` })
      return OK()
    }
    if (r[0] === 'enroll-tokens') {
      if (r.length === 1 && method === 'GET') return json(fx.enrollTokens)
      if (method === 'POST') return json({ token: `enr_demo_${Math.random().toString(36).slice(2, 14)}`, id: fx.enrollTokens.length + 1 })
      return OK()
    }

    // --- ai ---
    if (r[0] === 'ai') {
      if (r[1] === 'status') return json(fx.aiStatus)
      if (r[1] === 'presets') return json(fx.aiPresets)
      if (r[1] === 'providers') {
        if (r.length === 2 && method === 'GET') return json(fx.aiProviders)
        if (r[3] === 'test') return json({ ok: true, model: 'llama3.1:8b', latency_ms: ri(rr, 120, 900) })
        return OK()
      }
      if (r[1] === 'conversations') {
        if (r.length === 2 && method === 'GET') {
          const scope = q.get('scope')
          const ref = q.get('ref')
          const source = q.get('source')
          let list = world.conversations.slice().sort((a, b) => b.updated_ts - a.updated_ts)
          if (scope) list = list.filter((c) => c.scope_kind === scope)
          if (ref) list = list.filter((c) => String(c.scope_ref) === String(ref))
          if (source) list = list.filter((c) => c.source === source)
          return json(list)
        }
        if (r.length === 2 && method === 'POST') {
          const b = await readBody(init)
          const conv = {
            id: world.seq.conv++, title: 'Yeni sohbet', created_by: 'demo', site: '',
            scope_kind: b.scope_kind || 'fleet', scope_ref: b.scope_ref || '',
            provider_id: b.provider_id || 1, model: fx.aiStatus.default_model, source: 'user',
            created_ts: now(), updated_ts: now(), archived: false,
          }
          world.conversations.unshift(conv)
          world.messages[conv.id] = []
          return json({ id: conv.id })
        }
        const id = Number(r[2])
        if (r.length === 3 && method === 'GET') {
          const conv = world.conversations.find((c) => c.id === id)
          if (!conv) return json({ error: 'not found' }, 404)
          return json({ conversation: conv, messages: world.messages[id] ?? [] })
        }
        if (r.length === 3 && method === 'DELETE') {
          world.conversations = world.conversations.filter((c) => c.id !== id)
          return OK()
        }
        if (r[3] === 'messages' && method === 'POST') {
          const b = await readBody(init)
          return sseResponse(id, b)
        }
      }
    }
  }

  // bilinmeyen /api → 404 (bileşenler !res.ok'u güvenle ele alıyor)
  return json({ error: `demo: bilinmeyen uç ${method} ${p}` }, 404)
}

// tip yardımcısı — incidentDetail için
type Incident = (typeof world.incidents)[number]
