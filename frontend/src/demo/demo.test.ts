import { describe, it, expect, beforeAll } from 'vitest'
import { initWorld, tick, world, fleetSummary } from './world'
import { demoHandle } from './http'

const u = (path: string) => new URL(path, 'http://localhost')
async function get(path: string) {
  const res = await demoHandle('GET', u(path))
  expect(res, path).not.toBeNull()
  expect(res!.status, path).toBe(200)
  return res!.json()
}

beforeAll(() => initWorld())

describe('demo world', () => {
  it('kurulumda dolu bir filo üretir', () => {
    expect(world.agents.length).toBe(140)
    expect(world.agents.filter((a) => a.online).length).toBeGreaterThan(80)
    expect(world.devices.length).toBeGreaterThan(10)
    expect(world.incidents.length).toBeGreaterThan(0)
  })

  it('tick sayaçları yürütür, listeleri sınırlar', () => {
    for (let i = 0; i < 400; i++) tick()
    expect(world.flows.length).toBeLessThanOrEqual(600)
    expect(world.syslog.length).toBeLessThanOrEqual(400)
    const f = fleetSummary()
    expect(f.agents_total).toBe(140)
    expect(f.rx_bps).toBeGreaterThan(0)
  })
})

describe('demo http router — çekirdek uçlar dolu yanıt verir', () => {
  it('auth kapalı', async () => {
    expect(await get('/api/auth/status')).toMatchObject({ required: false })
  })

  it('agent listesi + detay', async () => {
    const list = await get('/api/v1/agents')
    expect(Array.isArray(list)).toBe(true)
    expect(list.length).toBe(140)
    const detail = await get(`/api/v1/agents/${list[0].id}`)
    expect(detail).toHaveProperty('agent')
    expect(detail).toHaveProperty('connections')
  })

  it.each([
    '/api/v1/devices',
    '/api/v1/flows?minutes=15&limit=20',
    '/api/v1/syslog?limit=20',
    '/api/v1/topology',
    '/api/v1/geo?minutes=60',
    '/api/v1/health',
    '/api/v1/processes?minutes=60&limit=20',
    '/api/v1/l7?minutes=60&limit=30',
    '/api/v1/dns?minutes=60&limit=30',
    '/api/v1/events?since_min=180&limit=50',
    '/api/v1/anomaly/baseline?dim=fleet&metric=bps',
    '/api/v1/anomaly/active',
    '/api/v1/incidents?limit=100',
    '/api/v1/alerts/events?limit=100',
    '/api/v1/isms/summary',
    '/api/v1/isms/soa',
    '/api/v1/compliance/status',
    '/api/v1/sla/targets',
    '/api/v1/reports/schedules',
    '/api/v1/users',
    '/api/v1/ai/status',
    '/api/v1/ai/conversations',
    '/api/v1/audit?limit=100',
  ])('%s → 200 + JSON', async (path) => {
    const body = await get(path)
    expect(body).toBeDefined()
  })

  it('süreç adları benzersiz (TuiTable getKey çakışmaz)', async () => {
    const rows: { process: string }[] = await get('/api/v1/processes?minutes=60&limit=20')
    const names = rows.map((r) => r.process)
    expect(new Set(names).size).toBe(names.length)
  })

  it('mutasyonlar {ok:true} döner ve uyarı durumunu değiştirir', async () => {
    const before = world.alerts.find((e) => e.state === 'firing')!
    const res = await demoHandle('POST', u(`/api/v1/alerts/events/${before.id}/ack`), {
      body: JSON.stringify({ note: 'test' }),
    })
    expect(await res!.json()).toMatchObject({ ok: true })
    expect(world.alerts.find((e) => e.id === before.id)!.state).toBe('ack')
  })

  it('bilinmeyen /api → 404', async () => {
    const res = await demoHandle('GET', u('/api/v1/bilinmeyen'))
    expect(res!.status).toBe(404)
  })

  it('AI mesajı SSE akışı döndürür', async () => {
    const created = await demoHandle('POST', u('/api/v1/ai/conversations'), { body: JSON.stringify({ scope_kind: 'fleet' }) })
    const { id } = await created!.json()
    const res = await demoHandle('POST', u(`/api/v1/ai/conversations/${id}/messages`), { body: JSON.stringify({ preset: 'fleet_health' }) })
    expect(res!.headers.get('content-type')).toContain('text/event-stream')
    const text = await res!.text()
    expect(text).toContain('data:')
    expect(text).toContain('"done":true')
  })
})
