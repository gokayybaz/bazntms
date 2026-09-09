// Demo katmanı — sorgu-anında türetilen görünümler (süreç/L7/DNS toplamları,
// olay akışı, anomali baseline, NetFlow konuşmaları, agent/cihaz detayları).
// Sonuçlar (dim/metric/pencere) + zaman tohumuyla deterministik.

import { mulberry32, makePicker, ri, hashStr } from './rng'
import { world, now, type DemoAgent } from './world'

const PROCS = ['chrome', 'firefox', 'slack', 'zoom', 'teams', 'code', 'node', 'python', 'curl', 'ssh', 'rsync', 'backup-agent', 'msmpeng', 'spotify', 'docker', 'kubelet', 'rclone']
const L7 = [
  ['api.github.com', 'tls', 'code'], ['www.google.com', 'tls', 'chrome'], ['slack.com', 'tls', 'slack'],
  ['zoom.us', 'tls', 'zoom'], ['cdn.jsdelivr.net', 'tls', 'chrome'], ['s3.eu-central-1.amazonaws.com', 'tls', 'backup-agent'],
  ['login.microsoftonline.com', 'tls', 'teams'], ['update.microsoft.com', 'http', 'msmpeng'], ['registry.npmjs.org', 'tls', 'node'],
  ['grafana.bazntms.local', 'http', 'chrome'], ['teams.microsoft.com', 'tls', 'teams'], ['raw.githubusercontent.com', 'tls', 'curl'],
  ['pypi.org', 'tls', 'python'], ['mail.kurum.local', 'tls', 'chrome'],
] as const
const DOMAINS = [...L7.map((r) => r[0]), 'dns.google', 'time.cloudflare.com', 'ocsp.digicert.com', '_ldap._tcp.kurum.local']

export function processes(minutes: number, limit: number, agentId?: number) {
  const r = mulberry32(hashStr(`proc${minutes}${agentId ?? ''}`) ^ Math.floor(Date.now() / 60000))
  const scale = minutes / 15
  const n = Math.min(agentId ? 12 : limit, PROCS.length)
  return Array.from({ length: n }, (_, i) => {
    const bin = 2_000_000 + Math.pow(r(), 2) * 380_000_000 * scale
    const bout = bin * (0.15 + r() * 0.4)
    return {
      process: PROCS[i % PROCS.length],
      bytes_in: Math.round(bin),
      bytes_out: Math.round(bout),
      total: Math.round(bin + bout),
      agent_count: agentId ? 1 : ri(r, 1, 46),
    }
  }).sort((a, b) => b.total - a.total)
}

export function l7(minutes: number, limit: number, agentId?: number) {
  const r = mulberry32(hashStr(`l7${minutes}${agentId ?? ''}`) ^ Math.floor(Date.now() / 60000))
  const scale = minutes / 15
  return L7.slice(0, Math.min(limit, L7.length)).map(([host, kind, proc]) => ({
    host, kind, process: proc,
    bytes: Math.round((80_000 + Math.pow(r(), 2) * 60_000_000) * scale),
    hits: Math.round((8 + r() * 220) * scale),
    agent_count: agentId ? 1 : ri(r, 1, 40),
  })).sort((a, b) => b.hits - a.hits)
}

export function dns(minutes: number, limit: number, agentId?: number) {
  const r = mulberry32(hashStr(`dns${minutes}${agentId ?? ''}`) ^ Math.floor(Date.now() / 60000))
  const scale = minutes / 15
  return DOMAINS.slice(0, Math.min(limit, DOMAINS.length)).map((domain, i) => {
    const q = Math.round((2 + r() * 60) * scale)
    return {
      domain,
      process: PROCS[(i + 1) % PROCS.length],
      queries: q,
      responses: Math.max(0, q - ri(r, 0, 2)),
      agent_count: agentId ? 1 : ri(r, 1, 38),
    }
  }).sort((a, b) => b.queries + b.responses - (a.queries + a.responses))
}

const EVENT_TYPES = ['dns.query', 'tls.sni_observed', 'http.host_observed', 'netflow.flow', 'syslog.received', 'connection.seen']
export function events(sinceMin: number, limit: number, before?: number, types?: string[], agentId?: number) {
  const online = world.agents.filter((a) => a.online)
  const end = before || now()
  const rows: any[] = []
  let ts = end - ri(mulberry32(end), 1, 20)
  for (let i = 0; i < limit; i++) {
    const r = mulberry32(hashStr(`ev${ts}${i}`))
    const type = types && types.length ? types[Math.floor(r() * types.length)] : EVENT_TYPES[Math.floor(r() * EVENT_TYPES.length)]
    const a = agentId ? world.agents.find((x) => x.id === agentId) : online[Math.floor(r() * online.length)]
    const dev = world.devices[Math.floor(r() * world.devices.length)]
    const dom = DOMAINS[Math.floor(r() * DOMAINS.length)]
    const proc = PROCS[Math.floor(r() * PROCS.length)]
    const base: any = { type, source: type.startsWith('syslog') || type.startsWith('netflow') ? 'device' : 'agent', ts }
    if (type === 'syslog.received' || type === 'netflow.flow') {
      base.device = dev.sys_name
      if (type === 'netflow.flow') { base.dst_ip = '140.82.121.4'; base.dst_port = 443; base.proto = 'tcp'; base.src_ip = a?.remote_ip }
      if (type === 'syslog.received') base.severity = ['info', 'notice', 'warning'][Math.floor(r() * 3)]
    } else {
      base.agent_id = a?.id
      base.process = proc
      base.pid = ri(r, 300, 9000)
      if (type === 'dns.query') base.domain = dom
      else if (type.includes('sni') || type.includes('host')) { base.domain = dom; base.proto = type.includes('sni') ? 'tls' : 'http' }
      else { base.dst_ip = '104.16.132.229'; base.dst_port = 443; base.proto = 'tcp' }
      base.count = ri(r, 1, 12)
    }
    rows.push(base)
    ts -= ri(r, 3, 90)
    if (ts < end - sinceMin * 60) break
  }
  const next = rows.length >= limit ? ts : 0
  return { events: rows, next: next > now() - sinceMin * 60 ? next : 0 }
}

export function anomalyBaseline(dim: string, metric: string) {
  const seasonality = 'weekday'
  const r = mulberry32(hashStr(`base${dim}${metric}`))
  const N = 48
  const rows = Array.from({ length: N }, (_, b) => {
    const workHour = (b % 24) >= 8 && (b % 24) <= 19
    const weekend = b >= 24
    let mean: number
    if (metric === 'dns_qps') mean = (weekend ? 6 : workHour ? 42 : 12) * (0.8 + r() * 0.5)
    else if (metric === 'l7_qps') mean = (weekend ? 8 : workHour ? 55 : 15) * (0.8 + r() * 0.5)
    else if (metric === 'proc_bps') mean = (weekend ? 3e6 : workHour ? 4.2e7 : 8e6) * (0.8 + r() * 0.5)
    else mean = (weekend ? 6e7 : workHour ? 7.5e8 : 1.4e8) * (0.8 + r() * 0.5)
    const stdev = mean * (0.12 + r() * 0.1)
    const n = ri(r, 40, 180)
    return { bucket: b, n, mean, m2: stdev * stdev * n }
  })
  return { dim, metric, seasonality, rows }
}

export function anomalyActive() {
  const bucketNow = (() => {
    const d = new Date()
    const wd = d.getDay()
    return wd === 0 || wd === 6 ? 24 + d.getHours() : d.getHours()
  })()
  const defs = [
    { metric: 'dns_qps', dim: 'fleet', key: 'fleet', scope: 'Tüm filo', mean: 42, std: 6, cur: 71, z: 4.8, n: 120 },
    { metric: 'bps', dim: 'fleet', key: 'fleet', scope: 'Tüm filo', mean: 7.5e8, std: 9e7, cur: 1.08e9, z: 3.7, n: 140 },
    { metric: 'proc_bps', dim: 'local', key: 'rclone', scope: 'süreç · rclone (agent-dc-1-004)', mean: 4e6, std: 1.1e6, cur: 3.2e7, z: 6.1, n: 64 },
  ]
  return { deviations: defs.map((d) => ({ ...d, bucket: bucketNow })) }
}

const WIN_SEC: Record<string, number> = { '15m': 900, '1h': 3600, '6h': 21600, '24h': 86400 }
export function flowConversations(win: string, by: string, sort: string, limit: number) {
  const horizon = now() - (WIN_SEC[win] ?? 900)
  const agg = new Map<string, any>()
  for (const f of world.flows) {
    if (f.ts < horizon) continue
    const key = by === '5tuple' ? `${f.src}|${f.dst}|${f.src_port}|${f.dst_port}|${f.proto}` : [f.src, f.dst].sort().join('|')
    const cur = agg.get(key) ?? { src: f.src, dst: f.dst, src_port: by === '5tuple' ? f.src_port : undefined, dst_port: by === '5tuple' ? f.dst_port : undefined, proto: by === '5tuple' ? f.proto : (f.proto), flows: 0, packets: 0, octets: 0, first_seen: f.ts, last_seen: f.ts }
    cur.flows++
    cur.packets += f.packets
    cur.octets += f.octets
    cur.first_seen = Math.min(cur.first_seen, f.ts)
    cur.last_seen = Math.max(cur.last_seen, f.ts)
    agg.set(key, cur)
  }
  const rows = [...agg.values()]
  const k = sort === 'last_seen' ? 'last_seen' : sort
  rows.sort((a, b) => (b[k] ?? 0) - (a[k] ?? 0))
  return rows.slice(0, limit)
}

export function flowConversationDrill(win: string, src: string, dst: string, proto?: string) {
  const horizon = now() - (WIN_SEC[win] ?? 900)
  const flows = world.flows
    .filter((f) => f.ts >= horizon && ((f.src === src && f.dst === dst) || (f.src === dst && f.dst === src)) && (!proto || f.proto === proto))
    .slice(-40)
    .reverse()
  const online = world.agents.filter((a) => a.online)
  const actors = online.slice(0, 3).map((a) => ({ agent_id: a.id, agent_name: a.name, process: PROCS[a.id % PROCS.length], ip: src }))
  return { flows, actors, src_info: undefined, dst_info: undefined, src_rep: undefined, dst_rep: undefined }
}

export function agentHistory(minutes: number) {
  const N = Math.min(minutes, 180)
  const step = Math.max(60, Math.floor((minutes * 60) / N))
  const end = now()
  const r = mulberry32(hashStr(`hist${minutes}`))
  let level = 0.3
  return Array.from({ length: N }, (_, i) => {
    const ts = end - (N - 1 - i) * step
    const hour = new Date(ts * 1000).getHours()
    const workBoost = hour >= 8 && hour <= 19 ? 1.6 : 0.6
    const burst = r() < 0.08 ? 2.5 : 1
    level += ((0.35 * workBoost * burst) - level) * 0.3 + (r() - 0.5) * 0.1
    level = Math.max(0.04, Math.min(1.4, level))
    const inB = Math.round(level * 9_000_000 * step / 60)
    return { ts, in: inB, out: Math.round(inB * (0.2 + r() * 0.15)), local: Math.round(inB * 0.05), packets: Math.round(inB / 800) }
  })
}

export function processDetail(a: DemoAgent, name: string, minutes: number) {
  const r = mulberry32(hashStr(`pd${a.id}${name}${minutes}`))
  const scale = minutes / 15
  const bin = Math.round((3_000_000 + r() * 120_000_000) * scale)
  const bout = Math.round(bin * (0.2 + r() * 0.3))
  const t = now()
  const remotes = Array.from({ length: ri(r, 3, 9) }, () => {
    const rip = `${ri(r, 20, 210)}.${ri(r, 1, 250)}.${ri(r, 1, 250)}.${ri(r, 1, 250)}`
    return {
      remote_ip: rip, port: makePicker(r)([443, 443, 80, 22, 8443]), proto: 'tcp',
      bytes_in: ri(r, 1e4, 4e7), bytes_out: ri(r, 5e3, 1e7),
      first_seen: t - ri(r, 600, 7200), last_seen: t - ri(r, 5, 300), conns: ri(r, 1, 12),
      country: makePicker(r)(['US', 'DE', 'IE', 'TR', 'NL']), asn: `AS${ri(r, 1000, 60000)}`, org: 'Örnek Sağlayıcı',
    }
  })
  const app = L7.slice(0, ri(r, 2, 6)).map(([host, kind]) => ({
    type: kind, host, observations: ri(r, 3, 90), bytes: ri(r, 5e3, 2e6),
    first_seen: t - ri(r, 600, 7200), last_seen: t - ri(r, 5, 400),
  }))
  const timeline = [
    { ts: t - ri(r, 5000, 9000), event: 'process.first_seen', target: name, detail: `pid ${ri(r, 300, 9000)}` },
    { ts: t - ri(r, 2000, 4000), event: 'dns.query', target: makePicker(r)(DOMAINS) },
    { ts: t - ri(r, 800, 1800), event: 'tls.sni', target: makePicker(r)(DOMAINS) },
    { ts: t - ri(r, 60, 600), event: 'traffic.spike', target: 'RX 32 Mb/s' },
  ]
  return {
    process: {
      process: name, pids: [ri(r, 300, 9000), ri(r, 300, 9000)],
      first_seen: t - ri(r, 6000, 12000), last_seen: t - ri(r, 5, 120),
      bytes_in: bin, bytes_out: bout, total: bin + bout,
      rx_bps: Math.round(bin / (minutes * 60)), tx_bps: Math.round(bout / (minutes * 60)),
    },
    remotes,
    connections: a.connSamples.filter((c) => c.process === name).length
      ? a.connSamples.filter((c) => c.process === name)
      : a.connSamples.slice(0, 4).map((c) => ({ ...c, process: name })),
    app_visibility: app,
    timeline,
  }
}

export function deviceInterfaces(deviceId: number) {
  const d = world.devices.find((x) => x.id === deviceId)
  if (!d) return []
  const r = mulberry32(hashStr(`if${deviceId}`) ^ Math.floor(Date.now() / 30000))
  const n = Math.min(d.ifCount, 16)
  return Array.from({ length: n }, (_, i) => {
    const up = r() > 0.12
    const speed = i < 2 ? 1e10 : 1e9
    const rxBps = up ? (i < 2 ? 8e7 + r() * 7e8 : 2e6 + r() * 9e7) : 0
    const txBps = up ? rxBps * (0.2 + r() * 0.5) : 0
    const rxUtil = up ? Math.min(99, (rxBps * 8 / speed) * 100) : -1
    return {
      if_index: i + 1,
      name: i < 2 ? `Te1/1/${i + 1}` : `Gi1/0/${i + 1}`,
      alias: i === 0 ? 'uplink-fw' : '',
      speed,
      speed_bps: speed,
      speed_source: 'ifHighSpeed',
      oper_status: up ? 1 : 2,
      rx_bps: Math.round(rxBps),
      tx_bps: Math.round(txBps),
      rx_bytes: ri(r, 1e9, 9e11),
      tx_bytes: ri(r, 1e9, 5e11),
      in_errors: r() < 0.15 ? ri(r, 1, 40) : 0,
      out_errors: r() < 0.1 ? ri(r, 1, 20) : 0,
      in_discards: r() < 0.2 ? ri(r, 1, 120) : 0,
      out_discards: r() < 0.15 ? ri(r, 1, 60) : 0,
      rx_util_pct: rxUtil < 0 ? -1 : Math.round(rxUtil * 10) / 10,
      tx_util_pct: rxUtil < 0 ? -1 : Math.round(rxUtil * 0.55 * 10) / 10,
      class: i < 2 ? 'uplink' : 'access',
    }
  })
}

// ---- FortiGate (fw-merkez-01, id 1) ----
export function fortiResources(minutes: number) {
  const N = Math.min(minutes, 120)
  const step = Math.floor((minutes * 60) / N)
  const end = now()
  const r = mulberry32(hashStr(`fr${minutes}`) ^ Math.floor(Date.now() / 60000))
  let cpu = 22, mem = 61, sess = 320000
  return Array.from({ length: N }, (_, i) => {
    cpu = Math.max(6, Math.min(95, cpu + (r() - 0.45) * 12))
    mem = Math.max(40, Math.min(92, mem + (r() - 0.5) * 3))
    sess = Math.max(120000, Math.min(520000, sess + (r() - 0.5) * 40000))
    return { ts: end - (N - 1 - i) * step, cpu_pct: Math.round(cpu), mem_pct: Math.round(mem), disk_pct: 47, sessions: Math.round(sess) }
  })
}
export function fortiVpn() {
  const t = now()
  // rx_bytes/tx_bytes FortiPanel'de anlık hız olarak gösteriliyor (bayt/sn) —
  // makul değerler ver (ör. ~40 Mbit/s → 5e6 bayt/sn).
  return [
    { vdom: 'root', kind: 'ipsec', name: 'ankara-branch', peer: '203.0.113.7', status: 'up', uptime: 86400 * 12 + 3600, rx_bytes: 5.1e6, tx_bytes: 1.9e6, ts: t - 20 },
    { vdom: 'root', kind: 'ipsec', name: 'izmir-branch', peer: '203.0.113.9', status: 'up', uptime: 86400 * 40, rx_bytes: 8.7e6, tx_bytes: 3.4e6, ts: t - 25 },
    { vdom: 'root', kind: 'ssl', name: 'remote-users', peer: '', status: 'up', uptime: 3600 * 6, rx_bytes: 1.2e6, tx_bytes: 4.4e5, ts: t - 15 },
    { vdom: 'root', kind: 'ipsec', name: 'dc1-transit', peer: '198.51.100.4', status: 'down', uptime: 0, rx_bytes: 0, tx_bytes: 0, ts: t - 40 },
  ]
}
export function fortiSdwan(minutes: number) {
  const r = mulberry32(hashStr(`sd${minutes}`) ^ Math.floor(Date.now() / 60000))
  const rows: any[] = []
  const members = ['wan1', 'wan2']
  const checks = ['google-ping', 'kurum-dns']
  const t = now()
  for (let s = 0; s < 6; s++) {
    for (const m of members) for (const hc of checks) {
      const bad = m === 'wan2' && r() < 0.4
      rows.push({
        ts: t - s * 300, vdom: 'root', member: m, health_check: hc,
        latency_ms: bad ? 120 + r() * 90 : 12 + r() * 25,
        jitter_ms: bad ? 20 + r() * 40 : 1 + r() * 5,
        packet_loss_pct: bad ? r() * 6 : 0,
        state: bad ? 'down' : 'up',
      })
    }
  }
  return rows
}
export function fortiPolicies() {
  const names = ['LAN→Internet', 'Guest→Internet', 'Branch-VPN→LAN', 'DMZ→DB', 'Block-Tor', 'Admin-Access']
  const r = mulberry32(hashStr('pol') ^ Math.floor(Date.now() / 60000))
  return names.map((name, i) => ({
    vdom: 'root', policy_id: 10 + i, name,
    action: name.startsWith('Block') ? 'deny' : 'accept',
    hits: ri(r, 200, 90000), bytes: ri(r, 1e6, 4e10),
  })).sort((a, b) => b.hits - a.hits)
}
