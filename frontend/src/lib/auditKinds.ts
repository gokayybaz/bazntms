// Denetim eylemi → renk sınıfı. Eylem adları "nesne.fiil" (user.create,
// token.revoke) veya sabit (login, denied). Kategori pill rengi için.
export function auditTone(action: string): string {
  if (action === 'denied' || action.endsWith('.failed')) return 'bg-rose-500/10 text-rose-400 ring-rose-500/20'
  if (/\.(create|add)$/.test(action)) return 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/20'
  if (/\.(delete|revoke|remove)$/.test(action)) return 'bg-amber-500/10 text-amber-400 ring-amber-500/20'
  if (/\.(update|rename|version)$/.test(action)) return 'bg-cyan-500/10 text-cyan-300 ring-cyan-500/20'
  return 'bg-slate-500/10 text-slate-400 ring-slate-500/20'
}
