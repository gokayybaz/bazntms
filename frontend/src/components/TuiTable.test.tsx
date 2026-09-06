import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'

interface Agent {
  id: string
  name: string
  conns: number
}

const AGENTS: Agent[] = [
  { id: 'a', name: 'edge-ist-01', conns: 120 },
  { id: 'b', name: 'edge-ank-02', conns: 40 },
  { id: 'c', name: 'edge-izm-03', conns: 999 },
]

const COLS: TuiColumn<Agent>[] = [
  { key: 'name', header: 'Ad', sortable: true },
  { key: 'conns', header: 'Bağlantı', align: 'right', sortable: true, sortValue: (r) => r.conns },
]

function setup(props: Partial<React.ComponentProps<typeof TuiTable<Agent>>> = {}) {
  const onActivate = vi.fn()
  render(
    <TuiTable
      columns={COLS}
      rows={AGENTS}
      getKey={(r) => r.id}
      onActivate={onActivate}
      filterText={(r) => r.name}
      {...props}
    />,
  )
  return { onActivate, user: userEvent.setup() }
}

const selectedRowText = () => screen.getByRole('row', { selected: true }).textContent

describe('TuiTable', () => {
  it('başlıkları ve satırları render eder, ilk satır seçili başlar', () => {
    setup()
    expect(screen.getByRole('columnheader', { name: /Ad/ })).toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(1 + AGENTS.length)
    expect(selectedRowText()).toContain('edge-ist-01')
  })

  it('ArrowDown / ArrowUp seçimi taşır (wrap yok)', async () => {
    const { user } = setup()
    const grid = screen.getAllByRole('row')[1].closest('div')!.parentElement!
    grid.focus()
    await user.keyboard('{ArrowDown}')
    expect(selectedRowText()).toContain('edge-ank-02')
    await user.keyboard('{ArrowUp}{ArrowUp}')
    expect(selectedRowText()).toContain('edge-ist-01') // baştan yukarı çıkmaz
  })

  it('Enter seçili satırla onActivate çağırır', async () => {
    const { onActivate, user } = setup()
    const wrap = screen.getByRole('table').closest('[tabindex]') as HTMLElement
    wrap.focus()
    await user.keyboard('{ArrowDown}{Enter}')
    expect(onActivate).toHaveBeenCalledWith(AGENTS[1])
  })

  it('/ filtreye odaklanır, yazınca daraltır ve sayaç "n / toplam" gösterir', async () => {
    const { user } = setup()
    const wrap = screen.getByRole('table').closest('[tabindex]') as HTMLElement
    wrap.focus()
    await user.keyboard('/')
    expect(screen.getByLabelText('Filtrele…')).toHaveFocus()
    await user.keyboard('ank')
    expect(screen.getAllByRole('row')).toHaveLength(1 + 1)
    expect(screen.getByText('1 / 3')).toBeInTheDocument()
  })

  it('Escape filtreyi temizler ve tabloya döner', async () => {
    const { user } = setup()
    const wrap = screen.getByRole('table').closest('[tabindex]') as HTMLElement
    wrap.focus()
    await user.keyboard('/ank')
    await user.keyboard('{Escape}')
    expect(screen.getByLabelText('Filtrele…')).toHaveValue('')
    expect(screen.getAllByRole('row')).toHaveLength(1 + AGENTS.length)
  })

  it('sortable başlığa tıklama asc→desc çevirir', async () => {
    const { user } = setup()
    const connHeader = screen.getByRole('columnheader', { name: /Bağlantı/ })
    await user.click(connHeader) // asc
    let bodyRows = screen.getAllByRole('row').slice(1)
    expect(within(bodyRows[0]).getByText('40')).toBeInTheDocument()
    await user.click(connHeader) // desc
    bodyRows = screen.getAllByRole('row').slice(1)
    expect(within(bodyRows[0]).getByText('999')).toBeInTheDocument()
    expect(connHeader).toHaveAttribute('aria-sort', 'descending')
  })

  it('s tuşu sortable kolonlar arasında döner', async () => {
    const { user } = setup()
    const wrap = screen.getByRole('table').closest('[tabindex]') as HTMLElement
    wrap.focus()
    await user.keyboard('s') // ilk sortable: name asc
    expect(screen.getByRole('columnheader', { name: /Ad/ })).toHaveAttribute('aria-sort', 'ascending')
    await user.keyboard('s') // sonraki: conns asc
    expect(screen.getByRole('columnheader', { name: /Bağlantı/ })).toHaveAttribute('aria-sort', 'ascending')
  })

  it('boş sonuç mesajı gösterir', async () => {
    const { user } = setup()
    const wrap = screen.getByRole('table').closest('[tabindex]') as HTMLElement
    wrap.focus()
    await user.keyboard('/zzz')
    expect(screen.getByText('Eşleşen satır yok.')).toBeInTheDocument()
  })

  it('maxRows üstünde kesme notu gösterir', () => {
    const many = Array.from({ length: 10 }, (_, i) => ({ id: `x${i}`, name: `n${i}`, conns: i }))
    render(<TuiTable columns={COLS} rows={many} getKey={(r) => r.id} maxRows={4} />)
    expect(screen.getByText('+6 satır daha — filtreyle daraltın')).toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(1 + 4)
  })
})
