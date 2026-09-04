// Uyarı türü → stil/etiket eşlemesi — AlertsCard ve AlertsPage arasında
// paylaşılır. Ayrı dosyada tutulması Fast Refresh'in bileşen dosyalarında
// düzgün çalışmasını sağlar (oxlint: only-export-components).

export const KIND_STYLES: Record<string, string> = {
  bw: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  port: 'bg-rose-500/10 text-rose-400 ring-rose-500/20',
  proc: 'bg-sky-500/10 text-sky-400 ring-sky-500/20',
  // target: violet tek başına DESIGN.md'nin Sabit Anlam Kuralı'nı ihlal
  // ediyordu (violet her zaman cyan ile eşleşmeli, tx trafiğine ayrılmış) —
  // amber'a taşındı (bw ile aynı "eşik/davranışsal uyarı" katmanı,
  // Overview.tsx'in yerel kopyasıyla eşleştirildi)
  target: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  anomaly: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  // ioc: red, 7 renklik sözleşmede tanımsız 8. bir tondu — rose'a taşındı
  // (kritik alarm anlamı zaten rose'a ait, Overview.tsx ile eşleştirildi)
  ioc: 'bg-rose-500/15 text-rose-300 ring-rose-500/30',
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
  ioc: 'ioc / tehdit',
  vpn_down: 'vpn down',
  sdwan_sla_breach: 'sd-wan sla',
  high_sessions: 'yüksek oturum',
}
