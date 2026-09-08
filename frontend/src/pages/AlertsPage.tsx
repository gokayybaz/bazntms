import { useMemo, useState } from 'react'
import type { AlertEvent } from '../types'
import { formatNum } from '../lib/format'
import { usePolledJson } from '../lib/usePolledJson'
import type { Incident } from '../components/IncidentsPanel'
import { AlertsCard } from '../components/AlertsCard'
import { AlertEventsPanel } from '../components/AlertEventsPanel'
import { EventsPanel } from '../components/EventsPanel'
import { IncidentsPanel } from '../components/IncidentsPanel'
import { KIND_LABELS, KIND_STYLES } from '../lib/alertKinds'
import { Panel } from '../components/Panel'

const SUBTABS = [
  { v: 'alarmlar', label: 'Alarmlar' },
  { v: 'olaylar', label: 'Olaylar' },
  { v: 'akis', label: 'Olay Akışı' },
] as const
type SubTab = (typeof SUBTABS)[number]['v']

export function AlertsPage({ alertEvents }: { alertEvents: AlertEvent[] }) {
  const [tab, setTab] = useState<SubTab>('alarmlar')
  // açık incident sayısı → "Olaylar" sekmesinde rozet (alarm bağlamından erişim)
  const { data: incData } = usePolledJson<{ incidents: Incident[] }>('/api/v1/incidents?status=open&limit=200', 20_000)
  const openIncidents = incData?.incidents?.length ?? 0
  const byKind = useMemo(() => {
    const counts = new Map<string, number>()
    for (const e of alertEvents) counts.set(e.kind, (counts.get(e.kind) ?? 0) + 1)
    return [...counts.entries()].sort((a, b) => b[1] - a[1])
  }, [alertEvents])

  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex flex-wrap items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Uyarılar</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">
          yaşam döngüsü (kabul / çöz / not) · korelasyon · bakım pencereleri · eşik &amp; bildirim ayarları
        </span>
        <span className="ml-auto text-[10px] text-tui-dim">{formatNum(alertEvents.length)} olay</span>
      </div>

      <nav className="flex flex-wrap border border-rule bg-ground text-[11px]">
        {SUBTABS.map((t) => (
          <button
            key={t.v}
            type="button"
            onClick={() => setTab(t.v)}
            aria-pressed={tab === t.v}
            className={`border-r border-rule px-3 py-1 uppercase tracking-[0.04em] transition ${
              tab === t.v ? 'bg-rx text-ground' : 'text-tui-dim hover:bg-panel-2 hover:text-ink-hi'
            }`}
          >
            {t.label}
            {t.v === 'olaylar' && openIncidents > 0 && (
              <span className={`ml-1.5 font-bold ${tab === t.v ? 'text-ground' : 'text-rose-400'}`}>{openIncidents}</span>
            )}
          </button>
        ))}
      </nav>

      {tab === 'alarmlar' ? (
        <>
          {byKind.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {byKind.map(([kind, count]) => (
                <span
                  key={kind}
                  className={`inline-flex items-center gap-1.5 border px-2 py-0.5 text-[10px] uppercase tracking-[0.04em] ${
                    KIND_STYLES[kind] ?? 'border-rule-hi text-tui-dim'
                  }`}
                >
                  {KIND_LABELS[kind] ?? kind}
                  <span className="font-bold">{formatNum(count)}</span>
                </span>
              ))}
            </div>
          )}

          <AlertEventsPanel />

          <Panel title="Eşikler &amp; Bildirim Kanalları" right={<span className="text-[10px] text-tui-dim">yalnız yönetici</span>}>
            <AlertsCard events={alertEvents} />
          </Panel>
        </>
      ) : tab === 'olaylar' ? (
        <Panel title="Olaylar (Incident)" right={<span className="text-[10px] text-tui-dim">korele uyarı kümeleri</span>}>
          <IncidentsPanel />
        </Panel>
      ) : (
        <Panel title="Olay Akışı" right={<span className="text-[10px] text-tui-dim">ham gözlem · uyarı değil</span>}>
          <EventsPanel />
        </Panel>
      )}
    </div>
  )
}
