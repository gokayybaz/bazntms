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
