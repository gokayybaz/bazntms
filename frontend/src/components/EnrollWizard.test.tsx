import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { EnrollWizard } from './EnrollWizard'

const TOKENS = [
  { id: 1, name: 'ofis', site: 'ofis-a', created_at: 1, expires_at: 0, last_used: 0, revoked: false },
]

function mockFetch(overrides?: (url: string, init?: RequestInit) => Response | undefined) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const o = overrides?.(url, init)
      if (o) return Promise.resolve(o)
      if (url === '/api/v1/enroll-tokens' && (!init || init.method === undefined))
        return Promise.resolve({ status: 200, json: async () => TOKENS } as Response)
      return Promise.resolve({ status: 200, ok: true, json: async () => ({ ok: true }), text: async () => '{"ok":true}' } as Response)
    }),
  )
}

describe('EnrollWizard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('token üretilmeden komut göstermez', async () => {
    mockFetch()
    render(<EnrollWizard />)
    await waitFor(() => expect(screen.getByText('ofis')).toBeInTheDocument())
    expect(screen.getByText(/Önce bir enrollment token/)).toBeInTheDocument()
  })

  it('token üretince kurulum komutunu hub URL + token ile gösterir', async () => {
    mockFetch((url, init) => {
      if (url === '/api/v1/enroll-tokens' && init?.method === 'POST')
        return { status: 200, ok: true, json: async () => ({ ok: true, id: 5, token: 'ent_wizardtok99' }) } as Response
      return undefined
    })
    render(<EnrollWizard />)
    await waitFor(() => expect(screen.getByText('ofis')).toBeInTheDocument())

    await userEvent.type(screen.getByPlaceholderText(/ad \(ör/), 'yeni-linux')
    await userEvent.click(screen.getByText('Token Üret'))

    // varsayılan sekme: Linux (binary) — komut hub origin + token içermeli
    const pre = await screen.findByText(/bazntms-agent-linux-amd64/)
    expect(pre.textContent).toContain('ent_wizardtok99')
    expect(pre.textContent).toContain(window.location.origin)
    expect(pre.textContent).toContain('-hub-url')

    // Windows sekmesi → msiexec + ENROLLTOKEN
    await userEvent.click(screen.getByText('Windows (.msi)'))
    const win = await screen.findByText(/msiexec/)
    expect(win.textContent).toContain('ENROLLTOKEN=ent_wizardtok99')
  })

  it('iki-aşamalı iptal DELETE /api/v1/enroll-tokens/{id} çağırır', async () => {
    const calls: string[] = []
    mockFetch((url, init) => {
      if (init?.method === 'DELETE') {
        calls.push(url)
        return { status: 200, ok: true, text: async () => '{"ok":true}' } as Response
      }
      return undefined
    })
    render(<EnrollWizard />)
    await waitFor(() => expect(screen.getByText('ofis')).toBeInTheDocument())
    await userEvent.click(screen.getByText('iptal et'))
    await userEvent.click(screen.getByText('emin misiniz?'))
    await waitFor(() => expect(calls).toEqual(['/api/v1/enroll-tokens/1']))
  })
})
