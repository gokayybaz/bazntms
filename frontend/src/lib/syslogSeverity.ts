// Syslog önem seviyesi (RFC 5424, 0=emergency..7=debug) → etiket/stil eşlemesi
// — SyslogCard ve DeviceDetailPage arasında paylaşılır. Daha önce ikisi de
// kendi yerel kopyasını taşıyordu; DeviceDetailPage'inki hiç güncellenmemiş
// düz bir stile sahipti (impeccable critique 2026-09-05) — alertKinds.ts'te
// aynı "iki elle-kopyalanan kopyanın sessizce sürüklenmesi" örüntüsü zaten
// bir kez yaşanmıştı, bu yüzden tek kaynağa çıkarıldı.

export const SEV_NAMES = ['emergency', 'alert', 'critical', 'error', 'warning', 'notice', 'info', 'debug']

export const SEV_STYLES: Record<number, string> = {
  0: 'bg-rose-600/20 text-rose-300 ring-rose-500/40',
  1: 'bg-rose-600/20 text-rose-300 ring-rose-500/40',
  2: 'bg-rose-500/15 text-rose-400 ring-rose-500/30',
  3: 'bg-amber-500/15 text-amber-400 ring-amber-500/30',
  4: 'bg-amber-500/10 text-amber-300/80 ring-amber-500/20',
  5: 'bg-sky-500/10 text-sky-400 ring-sky-500/20',
  6: 'bg-slate-500/10 text-slate-400 ring-slate-500/20',
  7: 'bg-slate-500/10 text-slate-500 ring-slate-500/20',
}
