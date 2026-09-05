import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuditCard } from './AuditCard'
import { auditTone } from '../lib/auditKinds'

const EVENTS = [
  { id: 3, ts: 1_700_000_200, username: 'ada', role: 'admin', action: 'user.create', target: 'user:bob', detail: 'rol: viewer', ip: '10.0.0.1', hash: 'h3' },
  { id: 2, ts: 1_700_000_100, username: 'ada', role: 'admin', action: 'login', target: 'legacy', detail: '', ip: '10.0.0.1', hash: 'h2' },
  { id: 1, ts: 1_700_000_000, username: '', role: '', action: 'login.failed', target: 'user:x', detail: 'hatalı', ip: '10.0.0.9', hash: 'h1' },
]

function mockFetch(verify = { ok: true, broken_at: 0, checked: 3 }) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url.startsWith('/api/v1/audit/verify'))
        return Promise.resolve({ ok: true, json: async () => verify } as Response)
      if (url.startsWith('/api/v1/audit'))
        return Promise.resolve({ status: 200, ok: true, json: async () => EVENTS } as Response)
      return Promise.resolve({ status: 404, json: async () => ({}) } as Response)
    }),
  )
}

describe('auditTone', () => {
  it('eylem kategorisine göre renk verir', () => {
    expect(auditTone('user.create')).toContain('emerald')
    expect(auditTone('token.revoke')).toContain('amber')
    expect(auditTone('user.update')).toContain('cyan')
    expect(auditTone('login.failed')).toContain('rose')
    expect(auditTone('denied')).toContain('rose')
    expect(auditTone('login')).toContain('slate')
  })
})

describe('AuditCard', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('zincir sağlam rozeti + olay tablosu gösterir', async () => {
    mockFetch()
    render(<AuditCard />)
    await waitFor(() => expect(screen.getByText('user.create')).toBeInTheDocument())
    expect(screen.getByText('✓ zincir sağlam')).toBeInTheDocument()
    expect(screen.getByText('3 kayıt')).toBeInTheDocument()
    expect(screen.getByText('user:bob')).toBeInTheDocument()
    expect(screen.getByText('login.failed')).toBeInTheDocument()
  })

  it('zincir bozuksa kırık kayıt numarasını gösterir', async () => {
    mockFetch({ ok: false, broken_at: 42, checked: 41 })
    render(<AuditCard />)
    await waitFor(() => expect(screen.getByText(/zincir bozuk — kayıt #42/)).toBeInTheDocument())
  })

  it('limit değişince yeni istek atar', async () => {
    const urls: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) => {
        urls.push(url)
        if (url.includes('verify')) return Promise.resolve({ ok: true, json: async () => ({ ok: true, checked: 0 }) } as Response)
        return Promise.resolve({ status: 200, ok: true, json: async () => [] } as Response)
      }),
    )
    render(<AuditCard />)
    await waitFor(() => expect(urls.some((u) => u.includes('limit=100'))).toBe(true))
    await userEvent.selectOptions(screen.getByRole('combobox'), '250')
    await waitFor(() => expect(urls.some((u) => u.includes('limit=250'))).toBe(true))
  })
})
