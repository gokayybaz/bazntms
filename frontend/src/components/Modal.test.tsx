import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Modal } from './Modal'

describe('Modal', () => {
  it('başlığı ve içeriği dialog rolüyle render eder', () => {
    render(
      <Modal title="Sil onayı" onClose={vi.fn()}>
        <p>emin misiniz</p>
      </Modal>,
    )
    expect(screen.getByRole('dialog', { name: 'Sil onayı' })).toBeInTheDocument()
    expect(screen.getByText('emin misiniz')).toBeInTheDocument()
  })

  it('Escape ve kapat butonu onClose çağırır', async () => {
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal title="X" onClose={onClose}>
        <span>içerik</span>
      </Modal>,
    )
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole('button', { name: 'Kapat' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('arka plan tıklaması kapatır, kutu içi tıklama kapatmaz', async () => {
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(
      <Modal title="X" onClose={onClose}>
        <span>içerik</span>
      </Modal>,
    )
    await user.click(screen.getByText('içerik'))
    expect(onClose).not.toHaveBeenCalled()
    await user.click(screen.getByRole('presentation'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Tab odağı kutu içinde tutar (focus trap)', async () => {
    const user = userEvent.setup()
    render(
      <Modal title="X" onClose={vi.fn()}>
        <button>bir</button>
        <button>iki</button>
      </Modal>,
    )
    const kapat = screen.getByRole('button', { name: 'Kapat' })
    const bir = screen.getByRole('button', { name: 'bir' })
    const iki = screen.getByRole('button', { name: 'iki' })
    iki.focus()
    await user.tab()
    expect(kapat).toHaveFocus() // son → ilk
    await user.tab({ shift: true })
    expect(iki).toHaveFocus() // ilk → son
    expect(bir).not.toHaveFocus()
  })
})
