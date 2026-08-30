// Uyarı türü → stil/etiket eşlemesi — AlertsCard ve AlertsPage arasında
// paylaşılır. Ayrı dosyada tutulması Fast Refresh'in bileşen dosyalarında
// düzgün çalışmasını sağlar (oxlint: only-export-components).

export const KIND_STYLES: Record<string, string> = {
  bw: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  port: 'bg-rose-500/10 text-rose-400 ring-rose-500/20',
  proc: 'bg-sky-500/10 text-sky-400 ring-sky-500/20',
  target: 'bg-violet-500/10 text-violet-400 ring-violet-500/20',
  anomaly: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  vpn_down: 'bg-rose-500/10 text-rose-400 ring-rose-500/20',
  sdwan_sla_breach: 'bg-orange-500/10 text-orange-300 ring-orange-500/20',
  high_sessions: 'bg-orange-500/10 text-orange-300 ring-orange-500/20',
}

export const KIND_LABELS: Record<string, string> = {
  bw: 'bant genişliği',
  port: 'şüpheli port',
  proc: 'yeni süreç',
  target: 'yeni hedef',
  anomaly: 'anomali',
  vpn_down: 'vpn down',
  sdwan_sla_breach: 'sd-wan sla',
  high_sessions: 'yüksek oturum',
}
