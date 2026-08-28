import { useCallback, useEffect, useState } from 'react'
import type { AlertConfig, AlertEvent } from '../types'

const KIND_STYLES: Record<string, string> = {
  bw: 'bg-amber-500/10 text-amber-400 ring-amber-500/20',
  port: 'bg-rose-500/10 text-rose-400 ring-rose-500/20',
  proc: 'bg-sky-500/10 text-sky-400 ring-sky-500/20',
  target: 'bg-violet-500/10 text-violet-400 ring-violet-500/20',
}
const KIND_LABELS: Record<string, string> = {
  bw: 'bant genişliği',
  port: 'şüpheli port',
  proc: 'yeni süreç',
  target: 'yeni hedef',
}

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-300 select-none">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="size-4 accent-cyan-500"
      />
      {label}
    </label>
  )
}

const inputCls =
  'w-full rounded-lg border border-slate-700/80 bg-slate-900 px-2.5 py-1.5 text-sm text-slate-200 outline-none placeholder:text-slate-600 focus:border-cyan-500/60'

export function AlertsCard({ events }: { events: AlertEvent[] }) {
  const [cfg, setCfg] = useState<AlertConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [savedAt, setSavedAt] = useState('')

  useEffect(() => {
    fetch('/api/alerts')
      .then((r) => r.json())
      .then(setCfg)
      .catch(() => {})
  }, [])

  const save = useCallback(async () => {
    if (!cfg) return
    setSaving(true)
    try {
      await fetch('/api/alerts', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg),
      })
      setSavedAt(new Date().toLocaleTimeString('tr-TR'))
    } catch {
      /* yoksay */
    } finally {
      setSaving(false)
    }
  }, [cfg])

  if (!cfg) {
    return <p className="py-6 text-center text-sm text-slate-600">Ayarlar yükleniyor…</p>
  }

  const set = <K extends keyof AlertConfig>(k: K, v: AlertConfig[K]) =>
    setCfg((c) => (c ? { ...c, [k]: v } : c))

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {/* olar akisi */}
      <div>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Olay Akışı</h3>
        {events.length === 0 ? (
          <p className="py-8 text-center text-sm text-slate-600">Henüz uyarı yok.</p>
        ) : (
          <ul className="max-h-96 space-y-2 overflow-y-auto pr-1">
            {events.map((e) => (
              <li key={e.id} className="rounded-lg border border-slate-800 bg-slate-900/50 px-3 py-2">
                <div className="flex items-center gap-2">
                  <span
                    className={`rounded px-1.5 py-0.5 text-[10px] font-semibold ring-1 ${KIND_STYLES[e.kind] ?? 'bg-slate-500/10 text-slate-400 ring-slate-500/20'}`}
                  >
                    {KIND_LABELS[e.kind] ?? e.kind}
                  </span>
                  <span className="ml-auto font-mono text-[10px] text-slate-600">
                    {new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}
                  </span>
                </div>
                <p className="mt-1 text-sm text-slate-300">{e.message}</p>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* ayar formu */}
      <div className="space-y-3">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-500">Ayarlar</h3>

        <div className="flex items-center gap-4">
          <Toggle checked={cfg.enabled} onChange={(v) => set('enabled', v)} label="Uyarılar açık" />
          <div className="flex items-center gap-1.5 text-xs text-slate-500">
            soğuma
            <input
              type="number"
              min={1}
              value={cfg.cooldown_min}
              onChange={(e) => set('cooldown_min', +e.target.value || 1)}
              className={inputCls + ' !w-16'}
            />
            dk
          </div>
        </div>

        <div className="rounded-lg border border-slate-800 p-3 space-y-2">
          <Toggle checked={cfg.bandwidth.enabled} onChange={(v) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, enabled: v } })} label="Bant genişliği zirvesi" />
          <div className="grid grid-cols-3 gap-2 text-xs text-slate-500">
            <label className="space-y-1">indirme (Mbps)
              <input type="number" min={1} value={cfg.bandwidth.in_mbps} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, in_mbps: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className="space-y-1">gönderme (Mbps)
              <input type="number" min={1} value={cfg.bandwidth.out_mbps} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, out_mbps: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className="space-y-1">süre (sn)
              <input type="number" min={1} value={cfg.bandwidth.seconds} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, seconds: +e.target.value || 1 } })} className={inputCls} />
            </label>
          </div>
        </div>

        <div className="rounded-lg border border-slate-800 p-3 space-y-2">
          <Toggle checked={cfg.ports.enabled} onChange={(v) => setCfg((c) => c && { ...c, ports: { ...c.ports, enabled: v } })} label="Şüpheli portlar" />
          <input
            value={cfg.ports.ports.join(', ')}
            onChange={(e) => setCfg((c) => c && { ...c, ports: { ...c.ports, ports: e.target.value.split(',').map((s) => +s.trim()).filter((n) => n > 0) } })}
            placeholder="23, 4444, 1337"
            className={inputCls}
          />
        </div>

        <div className="rounded-lg border border-slate-800 p-3 space-y-2">
          <Toggle checked={cfg.new_proc.enabled} onChange={(v) => setCfg((c) => c && { ...c, new_proc: { ...c.new_proc, enabled: v } })} label="Yeni süreç ağa çıkınca" />
          <input
            value={cfg.new_proc.ignore.join(', ')}
            onChange={(e) => setCfg((c) => c && { ...c, new_proc: { ...c.new_proc, ignore: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) } })}
            placeholder="yoksayılacak süreçler"
            className={inputCls}
          />
        </div>

        <div className="rounded-lg border border-slate-800 p-3 space-y-2">
          <Toggle checked={cfg.new_target.enabled} onChange={(v) => setCfg((c) => c && { ...c, new_target: { ...c.new_target, enabled: v } })} label="Yeni hedefle trafik" />
          <label className="block text-xs text-slate-500">en az transfer (MB)
            <input type="number" min={0} value={cfg.new_target.min_total_mb} onChange={(e) => setCfg((c) => c && { ...c, new_target: { ...c.new_target, min_total_mb: +e.target.value || 0 } })} className={inputCls} />
          </label>
        </div>

        <div className="rounded-lg border border-slate-800 p-3 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Bildirim Kanalları</p>
          <Toggle checked={cfg.notifiers.desktop} onChange={(v) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, desktop: v } })} label="Masaüstü bildirimi" />
          <input value={cfg.notifiers.generic_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, generic_url: e.target.value } })} placeholder="Generic webhook URL" className={inputCls} />
          <input value={cfg.notifiers.discord_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, discord_url: e.target.value } })} placeholder="Discord webhook URL" className={inputCls} />
          <input value={cfg.notifiers.slack_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, slack_url: e.target.value } })} placeholder="Slack webhook URL" className={inputCls} />
          <div className="grid grid-cols-2 gap-2">
            <input value={cfg.notifiers.telegram_token} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, telegram_token: e.target.value } })} placeholder="Telegram bot token" className={inputCls} />
            <input value={cfg.notifiers.telegram_chat_id} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, telegram_chat_id: e.target.value } })} placeholder="Telegram chat ID" className={inputCls} />
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={save}
            disabled={saving}
            className="rounded-lg bg-cyan-600 px-4 py-1.5 text-sm font-semibold text-white transition enabled:hover:brightness-110 disabled:opacity-40"
          >
            {saving ? 'Kaydediliyor…' : 'Ayarları Kaydet'}
          </button>
          {savedAt && <span className="text-xs text-emerald-400">kaydedildi · {savedAt}</span>}
        </div>
      </div>
    </div>
  )
}
