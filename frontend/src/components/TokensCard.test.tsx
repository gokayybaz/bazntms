import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TokensCard } from './TokensCard'

const TOKENS = [
  { id: 1, name: 'grafana', role: 'analyst', site: '', created_at: 1, last_used: 1_700_000_000, revoked: false },
  { id: 2, name: 'eski-ci', role: 'viewer', site: '', created_at: 1, last_used: 0, revoked: true },
]

function mockFetch(overrides?: (url: string, init?: RequestInit) => Response | undefined) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const o = overrides?.(url, init)
      if (o) return Promise.resolve(o)
      if (url === '/api/v1/tokens' && (!init || init.method === undefined))
        return Promise.resolve({ status: 200, json: async () => TOKENS } as Response)
      return Promise.resolve({ status: 200, ok: true, json: async () => ({ ok: true }), text: async () => '{"ok":true}' } as Response)
    }),
  )
}

describe('TokensCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('etkin ve iptal edilmiş token’ları ayırt eder', async () => {
    mockFetch()
    render(<TokensCard />)
    await waitFor(() => expect(screen.getByText('grafana')).toBeInTheDocument())
    expect(screen.getByText('etkin')).toBeInTheDocument()
    expect(screen.getByText('iptal')).toBeInTheDocument()
    // iptal edilmiş token'da "iptal et" butonu yok
    const revokedRow = screen.getByText('eski-ci').closest('tr')!
    expect([...revokedRow.querySelectorAll('button')].some((b) => b.textContent === 'iptal et')).toBe(false)
  })

  it('token oluşturunca düz değeri bir kez modalda gösterir', async () => {
    mockFetch((url, init) => {
      if (url === '/api/v1/tokens' && init?.method === 'POST')
        return { status: 200, ok: true, json: async () => ({ ok: true, id: 9, token: 'bnt_deadbeef123' }) } as Response
      return undefined
    })
    render(<TokensCard />)
    await waitFor(() => expect(screen.getByText('grafana')).toBeInTheDocument())

    await userEvent.click(screen.getByText('+ Token Oluştur'))
    await userEvent.type(screen.getByPlaceholderText(/ad \(ör/), 'yeni-token')
    await userEvent.click(screen.getByText('Oluştur'))

    await waitFor(() => expect(screen.getByText('bnt_deadbeef123')).toBeInTheDocument())
    expect(screen.getByText(/yalnızca bir kez/)).toBeInTheDocument()
  })

  it('iki-aşamalı iptal DELETE /api/v1/tokens/{id} çağırır', async () => {
    const calls: string[] = []
    mockFetch((url, init) => {
      if (init?.method === 'DELETE') {
        calls.push(url)
        return { status: 200, ok: true, text: async () => '{"ok":true}' } as Response
      }
      return undefined
    })
    render(<TokensCard />)
    await waitFor(() => expect(screen.getByText('grafana')).toBeInTheDocument())
    const row = screen.getByText('grafana').closest('tr')!
    const btn = [...row.querySelectorAll('button')].find((b) => b.textContent === 'iptal et')!
    await userEvent.click(btn)
    await userEvent.click(screen.getByText('emin misiniz?'))
    await waitFor(() => expect(calls).toEqual(['/api/v1/tokens/1']))
  })
})
