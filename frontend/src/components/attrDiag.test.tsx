import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { attrEmptyHint } from './attrDiag'

describe('attrEmptyHint', () => {
  it('pcap + arayüz → yanlış/sanal arayüz ipucu verir', () => {
    render(<div>{attrEmptyHint({ method: 'pcap', iface: 'Tailscale' }, 'süreç trafiği')}</div>)
    expect(screen.getByText('Tailscale')).toBeInTheDocument()
    expect(screen.getByText(/pcap_interface/)).toBeInTheDocument()
  })

  it('off + not → nedeni gösterir ve method satırını kaldırmayı önerir', () => {
    render(<div>{attrEmptyHint({ method: 'off', note: 'collect.method=off' }, 'DNS')}</div>)
    expect(screen.getByText(/collect\.method=off/)).toBeInTheDocument()
  })

  it('ebpf çalışıyor ama boş → "trafik gözlenmemiş olabilir"', () => {
    render(<div>{attrEmptyHint({ method: 'ebpf' }, 'süreç trafiği')}</div>)
    expect(screen.getByText(/gözlenmemiş olabilir/)).toBeInTheDocument()
  })

  it('yöntem bilinmiyor (filo geneli) → generic gereksinim ipucu', () => {
    render(<div>{attrEmptyHint({}, 'uygulama görünürlüğü (L7)')}</div>)
    expect(screen.getByText(/collect\.method: auto/)).toBeInTheDocument()
  })
})
