import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'

// TuiTable — imza bileşen. Klavye-öncelikli kolonlu tablo: reverse-video başlık,
// sort göstergesi, ↑↓/jk satır seçimi, `/` filtre, Enter → onActivate. Tuş
// yönetimi tablo-odağına bağlı (yerel keydown) — bir sayfada birden fazla
// TuiTable çakışmaz, yalnızca odaktaki yanıt verir. Bkz. DESIGN.md → TuiTable.

export interface TuiColumn<Row> {
  key: string
  header: string
  /** Hücre içeriği. Yoksa String((row as Record)[key]). */
  render?: (row: Row) => ReactNode
  /** Sıralama anahtarı. Yoksa render çıktısı/[key] string olarak. */
  sortValue?: (row: Row) => string | number
  sortable?: boolean
  align?: 'left' | 'right'
  /** <col> genişliği, ör. "8rem" | "20%". */
  width?: string
}

export interface TuiTableProps<Row> {
  columns: TuiColumn<Row>[]
  rows: Row[]
  getKey: (row: Row) => string
  onActivate?: (row: Row) => void
  /** Serbest-metin filtre kaynağı (satır → aranabilir metin). Yoksa filtre kapalı. */
  filterText?: (row: Row) => string
  filterLabel?: string
  initialSort?: { key: string; dir: 'asc' | 'desc' }
  /** Bu sayının üstünde iç kaydırma sınırı korunur + kesme notu (varsayılan 300). */
  maxRows?: number
  /** Kaydırma bölgesi max yüksekliği (Tailwind sınıfı, varsayılan max-h-[28rem]). */
  scrollClass?: string
  empty?: ReactNode
  className?: string
}

function rawCell<Row>(col: TuiColumn<Row>, row: Row): string {
  if (col.sortValue) {
    const v = col.sortValue(row)
    return typeof v === 'number' ? String(v) : v
  }
  const rec = row as Record<string, unknown>
  return rec[col.key] == null ? '' : String(rec[col.key])
}

function cmp<Row>(col: TuiColumn<Row>, a: Row, b: Row): number {
  const av = col.sortValue ? col.sortValue(a) : rawCell(col, a)
  const bv = col.sortValue ? col.sortValue(b) : rawCell(col, b)
  if (typeof av === 'number' && typeof bv === 'number') return av - bv
  return String(av).localeCompare(String(bv), 'tr', { numeric: true })
}

export function TuiTable<Row>({
  columns,
  rows,
  getKey,
  onActivate,
  filterText,
  filterLabel = 'Filtrele…',
  initialSort,
  maxRows = 300,
  scrollClass = 'max-h-[28rem]',
  empty,
  className = '',
}: TuiTableProps<Row>) {
  const [q, setQ] = useState('')
  const [sort, setSort] = useState<{ key: string; dir: 'asc' | 'desc' } | null>(initialSort ?? null)
  const [selKey, setSelKey] = useState<string | null>(null)
  // Tablo klavye odağında mı — seçili satır o zaman htop tarzı belirgin
  // "imleç" alır (odak dışında sönük seçim).
  const [focused, setFocused] = useState(false)

  const wrapRef = useRef<HTMLDivElement>(null)
  const filterRef = useRef<HTMLInputElement>(null)
  const selRowRef = useRef<HTMLTableRowElement>(null)
  const filterId = useId()

  const filtered = useMemo(() => {
    const needle = q.trim().toLocaleLowerCase('tr')
    if (!needle || !filterText) return rows
    return rows.filter((r) => filterText(r).toLocaleLowerCase('tr').includes(needle))
  }, [rows, q, filterText])

  const sorted = useMemo(() => {
    if (!sort) return filtered
    const col = columns.find((c) => c.key === sort.key)
    if (!col) return filtered
    const out = [...filtered].sort((a, b) => cmp(col, a, b))
    if (sort.dir === 'desc') out.reverse()
    return out
  }, [filtered, sort, columns])

  const view = sorted.length > maxRows ? sorted.slice(0, maxRows) : sorted
  const hiddenCount = sorted.length - view.length

  // Seçim türetilir: kullanıcı seçmediyse (veya seçili satır filtrelendiyse)
  // ilk görünür satır. moveTo her ok tuşunda selKey'i somut bir satıra yazar.
  const selIndex = useMemo(() => {
    if (selKey == null) return view.length ? 0 : -1
    const i = view.findIndex((r) => getKey(r) === selKey)
    return i >= 0 ? i : view.length ? 0 : -1
  }, [view, selKey, getKey])

  useEffect(() => {
    // yalnızca kullanıcı satır seçtiyse (j/k/ok) kaydır — türetilmiş varsayılan
    // seçim (selKey == null) sayfa açılışında tabloyu görünüme kaydırıp
    // "ortadan başlıyor" hissi veriyordu
    if (selKey == null) return
    selRowRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selIndex, selKey])

  const moveTo = useCallback(
    (i: number) => {
      const clamped = Math.max(0, Math.min(view.length - 1, i))
      if (view[clamped]) setSelKey(getKey(view[clamped]))
    },
    [view, getKey],
  )

  const cycleSort = useCallback(
    (dir: 'asc' | 'desc') => {
      const sortables = columns.filter((c) => c.sortable)
      if (!sortables.length) return
      const curIdx = sortables.findIndex((c) => c.key === sort?.key)
      const next = sortables[(curIdx + 1) % sortables.length]
      setSort({ key: next.key, dir })
    },
    [columns, sort],
  )

  const toggleSort = useCallback((key: string) => {
    setSort((s) => (s?.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }))
  }, [])

  const onKeyDown = (e: React.KeyboardEvent) => {
    const inFilter = e.target === filterRef.current
    switch (e.key) {
      case 'ArrowDown':
        moveTo(selIndex + 1)
        e.preventDefault()
        break
      case 'ArrowUp':
        moveTo(selIndex - 1)
        e.preventDefault()
        break
      case 'j':
        if (!inFilter) {
          moveTo(selIndex + 1)
          e.preventDefault()
        }
        break
      case 'k':
        if (!inFilter) {
          moveTo(selIndex - 1)
          e.preventDefault()
        }
        break
      case 'g':
        if (!inFilter) {
          moveTo(0)
          e.preventDefault()
        }
        break
      case 'G':
        if (!inFilter) {
          moveTo(view.length - 1)
          e.preventDefault()
        }
        break
      case 'Enter':
        if (selIndex >= 0 && view[selIndex]) {
          onActivate?.(view[selIndex])
          e.preventDefault()
        }
        break
      case '/':
        if (!inFilter && filterText) {
          filterRef.current?.focus()
          e.preventDefault()
        }
        break
      case 's':
        if (!inFilter) {
          cycleSort('asc')
          e.preventDefault()
        }
        break
      case 'S':
        if (!inFilter) {
          cycleSort('desc')
          e.preventDefault()
        }
        break
      case 'F6':
        cycleSort('asc')
        e.preventDefault()
        break
      case 'Escape':
        if (inFilter) {
          setQ('')
          wrapRef.current?.focus()
          e.preventDefault()
        }
        break
    }
  }

  return (
    <div
      ref={wrapRef}
      tabIndex={0}
      onKeyDown={onKeyDown}
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setFocused(false)
      }}
      className={`border bg-panel outline-none transition-colors ${
        focused ? 'border-rx/60' : 'border-rule'
      } ${className}`}
    >
      {filterText && (
        <div className="flex items-center gap-2 border-b border-rule px-2 py-1">
          <label htmlFor={filterId} className="font-mono text-[11px] text-tui-dim">
            /
          </label>
          <input
            id={filterId}
            ref={filterRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={filterLabel}
            aria-label={filterLabel}
            className="min-w-0 flex-1 bg-transparent font-mono text-[11px] text-ink outline-none placeholder:text-tui-dim"
          />
          <span className="shrink-0 font-mono text-[10px] tabular-nums text-tui-dim">
            {q.trim() ? `${sorted.length} / ${rows.length}` : rows.length}
          </span>
        </div>
      )}

      <div className={`overflow-auto ${scrollClass}`}>
        <table className="w-full border-collapse font-mono text-[11px]">
          {columns.some((c) => c.width) && (
            <colgroup>
              {columns.map((c) => (
                <col key={c.key} style={c.width ? { width: c.width } : undefined} />
              ))}
            </colgroup>
          )}
          <thead className="sticky top-0 z-10">
            <tr>
              {columns.map((c) => {
                const active = sort?.key === c.key
                return (
                  <th
                    key={c.key}
                    scope="col"
                    aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
                    onClick={c.sortable ? () => toggleSort(c.key) : undefined}
                    className={`whitespace-nowrap bg-rx px-2 py-1 font-medium uppercase tracking-[0.04em] text-ground ${
                      c.align === 'right' ? 'text-right' : 'text-left'
                    } ${c.sortable ? 'cursor-pointer select-none' : ''}`}
                  >
                    {c.header}
                    {active ? (sort.dir === 'asc' ? ' ▲' : ' ▼') : c.sortable ? ' ·' : ''}
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {view.map((row, i) => {
              const k = getKey(row)
              const selected = i === selIndex
              return (
                <tr
                  key={k}
                  ref={selected ? selRowRef : undefined}
                  aria-selected={selected}
                  onClick={() => {
                    setSelKey(k)
                    onActivate?.(row)
                  }}
                  className={`${
                    selected
                      ? focused
                        ? 'bg-rx/25 text-ink-hi outline outline-1 -outline-offset-1 outline-rx/70'
                        : 'bg-rule-hi/50 text-ink-hi'
                      : i % 2
                        ? 'bg-panel-2/50 text-ink'
                        : 'text-ink'
                  } ${onActivate ? 'cursor-pointer' : ''}`}
                >
                  {columns.map((c) => (
                    <td
                      key={c.key}
                      className={`whitespace-nowrap px-2 py-1 ${c.align === 'right' ? 'text-right' : 'text-left'}`}
                    >
                      {c.render ? c.render(row) : rawCell(c, row)}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>

        {view.length === 0 && (
          <div className="px-2 py-8 text-center font-mono text-[11px] text-tui-dim">
            {empty ?? (q.trim() ? 'Eşleşen satır yok.' : 'Kayıt yok.')}
          </div>
        )}
        {hiddenCount > 0 && (
          <p className="border-t border-rule px-2 py-1 text-center font-mono text-[10px] text-tui-dim">
            +{hiddenCount} satır daha — filtreyle daraltın
          </p>
        )}
      </div>
    </div>
  )
}
