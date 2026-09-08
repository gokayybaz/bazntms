import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { PanelState } from './PanelState'

describe('PanelState', () => {
  it('varsayılan metinleri türe göre gösterir', () => {
    const { rerender } = render(<PanelState kind="loading" />)
    expect(screen.getByText('Yükleniyor…')).toBeInTheDocument()
    expect(screen.getByRole('status')).toBeInTheDocument()

    rerender(<PanelState kind="empty" />)
    expect(screen.getByText('Kayıt yok.')).toBeInTheDocument()

    rerender(<PanelState kind="error" />)
    expect(screen.getByText('Veri alınamadı.')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('özel mesaj + ipucu render eder', () => {
    render(<PanelState kind="empty" message="Cihaz yok." hint="SNMP poller ekleyin." />)
    expect(screen.getByText('Cihaz yok.')).toBeInTheDocument()
    expect(screen.getByText('SNMP poller ekleyin.')).toBeInTheDocument()
  })

  it('error + onRetry → "Yeniden dene" düğmesi çağırır', async () => {
    const onRetry = vi.fn()
    render(<PanelState kind="error" message="patladı" onRetry={onRetry} />)
    await userEvent.click(screen.getByRole('button', { name: 'Yeniden dene' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('loading/empty için retry düğmesi göstermez', () => {
    render(<PanelState kind="loading" onRetry={() => {}} />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
