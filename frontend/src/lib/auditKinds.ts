// Denetim eylemi → renk sınıfı. Eylem adları "nesne.fiil" (user.create,
// token.revoke) veya sabit (login, denied). TUI: dolgusuz, kenarlıklı pill.
export function auditTone(action: string): string {
  if (action === 'denied' || action.endsWith('.failed')) return 'border-rose-500/40 text-rose-400'
  if (/\.(create|add)$/.test(action)) return 'border-emerald-500/40 text-emerald-400'
  if (/\.(delete|revoke|remove)$/.test(action)) return 'border-amber-500/40 text-amber-400'
  if (/\.(update|rename|version)$/.test(action)) return 'border-rx/40 text-rx'
  return 'border-rule-hi text-tui-dim'
}
