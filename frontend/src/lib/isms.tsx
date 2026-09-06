// ISMS (ISO 27001) paylaşılan tipler + yardımcılar — Uyumluluk alt
// sayfaları arasında ortak (Risk, SoA, Politika, Denetim, Yönetişim).

export interface SoaCounts {
  total: number
  applicable: number
  implemented: number
  verified: number
  excluded: number
}
export interface Summary {
  soa: SoaCounts
  risks: { total: number; open: number; high: number; medium: number; low: number }
  open_findings: number
  policies_published: number
  assets: number
  suppliers_due: number
  last_continuity_test?: ContinuityTest
}
export interface Asset {
  id: number
  kind: string
  name: string
  owner: string
  criticality: string
  auto: boolean
}
export interface Risk {
  id: number
  asset_id: number
  threat: string
  vulnerability: string
  impact: number
  likelihood: number
  score: number
  treatment: string
  plan: string
  res_score: number
  owner: string
  status: string
}
export interface SoaItem {
  control_id: string
  category: string
  title: string
  applicable: boolean
  justification: string
  status: string
  evidence: string
  owner: string
}
export interface Policy {
  id: number
  ref: string
  title: string
  owner: string
  status: string
  version: string
  approved_by: string
  next_review: number
}
export interface Audit {
  id: number
  title: string
  scope: string
  planned_date: string
  auditor: string
  status: string
}
export interface Finding {
  id: number
  audit_id: number
  ref: string
  description: string
  severity: string
  control_id: string
  capa: string
  capa_owner: string
  capa_due: string
  status: string
  verified_by: string
}
export interface MgmtReview {
  id: number
  ts: number
  period: string
  attendees: string
  decisions: string
  actions: string
}
export interface Supplier {
  id: number
  name: string
  service: string
  criticality: string
  next_review: number
}
export interface ContinuityTest {
  id: number
  kind: string
  title: string
  performed_at: number
  result: string
  evidence: string
}

export type Tone = 'ok' | 'warn' | 'bad' | 'muted'

export function pill(text: string, tone: Tone = 'muted') {
  const tones: Record<Tone, string> = {
    ok: 'border-emerald-500/40 text-emerald-400',
    warn: 'border-amber-500/40 text-amber-400',
    bad: 'border-rose-500/40 text-rose-400',
    muted: 'border-rule-hi text-tui-dim',
  }
  return <span className={`border px-1 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] ${tones[tone]}`}>{text}</span>
}

export const riskTone = (score: number): Tone => (score >= 15 ? 'bad' : score >= 8 ? 'warn' : 'ok')
export const statusTone = (s: string): Tone => {
  const map: Record<string, Tone> = {
    published: 'ok', verified: 'ok', closed: 'ok', implemented: 'ok', approved: 'ok', done: 'ok',
    in_review: 'warn', in_progress: 'warn', planned: 'muted', open: 'bad',
  }
  return map[s] ?? 'muted'
}

export const btnCls =
  'border border-rule-hi px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] text-tui-dim transition hover:border-ink-hi hover:text-ink-hi'

// ask — geçici native prompt sarmalayıcı. ISMS sayfaları dokunuldukça
// useDialog().prompt()'a taşınıyor; hepsi taşınınca bu silinecek.
export const ask = (q: string, def = '') => prompt(q, def) ?? ''

export async function ismsPost(url: string, body: unknown) {
  await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).catch(() => {})
}
export async function ismsPut(url: string, body: unknown) {
  await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).catch(() => {})
}
export async function ismsDel(url: string) {
  await fetch(url, { method: 'DELETE' }).catch(() => {})
}
