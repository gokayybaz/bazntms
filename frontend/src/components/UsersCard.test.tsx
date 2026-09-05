import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { UsersCard } from './UsersCard'

const USERS = [
  { id: 1, username: 'ada', role: 'admin', site: '', enabled: true, created_at: 1, last_login: 0 },
  { id: 2, username: 'bob', role: 'viewer', site: 'dc1', enabled: false, created_at: 1, last_login: 1_700_000_000 },
]

function mockFetch(overrides?: (url: string, init?: RequestInit) => Response | undefined) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      const o = overrides?.(url, init)
      if (o) return Promise.resolve(o)
      if (url === '/api/v1/users' && (!init || init.method === undefined || init.method === 'GET'))
        return Promise.resolve({ status: 200, json: async () => USERS } as Response)
      return Promise.resolve({ status: 200, ok: true, text: async () => '{"ok":true}', json: async () => ({ ok: true }) } as Response)
    }),
  )
}

describe('UsersCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('kullanıcı listesini ve rollerini gösterir', async () => {
    mockFetch()
    render(<UsersCard />)
    await waitFor(() => expect(screen.getByText('ada')).toBeInTheDocument())
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText('etkin')).toBeInTheDocument()
    expect(screen.getByText('pasif')).toBeInTheDocument()
  })

  it('rol değişikliği PUT /api/v1/users/{id} çağırır', async () => {
    const calls: { url: string; init?: RequestInit }[] = []
    mockFetch((url, init) => {
      if (init?.method === 'PUT') {
        calls.push({ url, init })
        return { status: 200, ok: true, text: async () => '{"ok":true}' } as Response
      }
      return undefined
    })
    render(<UsersCard />)
    await waitFor(() => expect(screen.getByText('bob')).toBeInTheDocument())
    const bobRow = screen.getByText('bob').closest('tr')!
    const roleSelect = bobRow.querySelector('select') as HTMLSelectElement
    await userEvent.selectOptions(roleSelect, 'netops')
    expect(calls[0].url).toBe('/api/v1/users/2')
    expect(JSON.parse(calls[0].init!.body as string)).toEqual({ role: 'netops' })
  })

  it('sunucu hatasını (son admin koruması) ekranda gösterir', async () => {
    mockFetch((_url, init) => {
      if (init?.method === 'DELETE')
        return { status: 400, ok: false, text: async () => 'sistemdeki son etkin yöneticiyi silemezsiniz' } as Response
      return undefined
    })
    render(<UsersCard />)
    await waitFor(() => expect(screen.getByText('ada')).toBeInTheDocument())
    const adaRow = screen.getByText('ada').closest('tr')!
    const delBtn = [...adaRow.querySelectorAll('button')].find((b) => b.textContent === 'sil')!
    await userEvent.click(delBtn) // 1. tık → onay
    await userEvent.click(screen.getByText('emin misiniz?')) // 2. tık → DELETE
    await waitFor(() => expect(screen.getByText(/son etkin yöneticiyi silemezsiniz/)).toBeInTheDocument())
  })
})
