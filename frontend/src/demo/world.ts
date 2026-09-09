// Demo katmanı — sentetik "dünya". Tüm sayfaların çektiği veriyi tek bir
// bellek-içi modelden türetir; tick() her saniye sayaçları yürütür, ara sıra
// akış / syslog / uyarı ekler. Backend YOK — bu, hub'ın gerçek veri yolunun
// (agent telemetrisi → store → API) yerini tutan tek taraflı bir taklittir.
//
// loadgen'deki (cmd/bazntms-loadgen) dağılımları aynalar: süreç adları, L7
// host'ları, uzak IP'ler, arayüz isimleri.

import { mulberry32, makePicker, ri, hashStr } from './rng'

const SEED = 20260909
const rnd = mulberry32(SEED)
const pick = makePicker(rnd)

export const now = () => Math.floor(Date.now() / 1000)

// ---- sabit havuzlar (loadgen ile hizalı) ----
const PROCESSES = ['chrome', 'firefox', 'slack', 'zoom', 'teams', 'code', 'node', 'python', 'curl', 'ssh', 'rsync', 'backup-agent', 'msmpeng', 'spotify', 'docker', 'kubelet']
const IFACES = ['eth0', 'eth1', 'en0', 'wlan0']
const SITES = ['merkez', 'ankara', 'izmir', 'dc-1', 'vpn'] as const

// uzak uçlar: [ip, ülke kodu, ülke adı, ASN org]
const REMOTES: [string, string, string, string][] = [
  ['140.82.121.4', 'US', 'ABD', 'AS36459 GitHub'],
  ['142.250.185.78', 'US', 'ABD', 'AS15169 Google'],
  ['13.107.42.14', 'IE', 'İrlanda', 'AS8075 Microsoft'],
  ['104.16.132.229', 'US', 'ABD', 'AS13335 Cloudflare'],
  ['52.95.116.115', 'DE', 'Almanya', 'AS16509 Amazon'],
  ['185.199.108.133', 'NL', 'Hollanda', 'AS54113 Fastly'],
  ['151.101.1.69', 'US', 'ABD', 'AS54113 Fastly'],
  ['193.140.100.10', 'TR', 'Türkiye', 'AS8517 ULAKNET'],
  ['195.175.39.49', 'TR', 'Türkiye', 'AS9121 Türk Telekom'],
  ['8.8.8.8', 'US', 'ABD', 'AS15169 Google'],
  ['1.1.1.1', 'AU', 'Avustralya', 'AS13335 Cloudflare'],
  ['77.88.55.60', 'RU', 'Rusya', 'AS13238 Yandex'],
  ['203.208.60.1', 'SG', 'Singapur', 'AS55967 Baidu'],
  ['13.228.0.10', 'SG', 'Singapur', 'AS16509 Amazon'],
  ['200.147.100.10', 'BR', 'Brezilya', 'AS22548 NIC.br'],
]

const GEO_CENTROIDS: Record<string, [number, number, string]> = {
  US: [38, -97, 'ABD'], IE: [53.4, -8, 'İrlanda'], DE: [51, 9, 'Almanya'], NL: [52.3, 5.3, 'Hollanda'],
  TR: [39, 35, 'Türkiye'], AU: [-25, 133, 'Avustralya'], RU: [61, 90, 'Rusya'], SG: [1.35, 103.8, 'Singapur'],
  BR: [-14, -51, 'Brezilya'], GB: [54, -2, 'Birleşik Krallık'], FR: [46, 2, 'Fransa'], JP: [36, 138, 'Japonya'],
}

// ---- tipler (frontend'in yerel tipleriyle alan-uyumlu) ----
interface Iface {
  name: string
  rxBytes: number
  txBytes: number
  rxPkts: number
  txPkts: number
  rxBps: number
  txBps: number
  pps: number
}
interface ConnSample {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
  pid: number
  process?: string
}
export interface DemoAgent {
  id: number
  name: string
  site: string
  first_seen: number
  last_seen: number
  version: string
  protocol_version: number
  remote_ip: string
  online: boolean
  attr_method: string
  attr_iface?: string
  attr_note?: string
  uplink_device_id?: number
  ifaces: Iface[]
  conns: number
  connSamples: ConnSample[]
  os: string
}
export interface DemoDevice {
  id: number
  name: string
  host: string
  kind: string
  site: string
  vendor: string
  snmp_version: number
  api_url: string
  api_verify_tls: boolean
  vdom: string
  poll_seconds: number
  enabled: boolean
  sys_name: string
  sys_descr: string
  added_at: number
  last_poll: number
  last_error: string
  ifCount: number
}
export interface Flow {
  ts: number
  device: string
  src: string
  dst: string
  src_port: number
  dst_port: number
  proto: string
  packets: number
  octets: number
}
export interface Syslog {
  id: number
  ts: number
  host: string
  source_ip: string
  severity: number
  tag: string
  message: string
}
export interface AlertEvt {
  id: number
  ts: number
  kind: string
  key: string
  message: string
  severity: 'info' | 'warn' | 'crit'
  state: 'firing' | 'ack' | 'resolved' | 'silenced'
  site: string
  count: number
  first_ts: number
  last_ts: number
  ack_by?: string
  note?: string
  group_id?: string
  ext_ref?: string
}
export interface Incident {
  id: number
  title: string
  severity: 'info' | 'warn' | 'crit'
  status: 'open' | 'investigating' | 'resolved' | 'closed'
  site: string
  agent_id: number
  correlation_reason: string
  correlation_key: string
  summary: string
  risk_score: number
  first_seen: number
  last_seen: number
  created_ts: number
  updated_ts: number
  ack_by?: string
  resolved_ts?: number
}

// ---- dünya durumu ----
export const world = {
  agents: [] as DemoAgent[],
  devices: [] as DemoDevice[],
  flows: [] as Flow[],
  syslog: [] as Syslog[],
  alerts: [] as AlertEvt[],
  incidents: [] as Incident[],
  silences: [] as Record<string, unknown>[],
  conversations: [] as any[],
  messages: {} as Record<number, any[]>,
  seq: { alert: 1000, syslog: 5000, flow: 1, conv: 1, msg: 1 },
  startedAt: now(),
}

// ---- kurulum ----
const AGENT_COUNT = 140
const OS_BY_SITE: Record<string, string[]> = {
  merkez: ['linux', 'linux', 'windows', 'darwin'],
  ankara: ['windows', 'windows', 'linux'],
  izmir: ['windows', 'linux'],
  'dc-1': ['linux', 'linux', 'linux'],
  vpn: ['darwin', 'windows', 'linux'],
}
const SITE_WEIGHT: [string, number][] = [['merkez', 58], ['ankara', 30], ['izmir', 20], ['dc-1', 22], ['vpn', 10]]

function attrFor(os: string): string {
  if (os === 'linux') return rnd() < 0.9 ? 'ebpf' : 'pcap'
  if (os === 'windows') return rnd() < 0.85 ? 'etw' : 'pcap'
  if (os === 'darwin') return rnd() < 0.15 ? 'off' : 'pcap'
  return 'pcap'
}

// attrIfaceFor / attrNoteFor — süreç-atıf teşhis alanları (yalnız gösterim).
function attrIfaceFor(os: string, method: string): string | undefined {
  if (method !== 'pcap') return undefined
  return os === 'windows' ? 'Ethernet' : os === 'darwin' ? 'en0' : 'eth0'
}
function attrNoteFor(method: string): string | undefined {
  return method === 'off' ? 'collect.method=off' : undefined
}

function buildDevices() {
  const base = now() - 86400 * 40
  const defs: Partial<DemoDevice>[] = [
    { name: 'fw-merkez-01', host: '10.0.0.1', kind: 'firewall', site: 'merkez', vendor: 'fortigate', api_url: 'https://10.0.0.1', vdom: 'root', ifCount: 12, sys_descr: 'FortiGate-100F v7.4.3' },
    { name: 'core-sw-merkez-01', host: '10.0.0.2', kind: 'switch', site: 'merkez', vendor: 'snmp', snmp_version: 3, ifCount: 48, sys_descr: 'Cisco Catalyst 9300, IOS-XE 17.12' },
    { name: 'core-sw-merkez-02', host: '10.0.0.3', kind: 'switch', site: 'merkez', vendor: 'snmp', snmp_version: 3, ifCount: 48, sys_descr: 'Cisco Catalyst 9300, IOS-XE 17.12' },
    { name: 'dist-sw-merkez-01', host: '10.0.1.10', kind: 'switch', site: 'merkez', vendor: 'snmp', snmp_version: 2, ifCount: 24, sys_descr: 'Aruba 2930F, ArubaOS 16.11' },
    { name: 'dist-sw-merkez-02', host: '10.0.1.11', kind: 'switch', site: 'merkez', vendor: 'snmp', snmp_version: 2, ifCount: 24, sys_descr: 'Aruba 2930F, ArubaOS 16.11' },
    { name: 'ap-merkez-01', host: '10.0.2.20', kind: 'ap', site: 'merkez', vendor: 'snmp', snmp_version: 2, ifCount: 4, sys_descr: 'Aruba AP-515' },
    { name: 'ap-merkez-02', host: '10.0.2.21', kind: 'ap', site: 'merkez', vendor: 'snmp', snmp_version: 2, ifCount: 4, sys_descr: 'Aruba AP-515' },
    { name: 'rtr-ankara-01', host: '10.10.0.1', kind: 'router', site: 'ankara', vendor: 'snmp', snmp_version: 3, ifCount: 8, sys_descr: 'Cisco ISR 4331, IOS-XE 17.9' },
    { name: 'sw-ankara-01', host: '10.10.0.2', kind: 'switch', site: 'ankara', vendor: 'snmp', snmp_version: 2, ifCount: 24, sys_descr: 'Aruba 2930F' },
    { name: 'ap-ankara-01', host: '10.10.2.20', kind: 'ap', site: 'ankara', vendor: 'snmp', snmp_version: 2, ifCount: 4, sys_descr: 'Aruba AP-505' },
    { name: 'rtr-izmir-01', host: '10.20.0.1', kind: 'router', site: 'izmir', vendor: 'snmp', snmp_version: 3, ifCount: 8, sys_descr: 'MikroTik CCR2004, RouterOS 7.14' },
    { name: 'sw-izmir-01', host: '10.20.0.2', kind: 'switch', site: 'izmir', vendor: 'snmp', snmp_version: 2, ifCount: 16, sys_descr: 'MikroTik CRS328' },
    { name: 'core-sw-dc1-01', host: '10.30.0.2', kind: 'switch', site: 'dc-1', vendor: 'snmp', snmp_version: 3, ifCount: 48, sys_descr: 'Arista 7050SX3, EOS 4.31' },
    { name: 'edge-sw-dc1-01', host: '10.30.0.3', kind: 'switch', site: 'dc-1', vendor: 'snmp', snmp_version: 3, ifCount: 32, sys_descr: 'Arista 7050SX3, EOS 4.31' },
  ]
  world.devices = defs.map((d, i) => ({
    id: i + 1,
    name: d.name!,
    host: d.host!,
    kind: d.kind!,
    site: d.site!,
    vendor: d.vendor!,
    snmp_version: d.snmp_version ?? 0,
    api_url: d.api_url ?? '',
    api_verify_tls: false,
    vdom: d.vdom ?? '',
    poll_seconds: 60,
    enabled: true,
    sys_name: d.name!,
    sys_descr: d.sys_descr ?? '',
    added_at: base + i * 3600,
    last_poll: now() - ri(rnd, 3, 55),
    last_error: i === 4 ? 'SNMP timeout (2/3 deneme)' : '',
    ifCount: d.ifCount ?? 8,
  }))
}

function pickSite(): string {
  const total = SITE_WEIGHT.reduce((s, [, w]) => s + w, 0)
  let r = rnd() * total
  for (const [s, w] of SITE_WEIGHT) {
    if (r < w) return s
    r -= w
  }
  return 'merkez'
}

function buildAgents() {
  const firstBase = now() - 86400 * 30
  const counters: Record<string, number> = {}
  for (let i = 0; i < AGENT_COUNT; i++) {
    const site = pickSite()
    counters[site] = (counters[site] ?? 0) + 1
    const os = pick(OS_BY_SITE[site] ?? ['linux'])
    const online = rnd() < 0.84
    const nIf = os === 'linux' && site === 'dc-1' ? 2 : 1 + (rnd() < 0.3 ? 1 : 0)
    const ifaces: Iface[] = []
    for (let j = 0; j < nIf; j++) {
      const rxBps = online ? 40_000 + rnd() * 1_400_000 : 0
      const txBps = online ? 12_000 + rnd() * 380_000 : 0
      ifaces.push({
        name: IFACES[j % IFACES.length],
        rxBytes: Math.floor(rxBps * ri(rnd, 3600, 86400)),
        txBytes: Math.floor(txBps * ri(rnd, 3600, 86400)),
        rxPkts: ri(rnd, 1e5, 9e6),
        txPkts: ri(rnd, 8e4, 6e6),
        rxBps,
        txBps,
        pps: online ? Math.floor(60 + rnd() * 2400) : 0,
      })
    }
    const id = i + 1
    const octet = 10 + Math.floor(i / 250)
    const attrForResolved = attrFor(os)
    const a: DemoAgent = {
      id,
      name: `agent-${site}-${String(counters[site]).padStart(3, '0')}`,
      site,
      first_seen: firstBase + i * 900,
      last_seen: online ? now() - ri(rnd, 1, 40) : now() - ri(rnd, 600, 86400 * 3),
      version: rnd() < 0.82 ? 'v1.3.0' : rnd() < 0.6 ? 'v1.2.0' : 'v1.1.0',
      protocol_version: 1,
      remote_ip: `10.${site === 'merkez' ? 0 : site === 'ankara' ? 10 : site === 'izmir' ? 20 : site === 'dc-1' ? 30 : 40}.${ri(rnd, 4, 250)}.${ri(rnd, 2, 250)}`,
      online,
      attr_method: attrForResolved,
      attr_iface: attrIfaceFor(os, attrForResolved),
      attr_note: attrNoteFor(attrForResolved),
      ifaces,
      conns: online ? ri(rnd, 4, 44) : 0,
      connSamples: [],
      os,
      uplink_device_id: undefined,
    }
    // uplink ata: aynı sahadaki switch/AP cihazına (%72)
    if (rnd() < 0.72) {
      const cands = world.devices.filter((d) => d.site === site && (d.kind === 'switch' || d.kind === 'ap'))
      if (cands.length) a.uplink_device_id = pick(cands).id
    }
    a.connSamples = mkConns(a, octet)
    world.agents.push(a)
  }
}

function mkConns(a: DemoAgent, octet: number): ConnSample[] {
  if (!a.online) return []
  const out: ConnSample[] = []
  const n = Math.min(a.conns, ri(rnd, 4, 18))
  for (let k = 0; k < n; k++) {
    const proc = pick(PROCESSES)
    const remote = pick(REMOTES)
    const listen = rnd() < 0.12
    out.push({
      proto: 'tcp',
      local_addr: `${a.remote_ip}:${ri(rnd, 40000, 61000)}`,
      remote_addr: listen ? undefined : `${remote[0]}:${pick([443, 443, 443, 80, 22, 5223])}`,
      status: listen ? 'LISTEN' : 'ESTABLISHED',
      pid: ri(rnd, 300, 9000),
      process: proc,
    })
  }
  void octet
  return out
}

function mkFlow(ts: number): Flow {
  const a = pick(world.agents.filter((x) => x.online)) ?? world.agents[0]
  const dev = pick(world.devices)
  const remote = pick(REMOTES)
  const inbound = rnd() < 0.5
  const local = a.remote_ip
  const bytes = Math.floor(400 + Math.pow(rnd(), 3) * 4_000_000)
  const pkts = Math.max(1, Math.floor(bytes / (400 + rnd() * 1000)))
  return {
    ts,
    device: dev.host,
    src: inbound ? remote[0] : local,
    dst: inbound ? local : remote[0],
    src_port: inbound ? pick([443, 80, 22]) : ri(rnd, 40000, 61000),
    dst_port: inbound ? ri(rnd, 40000, 61000) : pick([443, 443, 80, 53, 22, 8443]),
    proto: pick(['tcp', 'tcp', 'tcp', 'udp']),
    packets: pkts,
    octets: bytes,
  }
}

const SYS_MSGS: [number, string, string][] = [
  [6, '%LINK-3-UPDOWN', 'Interface GigabitEthernet1/0/12, changed state to up'],
  [5, '%SYS-5-CONFIG_I', 'Configured from console by admin on vty0'],
  [4, '%LINEPROTO-5-UPDOWN', 'Line protocol on Interface Gi1/0/24, changed state to down'],
  [3, '%OSPF-5-ADJCHG', 'Process 1, Nbr 10.0.0.3 on Gi1/0/1 from FULL to DOWN'],
  [6, '%DHCPD-6-LEASE', 'assigned 10.0.4.87 to 3c:22:fb:1a:9e:04'],
  [5, '%SEC_LOGIN-5-LOGIN_SUCCESS', 'Login Success [user: netops] from 10.0.9.5'],
  [4, '%SW_MATM-4-MACFLAP_NOTIF', 'Host aabb.cc00.1122 in vlan 20 is flapping between port Gi1/0/3 and Gi1/0/9'],
  [2, '%PLATFORM-1-CRASHED', 'Power supply 2 failed or removed'],
]

function mkSyslog(ts: number): Syslog {
  const dev = pick(world.devices)
  const [sev, tag, msg] = pick(SYS_MSGS)
  return {
    id: world.seq.syslog++,
    ts,
    host: dev.sys_name,
    source_ip: dev.host,
    severity: sev,
    tag,
    message: msg,
  }
}

const ALERT_TEMPLATES: [string, 'info' | 'warn' | 'crit', (site: string) => string][] = [
  ['bw', 'warn', (s) => `Bant genişliği zirvesi — ${s} uplink ↓ 940 Mb/s (eşik 800)`],
  ['proc', 'info', () => `Yeni süreç ağa çıktı — rclone · agent-dc-1-004`],
  ['target', 'warn', () => `Yeni hedefe hacimli trafik — 77.88.55.60 (RU) · 1.4 GB`],
  ['anomaly', 'warn', (s) => `DNS sorgu hızı beklenenin 4.2σ üstünde — saha ${s}`],
  ['iface_util', 'warn', () => `core-sw-merkez-01 Gi1/0/1 kullanımı %92 (5 dk sürdü)`],
  ['ioc', 'crit', () => `IOC eşleşmesi — agent-vpn-002 → bilinen C2 alan adı`],
  ['vpn_down', 'crit', () => `fw-merkez-01 · IPsec tüneli "ankara-branch" DOWN`],
  ['high_sessions', 'warn', () => `fw-merkez-01 oturum sayısı 512k (tavan 500k)`],
  ['sla_breach', 'crit', (s) => `SLA ihlali — ${s} agent uptime %96.1 (hedef %99)`],
  ['port', 'crit', () => `Şüpheli port — agent-ankara-011 → :4444 giden bağlantı`],
]

function mkAlert(ts: number, state: AlertEvt['state'] = 'firing'): AlertEvt {
  const [kind, severity, msgFn] = pick(ALERT_TEMPLATES)
  const site = pick([...SITES])
  const id = world.seq.alert++
  return {
    id,
    ts,
    kind,
    key: `${kind}:${site}:${ri(rnd, 1, 40)}`,
    message: msgFn(site),
    severity,
    state,
    site: rnd() < 0.6 ? site : '',
    count: ri(rnd, 1, 9),
    first_ts: ts - ri(rnd, 60, 6000),
    last_ts: ts,
    ack_by: state === 'ack' ? pick(['netops', 'admin']) : undefined,
    note: state === 'ack' ? 'İnceleniyor — saha ekibi bilgilendirildi.' : undefined,
    group_id: rnd() < 0.3 ? `grp-${ri(rnd, 100, 400)}` : undefined,
  }
}

function buildAlerts() {
  const t = now()
  for (let i = 0; i < 42; i++) {
    const age = Math.floor(Math.pow(rnd(), 2) * 86400 * 4)
    const st: AlertEvt['state'] = rnd() < 0.55 ? 'firing' : rnd() < 0.5 ? 'ack' : rnd() < 0.6 ? 'resolved' : 'silenced'
    world.alerts.push(mkAlert(t - age, st))
  }
  world.alerts.sort((a, b) => b.ts - a.ts)
}

function buildIncidents() {
  const t = now()
  const defs: [string, Incident['severity'], Incident['status'], string, string][] = [
    ['agent-vpn-002 · olası C2 iletişimi', 'crit', 'investigating', 'IOC eşleşmesi + yeni hedef + gece trafiği aynı 4 dk içinde', 'DNS beacon deseni + bilinen kötü ASN. Agent izole edilmesi önerilir.'],
    ['merkez uplink doygunluğu', 'warn', 'open', 'Bant genişliği zirvesi + arayüz kullanımı %90 + SLA ihlali korele', '18:00–19:30 arası yedekleme trafiği ile çakışma. Yedek penceresi kaydırıldı.'],
    ['ankara şubesi VPN kesintisi', 'crit', 'resolved', 'vpn_down + 12 agent offline + syslog OSPF adj down', 'ISP kaynaklı; 22 dk sonra otomatik toparlandı.'],
    ['dc-1 anormal DNS hacmi', 'warn', 'open', 'anomali (dns_qps 5.1σ) + yeni süreç (rclone)', 'Muhtemel toplu yedek/senkron işi. Sahibi doğrulanıyor.'],
    ['core-sw-merkez-02 MAC flap', 'info', 'closed', 'syslog MACFLAP + kısa süreli paket kaybı', 'Kablo değişimi sonrası çözüldü.'],
    ['izmir agent sürüm sürüklenmesi', 'info', 'open', '8 agent v1.1.0 · otomatik güncelleme başarısız', 'Npcap sürüm çakışması. Manuel MSI planlandı.'],
  ]
  world.incidents = defs.map((d, i) => {
    const first = t - ri(rnd, 3600, 86400 * 3)
    return {
      id: i + 1,
      title: d[0],
      severity: d[1],
      status: d[2],
      site: pick([...SITES]),
      agent_id: i === 0 ? 135 : i === 3 ? 122 : i === 5 ? 90 : 0,
      correlation_reason: d[3],
      correlation_key: `corr-${hashStr(d[0]).toString(16).slice(0, 8)}`,
      summary: d[4],
      risk_score: d[1] === 'crit' ? ri(rnd, 72, 94) : d[1] === 'warn' ? ri(rnd, 42, 68) : ri(rnd, 12, 34),
      first_seen: first,
      last_seen: d[2] === 'resolved' || d[2] === 'closed' ? first + ri(rnd, 1200, 6000) : t - ri(rnd, 60, 3000),
      created_ts: first,
      updated_ts: t - ri(rnd, 60, 4000),
      ack_by: d[2] !== 'open' ? pick(['netops', 'admin']) : undefined,
      resolved_ts: d[2] === 'resolved' || d[2] === 'closed' ? first + ri(rnd, 1200, 6000) : undefined,
    }
  })
}

function seedFlows() {
  const t = now()
  for (let i = 0; i < 260; i++) world.flows.push(mkFlow(t - ri(rnd, 1, 900)))
  world.flows.sort((a, b) => a.ts - b.ts)
}
function seedSyslog() {
  const t = now()
  for (let i = 0; i < 180; i++) world.syslog.push(mkSyslog(t - ri(rnd, 1, 7200)))
  world.syslog.sort((a, b) => a.ts - b.ts)
}

// ---- türetilmiş görünümler ----
export function agentRates(a: DemoAgent) {
  return a.ifaces.map((f) => ({
    name: f.name,
    rx_bps: f.rxBps,
    tx_bps: f.txBps,
    rx_bytes: f.rxBytes,
    tx_bytes: f.txBytes,
    pps: f.pps,
    rx_packets: f.rxPkts,
    tx_packets: f.txPkts,
    last_seen: a.last_seen,
  }))
}

export function agentJSON(a: DemoAgent) {
  return {
    id: a.id,
    name: a.name,
    site: a.site,
    first_seen: a.first_seen,
    last_seen: a.last_seen,
    version: a.version,
    protocol_version: a.protocol_version,
    remote_ip: a.remote_ip,
    online: a.online,
    rates: agentRates(a),
    conns: a.conns,
    uplink_device_id: a.uplink_device_id,
    attr_method: a.attr_method,
    attr_iface: a.attr_iface,
    attr_note: a.attr_note,
  }
}

export function fleetSummary() {
  let rx = 0, tx = 0, pps = 0, online = 0
  for (const a of world.agents) {
    if (!a.online) continue
    online++
    for (const f of a.ifaces) {
      rx += f.rxBps * 8
      tx += f.txBps * 8
      pps += f.pps
    }
  }
  const flowsLastMin = world.flows.filter((f) => f.ts > now() - 60).length + 40
  return {
    agents_total: world.agents.length,
    agents_online: online,
    rx_bps: Math.round(rx),
    tx_bps: Math.round(tx),
    pps: Math.round(pps),
    flows_per_min: flowsLastMin,
  }
}

export function legacyAlerts(limit = 20) {
  return world.alerts
    .slice()
    .sort((a, b) => b.ts - a.ts)
    .slice(0, limit)
    .map((e) => ({ id: e.id, ts: e.ts, kind: e.kind, key: e.key, message: e.message }))
}

export function geoRows(minutes: number) {
  const bucket = new Map<string, { bytes: number; sessions: number }>()
  const horizon = now() - minutes * 60
  for (const f of world.flows) {
    if (f.ts < horizon) continue
    const r = REMOTES.find((x) => x[0] === f.src || x[0] === f.dst)
    if (!r) continue
    const cur = bucket.get(r[1]) ?? { bytes: 0, sessions: 0 }
    cur.bytes += f.octets
    cur.sessions += 1
    bucket.set(r[1], cur)
  }
  // taban dolgu — her zaman dolu görünsün
  for (const cc of ['US', 'TR', 'DE', 'IE', 'NL', 'SG']) {
    if (!bucket.has(cc)) bucket.set(cc, { bytes: 5e6 + rnd() * 4e7, sessions: ri(rnd, 20, 300) })
  }
  return [...bucket.entries()]
    .map(([cc, v]) => {
      const c = GEO_CENTROIDS[cc] ?? [0, 0, cc]
      return { country: cc, name: c[2], lat: c[0], lon: c[1], bytes: Math.round(v.bytes), sessions: v.sessions }
    })
    .sort((a, b) => b.bytes - a.bytes)
}

export function topologyGraph() {
  const devs = world.devices.map((d) => ({
    id: d.id,
    name: d.name,
    host: d.host,
    kind: d.kind,
    sys_name: d.sys_name,
    online: d.enabled && !d.last_error,
  }))
  const agents = world.agents.map((a) => ({ id: a.id, name: a.name, site: a.site, online: a.online }))
  const links: any[] = []
  let lid = 1
  const dByName = (n: string) => world.devices.find((d) => d.name === n)!
  const link = (srcName: string, peerName: string, kind: string, port: string) => {
    const s = dByName(srcName)
    const p = dByName(peerName)
    links.push({
      id: lid++,
      ts: now() - ri(rnd, 60, 3600),
      kind,
      source_type: 'device',
      source_id: s.id,
      source_name: s.name,
      local_port: port,
      peer_type: 'device',
      peer_id: p.id,
      peer_name: `${p.sys_name} (${p.host})`,
      peer_ip: p.host,
      confidence: 'discovered',
      telemetry: {
        if_name: port,
        oper_status: 1,
        speed_bps: 1e10,
        rx_bps: 2e8 + rnd() * 6e8,
        tx_bps: 1e8 + rnd() * 3e8,
        rx_util_pct: 20 + rnd() * 70,
        tx_util_pct: 15 + rnd() * 55,
        class: 'uplink',
        errors: ri(rnd, 0, 4),
        discards: ri(rnd, 0, 12),
      },
    })
  }
  link('core-sw-merkez-01', 'fw-merkez-01', 'lldp', 'Gi1/0/1')
  link('core-sw-merkez-02', 'fw-merkez-01', 'lldp', 'Gi1/0/1')
  link('core-sw-merkez-01', 'core-sw-merkez-02', 'lldp', 'Te1/1/1')
  link('dist-sw-merkez-01', 'core-sw-merkez-01', 'lldp', 'Gi1/0/48')
  link('dist-sw-merkez-02', 'core-sw-merkez-02', 'lldp', 'Gi1/0/48')
  link('ap-merkez-01', 'dist-sw-merkez-01', 'cdp', 'eth0')
  link('ap-merkez-02', 'dist-sw-merkez-01', 'cdp', 'eth0')
  link('sw-ankara-01', 'rtr-ankara-01', 'cdp', 'Gi0/1')
  link('ap-ankara-01', 'sw-ankara-01', 'cdp', 'eth0')
  link('sw-izmir-01', 'rtr-izmir-01', 'lldp', 'ether1')
  link('edge-sw-dc1-01', 'core-sw-dc1-01', 'lldp', 'Et49')
  // agent subnet keşfi (çözümlenmemiş komşular)
  for (const a of world.agents.filter((x) => x.online).slice(0, 6)) {
    const p = a.remote_ip.split('.').slice(0, 3).join('.') + '.0/24'
    links.push({
      id: lid++, ts: now() - ri(rnd, 60, 1800), kind: 'subnet',
      source_type: 'agent', source_id: a.id, source_name: a.name,
      local_port: '', peer_type: 'subnet', peer_id: 0, peer_name: '', peer_ip: p, confidence: 'inferred',
    })
  }
  return { generated_at: now(), devices: devs, agents, links }
}

// ---- tick: canlılık ----
export function tick() {
  const t = now()
  for (const a of world.agents) {
    if (!a.online) {
      if (Math.random() < 0.002) {
        a.online = true
        a.last_seen = t
        a.ifaces.forEach((f) => {
          f.rxBps = 50_000 + Math.random() * 900_000
          f.txBps = 15_000 + Math.random() * 250_000
          f.pps = 40 + Math.random() * 1800
        })
        a.conns = 4 + Math.floor(Math.random() * 30)
        a.connSamples = mkConns(a, 10)
      }
      continue
    }
    if (Math.random() < 0.0015) {
      a.online = false
      a.ifaces.forEach((f) => { f.rxBps = 0; f.txBps = 0; f.pps = 0 })
      a.conns = 0
      a.connSamples = []
      continue
    }
    a.last_seen = t
    for (const f of a.ifaces) {
      const burst = Math.random() < 0.04 ? 2.2 + Math.random() * 3 : 1
      const target = (60_000 + Math.random() * 1_500_000) * burst
      f.rxBps += (target - f.rxBps) * 0.25
      f.txBps += (f.rxBps * (0.18 + Math.random() * 0.2) - f.txBps) * 0.25
      f.pps += ((80 + Math.random() * 2600) * burst - f.pps) * 0.25
      f.rxBytes += Math.round(f.rxBps)
      f.txBytes += Math.round(f.txBps)
      f.rxPkts += Math.round(f.pps * 0.7)
      f.txPkts += Math.round(f.pps * 0.3)
    }
    if (Math.random() < 0.05) {
      a.conns = Math.max(2, a.conns + (Math.random() < 0.5 ? -1 : 1))
      a.connSamples = mkConns(a, 10)
    }
  }

  // akış / syslog akışı
  const nFlows = 1 + Math.floor(Math.random() * 5)
  for (let i = 0; i < nFlows; i++) world.flows.push(mkFlow(t))
  if (world.flows.length > 600) world.flows.splice(0, world.flows.length - 600)
  if (Math.random() < 0.4) {
    world.syslog.push(mkSyslog(t))
    if (world.syslog.length > 400) world.syslog.splice(0, world.syslog.length - 400)
  }

  // ara sıra yeni uyarı
  if (Math.random() < 0.06) {
    world.alerts.unshift(mkAlert(t))
    if (world.alerts.length > 120) world.alerts.pop()
  }
}

function seedConversations() {
  const t = now()
  const nightly = {
    id: world.seq.conv++,
    title: 'Gecelik filo analizi',
    created_by: 'scheduler',
    site: '',
    scope_kind: 'fleet',
    scope_ref: '',
    provider_id: 1,
    model: 'llama3.1:8b',
    source: 'nightly',
    created_ts: t - 3600 * 9,
    updated_ts: t - 3600 * 9,
    archived: false,
  }
  const triage = {
    id: world.seq.conv++,
    title: 'agent-vpn-002 · olası C2 iletişimi',
    created_by: 'incident-engine',
    site: 'vpn',
    scope_kind: 'incident',
    scope_ref: '1',
    provider_id: 1,
    model: 'llama3.1:8b',
    source: 'triage',
    created_ts: t - 3600 * 2,
    updated_ts: t - 3600 * 2,
    archived: false,
  }
  world.conversations.push(nightly, triage)
  world.messages[nightly.id] = [
    { id: world.seq.msg++, role: 'system', content: 'Filo bağlamı yüklendi (140 agent, 14 cihaz).', tokens_in: 0, tokens_out: 0, created_ts: nightly.created_ts },
    { id: world.seq.msg++, role: 'assistant', tokens_in: 820, tokens_out: 240, created_ts: nightly.created_ts, content: '## Gecelik Özet (demo)\n\nFilo stabil. 3 izlenmesi gereken nokta: **izmir sürüm sürüklenmesi**, **merkez uplink doygunluğu**, **dc-1 DNS hacmi (rclone)**. Kritik uyarı yok; 1 açık olay inceleme aşamasında.' },
  ]
  world.messages[triage.id] = [
    { id: world.seq.msg++, role: 'system', content: 'Olay #1 bağlamı + kanıt zaman çizelgesi yüklendi.', tokens_in: 0, tokens_out: 0, created_ts: triage.created_ts },
    { id: world.seq.msg++, role: 'assistant', tokens_in: 1100, tokens_out: 300, created_ts: triage.created_ts, content: '## Triyaj (demo)\n\n3 sinyal 4 dk içinde çakıştı (IOC + yeni hedef + gece trafiği). Risk 86/100. **Öneri:** agent-vpn-002 izole edilmeli, süreç ağacı ve bağlantı envanteri toplanmalı, hedef alan adı threat-intel ile doğrulanmalı.' },
  ]
}

// ---- init ----
let inited = false
export function initWorld() {
  if (inited) return
  inited = true
  buildDevices()
  buildAgents()
  seedFlows()
  seedSyslog()
  buildAlerts()
  buildIncidents()
  seedConversations()
}
