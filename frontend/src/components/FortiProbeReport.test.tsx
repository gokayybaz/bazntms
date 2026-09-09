import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FortiProbeReport, type ProbeReport } from './FortiProbeReport'

const okReport: ProbeReport = {
  ok: true,
  version: 'v7.2.11',
  build: 1639,
  serial: 'FG100',
  hostname: 'fgt-lab',
  vdom_mode: 'single',
  vdoms: ['root'],
  profile_id: '7.2',
  endpoints: [
    { endpoint: 'monitor/system/interface', label: 'Arayüzler', http_status: 200, ok: true, count: 12, shape: 'object' },
    { endpoint: 'monitor/firewall/policy', label: 'Politika sayaçları', http_status: 403, ok: false, count: 0, shape: '', note: 'yetki reddi' },
    { endpoint: 'monitor/vpn/ipsec', label: 'IPsec tünelleri', http_status: 200, ok: false, count: 0, shape: 'array', note: 'bağlandı, veri yok' },
  ],
  caps: { interface: 'ok', policy_mon: 'denied', vpn_ipsec: 'empty' },
}

describe('FortiProbeReport', () => {
  it('başarılı raporu sürüm + profil + uç durumlarıyla gösterir', () => {
    render(<FortiProbeReport report={okReport} />)
    expect(screen.getByText('FortiOS v7.2.11')).toBeInTheDocument()
    expect(screen.getByText(/profil:/)).toHaveTextContent('7.2')
    expect(screen.getByText('Arayüzler')).toBeInTheDocument()
    expect(screen.getByText(/HTTP 403/)).toBeInTheDocument()
    expect(screen.getByText(/bağlandı, veri yok/)).toBeInTheDocument()
  })

  it('bağlantı başarısızsa hata kutusu gösterir', () => {
    render(<FortiProbeReport report={{ ...okReport, ok: false, error: 'x509: unknown authority' }} />)
    expect(screen.getByText(/bağlantı başarısız/i)).toBeInTheDocument()
    expect(screen.getByText(/x509/)).toBeInTheDocument()
  })
})
