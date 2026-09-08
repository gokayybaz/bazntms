// Paylaşılan zenginleştirme render yardımcıları (Faz 23-E). Süreç detayı,
// NetFlow konuşmaları ve coğrafi görünüm aynı IP/alan rozetini kullanır.
// Backend: internal/enrich · GET /api/v1/enrich?ip=&domain=

import { flagEmoji } from './format'

export interface IPInfo {
  ip?: string
  private?: boolean
  country?: string
  asn?: string
  org?: string
}
export interface DomainInfo {
  domain?: string
  normalized?: string
  registrable?: string
  category?: string
}

/** Bir uzak IP'nin bağlamsal rozeti — ülke bayrağı + ASN + org; RFC1918 → YEREL. */
export function IpBadge({ info, className = '' }: { info?: IPInfo | null; className?: string }) {
  if (!info) return null
  if (info.private) {
    return <span className={`border border-rule px-1 text-[10px] uppercase text-tui-dim ${className}`}>yerel</span>
  }
  const flag = flagEmoji(info.country)
  if (!flag && !info.asn) return null
  return (
    <span className={`inline-flex items-center gap-1 text-[10px] text-tui-dim ${className}`}>
      {info.country && (
        <span className="border border-rule px-1 uppercase">
          {flag} {info.country}
        </span>
      )}
      {info.asn && <span title={info.org}>{info.org ? `${info.asn} · ${info.org}` : info.asn}</span>}
    </span>
  )
}

export type Reputation = 'trusted' | 'neutral' | 'suspicious' | 'malicious' | 'unknown'

const REP_STYLE: Record<Reputation, string> = {
  malicious: 'border-rose-500 text-rose-300 bg-rose-500/15',
  suspicious: 'border-amber-500 text-amber-300 bg-amber-500/15',
  trusted: 'border-emerald-500 text-emerald-300 bg-emerald-500/10',
  neutral: 'border-rule text-tui-dim',
  unknown: 'border-rule text-tui-dim',
}
const REP_LABEL: Record<Reputation, string> = {
  malicious: 'kötücül',
  suspicious: 'şüpheli',
  trusted: 'güvenilir',
  neutral: 'nötr',
  unknown: 'bilinmiyor',
}

/** Tehdit itibarı pill'i (Faz 24-E). unknown/neutral → hiçbir şey (gürültü yok). */
export function RepBadge({ reputation, source, className = '' }: { reputation?: string | null; source?: string; className?: string }) {
  const r = (reputation ?? '') as Reputation
  if (!r || r === 'unknown' || r === 'neutral') return null
  return (
    <span
      className={`inline-block border px-1 font-mono text-[10px] uppercase tracking-[0.04em] ${REP_STYLE[r] ?? REP_STYLE.unknown} ${className}`}
      title={source ? `kaynak: ${source}` : undefined}
    >
      {REP_LABEL[r] ?? r}
    </span>
  )
}

/** Bir alan adının rozeti — kayıtlı alan + (varsa) kategori. */
export function DomainBadge({ info, className = '' }: { info?: DomainInfo | null; className?: string }) {
  if (!info || !info.registrable) return null
  return (
    <span className={`inline-flex items-center gap-1 text-[10px] text-tui-dim ${className}`}>
      <span>{info.registrable}</span>
      {info.category && <span className="border border-rule px-1 uppercase text-emerald-400">{info.category}</span>}
    </span>
  )
}
