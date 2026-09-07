import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { DialogProvider, useDialog } from './dialog'

// confirm/prompt sonucunu ekrana yazan test sürücüsü
function Harness() {
  const { confirm, prompt, form } = useDialog()
  const [out, setOut] = useState('—')
  return (
    <div>
      <button onClick={async () => setOut(String(await confirm('Silinsin mi?', { danger: true })))}>ask-confirm</button>
      <button onClick={async () => setOut(String(await prompt('Yeni ad', { defaultValue: 'eski' })))}>ask-prompt</button>
      <button
        onClick={async () =>
          setOut(
            JSON.stringify(
              await form({
                title: 'Risk',
                fields: [
                  { key: 'threat', label: 'Tehdit' },
                  { key: 'impact', label: 'Etki', type: 'number', defaultValue: '3' },
                ],
              }),
            ),
          )
        }
      >
        ask-form
      </button>
      <button
        onClick={async () =>
          setOut(
            JSON.stringify(
              await form({
                title: 'SNMP',
                fields: [
                  { key: 'host', label: 'Host' },
                  { key: 'community', label: 'Community', type: 'password' },
                ],
              }),
            ),
          )
        }
      >
        ask-secret
      </button>
      <output>{out}</output>
    </div>
  )
}

const renderHarness = () => {
  const user = userEvent.setup()
  render(
    <DialogProvider>
      <Harness />
    </DialogProvider>,
  )
  return { user }
}

describe('useDialog', () => {
  it('confirm → Onayla true, İptal false döner', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-confirm'))
    await user.click(screen.getByRole('button', { name: 'Onayla' }))
    expect(screen.getByRole('status').textContent).toBe('true')

    await user.click(screen.getByText('ask-confirm'))
    await user.click(screen.getByRole('button', { name: 'İptal' }))
    expect(screen.getByRole('status').textContent).toBe('false')
  })

  it('confirm → Escape false döner', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-confirm'))
    await user.keyboard('{Escape}')
    expect(screen.getByRole('status').textContent).toBe('false')
  })

  it('prompt → düzenlenen değeri, İptal null döner', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-prompt'))
    const input = screen.getByRole('textbox')
    expect(input).toHaveValue('eski')
    await user.clear(input)
    await user.type(input, 'yeni-ad{Enter}')
    expect(screen.getByRole('status').textContent).toBe('yeni-ad')

    await user.click(screen.getByText('ask-prompt'))
    await user.click(screen.getByRole('button', { name: 'İptal' }))
    expect(screen.getByRole('status').textContent).toBe('null')
  })

  it('form → alanları toplar, İptal null döner', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-form'))
    const inputs = screen.getAllByRole('spinbutton').concat(screen.getAllByRole('textbox'))
    await user.type(screen.getByRole('textbox'), 'sızıntı')
    await user.click(screen.getByRole('button', { name: 'Kaydet' }))
    expect(screen.getByRole('status').textContent).toContain('"threat":"sızıntı"')
    expect(screen.getByRole('status').textContent).toContain('"impact":"3"')
    expect(inputs.length).toBeGreaterThan(0)

    await user.click(screen.getByText('ask-form'))
    await user.click(screen.getByRole('button', { name: 'İptal' }))
    expect(screen.getByRole('status').textContent).toBe('null')
  })

  it('form password alanı type="password" input üretir ve değeri toplar', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-secret'))
    const pw = document.querySelector('input[type="password"]') as HTMLInputElement
    expect(pw).toBeTruthy()
    await user.type(screen.getByRole('textbox'), '10.0.0.2')
    await user.type(pw, 's3cret')
    await user.click(screen.getByRole('button', { name: 'Kaydet' }))
    expect(screen.getByRole('status').textContent).toContain('"community":"s3cret"')
  })

  it('danger onay butonu rose reverse-video sınıfı taşır', async () => {
    const { user } = renderHarness()
    await user.click(screen.getByText('ask-confirm'))
    expect(screen.getByRole('button', { name: 'Onayla' })).toHaveClass('bg-rose-400')
  })

  it('sağlayıcı dışında useDialog hata verir', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<Harness />)).toThrow(/DialogProvider/)
    spy.mockRestore()
  })
})
