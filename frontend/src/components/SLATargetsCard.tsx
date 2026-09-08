import { useCallback, useEffect, useState } from 'react'
import { useDialog } from '../lib/dialog'

interface Target {
  scope: string
  site: string
  agent_uptime_pct: number
  device_health_pct: number
  iface_err_ceiling: number
}

export function SLATargetsCard() {
  const { form, confirm } = useDialog()
  const [targets, setTargets] = useState<Target[]>([])
  const [err, setErr] = useState('')

  const load = useCallback(async () => {
    try {
      const j = await fetch('/api/v1/sla/targets').then((r) => r.json())
      setTargets(j.targets ?? [])
      setErr('')
    } catch {
      setErr('alınamadı')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const edit = async (existing?: Target) => {
    const res = await form({
      title: existing ? `SLA hedefi — ${existing.scope}${existing.site ? '/' + existing.site : ''}` : 'SLA hedefi ekle',
      fields: [
        { key: 'scope', label: 'Kapsam', type: 'select', options: ['global', 'site'], defaultValue: existing?.scope ?? 'global' },
        { key: 'site', label: 'Saha (kapsam=site ise)', type: 'text', defaultValue: existing?.site ?? '' },
        { key: 'agent_uptime_pct', label: 'Agent uptime tabanı %', type: 'number', defaultValue: String(existing?.agent_uptime_pct ?? 0) },
        { key: 'device_health_pct', label: 'Cihaz sağlığı tabanı %', type: 'number', defaultValue: String(existing?.device_health_pct ?? 0) },
        { key: 'iface_err_ceiling', label: 'Arayüz iskarta+hata tavanı (24s)', type: 'number', defaultValue: String(existing?.iface_err_ceiling ?? 0) },
      ],
      confirmLabel: 'Kaydet',
    })
    if (!res) return
    const body = {
      scope: res.scope,
      site: res.scope === 'site' ? res.site : '',
      agent_uptime_pct: Number(res.agent_uptime_pct) || 0,
      device_health_pct: Number(res.device_health_pct) || 0,
      iface_err_ceiling: Number(res.iface_err_ceiling) || 0,
    }
    const r = await fetch('/api/v1/sla/targets', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    if (!r.ok) {
      setErr((await r.text()) || 'kaydedilemedi')
      return
    }
    await load()
  }

  const del = async (t: Target) => {
    if (!(await confirm('SLA hedefi silinsin mi?'))) return
    await fetch(`/api/v1/sla/targets?scope=${encodeURIComponent(t.scope)}&site=${encodeURIComponent(t.site)}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div className="space-y-2 font-mono text-[11px]">
      {err && <p className="text-rose-400">⚠ {err}</p>}
      <p className="text-tui-dim">
        Kurumsal raporda hedef-vs-gerçek + ihlal vurgusu; motor ihlalde <span className="text-rose-400">sla_breach</span> uyarısı üretir.
        Saha-özel hedef, global hedefi geçersiz kılar. 0 = kontrol yok.
      </p>
      <div className="space-y-1">
        {targets.map((t, i) => (
          <div key={i} className="flex flex-wrap items-baseline gap-x-3 border-b border-rule py-1 last:border-0">
            <span className="text-ink-hi">{t.scope}{t.site ? ` / ${t.site}` : ''}</span>
            {t.agent_uptime_pct > 0 && <span className="text-tui-dim">uptime ≥ {t.agent_uptime_pct}%</span>}
            {t.device_health_pct > 0 && <span className="text-tui-dim">sağlık ≥ {t.device_health_pct}%</span>}
            {t.iface_err_ceiling > 0 && <span className="text-tui-dim">iskarta ≤ {t.iface_err_ceiling}</span>}
            <span className="ml-auto flex gap-2">
              <button onClick={() => edit(t)} className="text-rx hover:underline">düzenle</button>
              <button onClick={() => del(t)} className="text-rose-400 hover:underline">sil</button>
            </span>
          </div>
        ))}
      </div>
      <button onClick={() => edit()} className="border border-rule px-2 py-0.5 text-[10px] uppercase text-tui-dim hover:text-ink-hi">
        + hedef ekle
      </button>
    </div>
  )
}
