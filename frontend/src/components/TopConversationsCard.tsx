import { useMemo, useState } from 'react'
import { formatBytes, formatNum } from '../lib/format'
import { usePolledJson } from '../lib/usePolledJson'
import { PanelState } from './PanelState'
import { RangeTabs } from './RangeTabs'
import { TuiTable } from './TuiTable'
import type { TuiColumn } from './TuiTable'
import { IpBadge, RepBadge } from '../lib/enrich'
import type { IPInfo } from '../lib/enrich'

interface RepInfo {
  reputation?: string
  source?: string
}

interface Conversation {
  src: string
  dst: string
  src_port?: number
  dst_port?: number
  proto?: string
  flows: number
  packets: number
  octets: number
  first_seen: number
  last_seen: number
}
interface FlowRow {
  ts: number
  device: string
  src: string
  dst: string
  src_port: number
  dst_port: number
  proto: string
  packets: number
  octets: number
}
interface Actor {
  agent_id: number
  agent_name: string
  process: string
  ip: string
}
interface DrillResponse {
  flows: FlowRow[]
  actors: Actor[]
  src_info?: IPInfo
  dst_info?: IPInfo
  src_rep?: RepInfo
  dst_rep?: RepInfo
}

const WINDOWS = [
  { label: '15 dk', value: '15m' },
  { label: '1 saat', value: '1h' },
  { label: '6 saat', value: '6h' },
  { label: '24 saat', value: '24h' },
] as const
type Window = (typeof WINDOWS)[number]['value']

const SORTS = [
  { label: 'veri', value: 'octets' },
  { label: 'paket', value: 'packets' },
  { label: 'akış', value: 'flows' },
  { label: 'son', value: 'last_seen' },
] as const
type Sort = (typeof SORTS)[number]['value']

function relTime(unix: number): string {
  if (!unix) return '—'
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk`
  const h = Math.floor(m / 60)
  return h < 48 ? `${h} sa` : `${Math.floor(h / 24)} g`
}

export function TopConversationsCard() {
  const [win, setWin] = useState<Window>('15m')
  const [by, setBy] = useState<'pair' | '5tuple'>('pair')
  const [sort, setSort] = useState<Sort>('octets')
  const [drill, setDrill] = useState<Conversation | null>(null)

  const { data, loaded } = usePolledJson<Conversation[]>(
    `/api/v1/flows/conversations?window=${win}&by=${by}&sort=${sort}&limit=25`,
    15_000,
  )
  const rows = useMemo(() => (Array.isArray(data) ? data : []), [data])

  const drillUrl = drill
    ? `/api/v1/flows/conversation?window=${win}&src=${encodeURIComponent(drill.src)}&dst=${encodeURIComponent(drill.dst)}${drill.proto ? `&proto=${drill.proto}` : ''}`
    : null
  const { data: drillData } = usePolledJson<DrillResponse>(drillUrl, 20_000)

  const cols: TuiColumn<Conversation>[] = useMemo(
    () => [
      {
        key: 'src',
        header: by === 'pair' ? 'Uç A' : 'Kaynak',
        sortable: true,
        render: (c) => (
          <span className="text-ink-hi">
            {c.src}
            {c.src_port ? <span className="text-tui-dim">:{c.src_port}</span> : null}
          </span>
        ),
      },
      {
        key: 'dst',
        header: by === 'pair' ? 'Uç B' : 'Hedef',
        sortable: true,
        render: (c) => (
          <span className="text-ink">
            {c.dst}
            {c.dst_port ? <span className="text-tui-dim">:{c.dst_port}</span> : null}
          </span>
        ),
      },
      { key: 'proto', header: 'Proto', width: '4rem', sortable: true, render: (c) => <span className="uppercase text-tui-dim">{c.proto || '—'}</span> },
      { key: 'flows', header: 'Akış', width: '5rem', align: 'right', sortable: true, sortValue: (c) => c.flows, render: (c) => <span className="text-tui-dim">{formatNum(c.flows)}</span> },
      { key: 'packets', header: 'Paket', align: 'right', sortable: true, sortValue: (c) => c.packets, render: (c) => <span className="text-tui-dim">{formatNum(c.packets)}</span> },
      { key: 'octets', header: 'Veri', align: 'right', sortable: true, sortValue: (c) => c.octets, render: (c) => <span className="text-emerald-400">{formatBytes(c.octets)}</span> },
      { key: 'last_seen', header: 'Son', width: '5rem', align: 'right', sortable: true, sortValue: (c) => c.last_seen, render: (c) => <span className="text-tui-dim">{relTime(c.last_seen)}</span> },
    ],
    [by],
  )

  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2 font-mono text-[10px]">
        <RangeTabs ranges={WINDOWS} value={win} onChange={setWin} />
        <div className="flex border border-rule">
          {(['pair', '5tuple'] as const).map((b) => (
            <button
              key={b}
              type="button"
              onClick={() => setBy(b)}
              aria-pressed={by === b}
              className={`px-2 py-0.5 ${by === b ? 'bg-rx text-ground' : 'text-tui-dim hover:text-ink-hi'}`}
            >
              {b === 'pair' ? 'uç çifti' : "5'li"}
            </button>
          ))}
        </div>
        <div className="flex border border-rule">
          {SORTS.map((sopt) => (
            <button
              key={sopt.value}
              type="button"
              onClick={() => setSort(sopt.value)}
              aria-pressed={sort === sopt.value}
              className={`px-2 py-0.5 ${sort === sopt.value ? 'bg-rx text-ground' : 'text-tui-dim hover:text-ink-hi'}`}
            >
              {sopt.label}
            </button>
          ))}
        </div>
        <span className="ml-auto text-tui-dim">Enter → drill-down · ham NetFlow sunucu-tarafı toplanır</span>
      </div>

      {!loaded ? (
        <PanelState kind="loading" />
      ) : rows.length === 0 ? (
        <PanelState
          kind="empty"
          message="Bu pencerede NetFlow yok."
          hint="cihaz(lar)ı NetFlow/IPFIX exporter olarak yapılandırın"
        />
      ) : (
        <TuiTable
          columns={cols}
          rows={rows}
          getKey={(c) => `${c.src}-${c.dst}-${c.src_port ?? ''}-${c.dst_port ?? ''}-${c.proto ?? ''}`}
          filterText={(c) => `${c.src} ${c.dst} ${c.proto ?? ''}`}
          filterLabel="Konuşma filtrele…"
          initialSort={{ key: sort, dir: 'desc' }}
          scrollClass="max-h-80"
          className="border-0"
          onActivate={(c) => setDrill((cur) => (cur && cur.src === c.src && cur.dst === c.dst ? null : c))}
        />
      )}

      {drill && (
        <div className="mt-3 border border-rule bg-panel-2/40 p-3 font-mono text-[11px]">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <span className="inline-flex flex-wrap items-center gap-1">
              {drill.src} <RepBadge reputation={drillData?.src_rep?.reputation} source={drillData?.src_rep?.source} /> <IpBadge info={drillData?.src_info} />
              <span className="mx-1">↔</span>
              {drill.dst} <RepBadge reputation={drillData?.dst_rep?.reputation} source={drillData?.dst_rep?.source} /> <IpBadge info={drillData?.dst_info} />
            </span>
            <button type="button" onClick={() => setDrill(null)} className="ml-auto border border-rule-hi px-2 py-0.5 uppercase text-tui-dim hover:text-ink-hi">
              kapat
            </button>
          </div>
          {drillData?.actors && drillData.actors.length > 0 && (
            <p className="mb-2 text-tui-dim">
              ilişkili:{' '}
              {drillData.actors.map((a, i) => (
                <span key={`${a.agent_id}-${a.process}-${i}`} className="text-ink">
                  {a.agent_name}/{a.process} → {a.ip}
                  {i < drillData.actors.length - 1 ? ' · ' : ''}
                </span>
              ))}
            </p>
          )}
          <div className="max-h-56 overflow-y-auto">
            <table className="w-full">
              <thead>
                <tr className="text-[10px] uppercase text-tui-dim">
                  <th className="py-0.5 text-left">Saat</th>
                  <th className="text-left">Cihaz</th>
                  <th className="text-left">Yön</th>
                  <th className="text-left">Proto</th>
                  <th className="text-right">Paket</th>
                  <th className="text-right">Veri</th>
                </tr>
              </thead>
              <tbody>
                {(drillData?.flows ?? []).map((f, i) => (
                  <tr key={`${f.ts}-${i}`} className="text-ink">
                    <td className="py-0.5 text-tui-dim">{new Date(f.ts * 1000).toLocaleTimeString('tr-TR')}</td>
                    <td className="text-tui-dim">{f.device}</td>
                    <td>
                      {f.src}:{f.src_port} → {f.dst}:{f.dst_port}
                    </td>
                    <td className="uppercase text-tui-dim">{f.proto}</td>
                    <td className="text-right text-tui-dim">{formatNum(f.packets)}</td>
                    <td className="text-right text-emerald-400">{formatBytes(f.octets)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {(drillData?.flows ?? []).length === 0 && <p className="py-3 text-center text-tui-dim">Ham akış bulunamadı (retention penceresi dışı olabilir).</p>}
          </div>
        </div>
      )}
    </div>
  )
}
