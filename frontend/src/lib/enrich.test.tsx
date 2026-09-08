import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DomainBadge, IpBadge } from './enrich'

describe('enrich helpers', () => {
  it('IpBadge: RFC1918 → YEREL', () => {
    render(<IpBadge info={{ ip: '10.0.0.1', private: true }} />)
    expect(screen.getByText('yerel')).toBeInTheDocument()
  })

  it('IpBadge: public IP → ülke + ASN·org', () => {
    render(<IpBadge info={{ country: 'US', asn: 'AS399358', org: 'Anthropic, PBC' }} />)
    expect(screen.getByText(/US/)).toBeInTheDocument()
    expect(screen.getByText('AS399358 · Anthropic, PBC')).toBeInTheDocument()
  })

  it('IpBadge: veri yoksa hiçbir şey render etmez', () => {
    const { container } = render(<IpBadge info={{ country: '', asn: '' }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('DomainBadge: kayıtlı alan + kategori', () => {
    render(<DomainBadge info={{ registrable: 'anthropic.com', category: 'AI' }} />)
    expect(screen.getByText('anthropic.com')).toBeInTheDocument()
    expect(screen.getByText('AI')).toBeInTheDocument()
  })
})
