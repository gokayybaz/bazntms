// Paylaşılan API tipleri — yalnızca birden çok sayfa/bileşende kullanılanlar.
// Sayfaya özel tipler ilgili dosyada yerelde tanımlanır (bkz. CLAUDE.md).

export interface Bucket {
  ts: number
  in: number
  out: number
  local: number
  packets: number
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
  kind: 'bw' | 'port' | 'proc' | 'target' | 'anomaly' | 'vpn_down' | 'sdwan_sla_breach' | 'high_sessions'
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
  anomaly: { enabled: boolean; sensitivity: number; min_samples: number; window_min: number }
  forti: { vpn_down: boolean; sdwan_latency_ms: number; sdwan_jitter_ms: number; sdwan_loss_pct: number; max_sessions: number }
  notifiers: {
    desktop: boolean
    generic_url: string
    discord_url: string
    slack_url: string
    telegram_token: string
    telegram_chat_id: string
    siem?: {
      enabled: boolean
      format: '' | 'cef' | 'leef' | 'json' | 'text'
      transport: '' | 'syslog-udp' | 'syslog-tcp' | 'http'
      target: string
      token: string
      insecure: boolean
    }
  }
}
