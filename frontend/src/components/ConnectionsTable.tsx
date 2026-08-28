import { useMemo, useState } from 'react'
import type { Connection } from '../types'
import { flagEmoji, formatNum } from '../lib/format'

const STATUS_STYLES: Record<string, string> = {
  ESTABLISHED: 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/20',
  LISTEN: 'bg-sky-500/10 text-sky-400 ring-sky-500/20',
  TIME_WAIT: 'bg-slate-500/10 text-slate-400 ring-slate-500/20',
  CLOSE_WAIT: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  SYN_SENT: 'bg-violet-500/10 text-violet-400 ring-violet-500/20',
}

export function ConnectionsTable({ connections }: { connections: Connection[] }) {
  const [query, setQuery] = useState('')
  const [proto, setProto] = useState<'all' | 'tcp' | 'udp'>('all')

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return connections.filter((c) => {
      if (proto !== 'all' && c.proto !== proto) return false
      if (!q) return true
      return [c.local_addr, c.remote_addr ?? '', c.process ?? '', c.status ?? '', String(c.pid)]
        .join(' ')
        .toLowerCase()
        .includes(q)
    })
  }, [connections, query, proto])

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Filtrele: adres, süreç, durum…"
          className="w-64 rounded-lg border border-slate-700/80 bg-slate-900 px-3 py-1.5 text-sm outline-none placeholder:text-slate-600 focus:border-cyan-500/60"
        />
        <div className="flex rounded-lg border border-slate-700/80 p-0.5">
          {(['all', 'tcp', 'udp'] as const).map((p) => (
            <button
              key={p}
              onClick={() => setProto(p)}
              className={`rounded-md px-3 py-1 text-xs font-medium transition ${
                proto === p ? 'bg-slate-700 text-white' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              {p === 'all' ? 'Tümü' : p.toUpperCase()}
            </button>
          ))}
        </div>
        <span className="ml-auto text-xs text-slate-500">
          {formatNum(filtered.length)} bağlantı
        </span>
      </div>

      <div className="max-h-96 overflow-y-auto rounded-lg border border-slate-800/60">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
            <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
              <th className="px-3 py-2 font-medium">Protokol</th>
              <th className="px-3 py-2 font-medium">Yerel Adres</th>
              <th className="px-3 py-2 font-medium">Uzak Adres</th>
              <th className="px-3 py-2 font-medium">Durum</th>
              <th className="px-3 py-2 font-medium">Süreç</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/50">
            {filtered.slice(0, 300).map((c, i) => (
              <tr key={`${c.proto}-${c.local_addr}-${c.remote_addr ?? ''}-${i}`} className="hover:bg-slate-800/30">
                <td className="px-3 py-1.5">
                  <span
                    className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-bold uppercase ${
                      c.proto === 'udp'
                        ? 'bg-violet-500/10 text-violet-400'
                        : 'bg-cyan-500/10 text-cyan-400'
                    }`}
                  >
                    {c.proto}
                  </span>
                </td>
                <td className="px-3 py-1.5 font-mono text-xs text-slate-300">{c.local_addr}</td>
                <td className="px-3 py-1.5 font-mono text-xs text-slate-400">
                  {c.remote_addr ? (
                    <span title={c.asn}>
                      {c.country && flagEmoji(c.country) !== '' && <span className="mr-1 not-italic">{flagEmoji(c.country)}</span>}
                      {c.remote_addr}
                    </span>
                  ) : (
                    '—'
                  )}
                </td>
                <td className="px-3 py-1.5">
                  {c.status ? (
                    <span
                      className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ring-1 ${
                        STATUS_STYLES[c.status] ?? 'bg-slate-500/10 text-slate-400 ring-slate-500/20'
                      }`}
                    >
                      {c.status}
                    </span>
                  ) : (
                    <span className="text-slate-600">—</span>
                  )}
                </td>
                <td className="px-3 py-1.5">
                  {c.process ? (
                    <span className="text-slate-300">
                      {c.process}
                      {c.pid > 0 && <span className="ml-1 text-[10px] text-slate-600">[{c.pid}]</span>}
                    </span>
                  ) : c.pid > 0 ? (
                    <span className="font-mono text-xs text-slate-500">pid {c.pid}</span>
                  ) : (
                    <span className="text-slate-600">—</span>
                  )}
                </td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr>
                <td colSpan={5} className="px-3 py-8 text-center text-sm text-slate-600">
                  Eşleşen bağlantı yok.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
