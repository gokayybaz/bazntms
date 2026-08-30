export interface NetInterface {
  name: string
  hw_addr?: string
  mtu: number
  up: boolean
  loopback: boolean
  can_sniff: boolean
  addresses?: string[]
  rx_bytes: number
  tx_bytes: number
  rx_packets: number
  tx_packets: number
}

export interface EndpointStat {
  ip: string
  hostname?: string
  local: boolean
  country?: string
  asn?: string
  in: number
  out: number
  total: number
  packets: number
}

export interface PortStat {
  port: number
  name?: string
  bytes: number
  packets: number
}

export interface Bucket {
  ts: number
  in: number
  out: number
  local: number
  packets: number
}

export interface DomainStat {
  domain: string
  queries: number
  responses: number
  ips?: string[]
}

export interface Snapshot {
  running: boolean
  device?: string
  error?: string
  started_at?: string
  total_packets: number
  total_bytes: number
  dropped: number
  bps_in: number
  bps_out: number
  bps_local: number
  pps: number
  protocols: Record<string, number>
  top_endpoints: EndpointStat[]
  top_ports: PortStat[]
  top_domains: DomainStat[]
  history: Bucket[]
  local_ip_count: number
}

export interface Connection {
  proto: string
  local_addr: string
  remote_addr?: string
  status?: string
  pid: number
  process?: string
  count: number
  country?: string
  asn?: string
}

export interface Tick {
  stats: Snapshot
  connections: Connection[]
  alert_events?: AlertEvent[]
  record?: RecordInfo
}

export interface RecordInfo {
  recording: boolean
  file?: string
  packets: number
  bytes: number
  error?: string
}

export interface RecordFile {
  name: string
  bytes: number
  mod_time: number
}

export interface DayTotal {
  day: number
  avg_bps_in: number
  avg_bps_out: number
  peak_bps_in: number
  peak_bps_out: number
  samples: number
}

export interface HourAvg {
  hour: number
  bps_in: number
  bps_out: number
}

export interface CompareResponse {
  days: DayTotal[]
  today_hours: HourAvg[]
  yesterday_hours: HourAvg[]
}

export interface AgentRate {
  name: string
  rx_bps: number
  tx_bps: number
  rx_bytes: number
  tx_bytes: number
  pps: number
  rx_packets: number
  tx_packets: number
  last_seen: number
}

export interface AgentWithRates {
  id: number
  name: string
  site: string
  first_seen: number
  last_seen: number
  version: string
  protocol_version: number
  remote_ip: string
  online: boolean
  rates: AgentRate[]
  conns: number
}

export interface AlertEvent {
  id: number
  ts: number
  kind: 'bw' | 'port' | 'proc' | 'target'
  key: string
  message: string
}

export interface AlertConfig {
  enabled: boolean
  cooldown_min: number
  bandwidth: { enabled: boolean; in_mbps: number; out_mbps: number; seconds: number }
  ports: { enabled: boolean; ports: number[] }
  new_proc: { enabled: boolean; ignore: string[] }
  new_target: { enabled: boolean; min_total_mb: number }
  notifiers: {
    desktop: boolean
    generic_url: string
    discord_url: string
    slack_url: string
    telegram_token: string
    telegram_chat_id: string
  }
}

export interface HistoryBucket {
  ts: number
  in: number
  out: number
  local: number
  pps: number
}

export interface HistoryTotals {
  avg_bps_in: number
  avg_bps_out: number
  peak_bps_in: number
  peak_bps_out: number
  seconds: number
  samples: number
}

export interface HistoryResponse {
  range_minutes: number
  db_bytes: number
  totals: HistoryTotals
  buckets: HistoryBucket[]
}

export interface AIStatus {
  enabled: boolean
  model: string
}

export interface ModelInfo {
  id: string
}

export interface Insight {
  id: number
  ts: number
  model: string
  period_minutes: number
  summary: string
}
