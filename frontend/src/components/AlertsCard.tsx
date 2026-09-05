import { useCallback, useEffect, useState } from 'react'
import type { AlertConfig, AlertEvent } from '../types'
import { KIND_LABELS, KIND_STYLES } from '../lib/alertKinds'

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

// daraltılabilir ayar bölümü — native <details>/<summary> (harici kütüphane
// yok, klavye-erişilebilir Enter/Space varsayılan tarayıcı davranışı);
// <fieldset> form kontrollerini semantik olarak grupluyor, bu aynı zamanda
// "kart içinde kart" görsel yinelemesini de ortadan kaldırıyor
function Section({
  title,
  status,
  accent = 'slate',
  children,
}: {
  title: string
  status?: string
  accent?: 'slate' | 'orange'
  children: React.ReactNode
}) {
  const borderCls = accent === 'orange' ? 'border-orange-500/20' : 'border-slate-800'
  return (
    <details open className={`group rounded-lg border ${borderCls} p-3`}>
      <summary className="cursor-pointer list-none text-sm font-medium text-slate-200 marker:content-none">
        <span className="mr-1.5 inline-block text-dim-aa transition group-open:rotate-90">▸</span>
        {title}
        {status && <span className="ml-2 text-xs font-normal text-dim-aa">· {status}</span>}
      </summary>
      <fieldset className="mt-2 space-y-2">{children}</fieldset>
    </details>
  )
}

const inputCls =
  'w-full rounded-lg border border-slate-700/80 bg-slate-900 px-2.5 py-1.5 text-sm text-slate-200 outline-none placeholder:text-dim-aa focus:border-cyan-500/60'
const fieldLabelCls = 'block space-y-1 text-xs text-slate-500'

type ChannelStatus = { last_attempt: number; ok: boolean; error?: string }

// ChannelDot, bir bildirim kanalının son teslim durumunu gösterir (D3).
// Hiç denenmediyse hiçbir şey göstermez.
function ChannelDot({ s }: { s?: ChannelStatus }) {
  if (!s || !s.last_attempt) return null
  const when = new Date(s.last_attempt * 1000).toLocaleString('tr-TR')
  return (
    <span
      title={s.ok ? `son teslim: ${when}` : `hata (${when}): ${s.error}`}
      className={`ml-1.5 inline-block size-2 rounded-full align-middle ${s.ok ? 'bg-emerald-400' : 'bg-rose-500'}`}
    />
  )
}

export function AlertsCard({ events }: { events: AlertEvent[] }) {
  const [cfg, setCfg] = useState<AlertConfig | null>(null)
  const [loadError, setLoadError] = useState(false)
  const [forbidden, setForbidden] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState(false)
  const [savedAt, setSavedAt] = useState('')
  const [portsDropped, setPortsDropped] = useState(0)
  const [notify, setNotify] = useState<Record<string, ChannelStatus>>({})
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    fetch('/api/alerts')
      .then((r) => {
        if (r.status === 403) {
          setForbidden(true)
          return null
        }
        if (!r.ok) throw new Error(String(r.status))
        return r.json()
      })
      .then((c) => {
        if (c) {
          setCfg(c)
          setLoadError(false)
        }
      })
      .catch(() => setLoadError(true))
    fetch('/api/alerts/status')
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => d?.channels && setNotify(d.channels))
      .catch(() => {})
  }, [])

  const testChannels = useCallback(async () => {
    setTesting(true)
    try {
      const res = await fetch('/api/alerts/test', { method: 'POST' })
      const d = await res.json()
      if (d?.channels) setNotify(d.channels)
    } catch {
      /* durum güncellenmez */
    } finally {
      setTesting(false)
    }
  }, [])

  const save = useCallback(async () => {
    if (!cfg) return
    setSaving(true)
    setSaveError(false)
    try {
      const res = await fetch('/api/alerts', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg),
      })
      if (!res.ok) throw new Error(String(res.status))
      setSavedAt(new Date().toLocaleTimeString('tr-TR'))
    } catch {
      setSaveError(true)
    } finally {
      setSaving(false)
    }
  }, [cfg])

  const eventFeed = (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Olay Akışı</h3>
      {events.length === 0 ? (
        <p className="py-8 text-center text-sm text-dim-aa">Henüz uyarı yok.</p>
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
                <span className="ml-auto font-mono text-[10px] text-dim-aa">
                  {new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}
                </span>
              </div>
              <p className="mt-1 text-sm text-slate-300">{e.message}</p>
            </li>
          ))}
        </ul>
      )}
    </div>
  )

  if (!cfg) {
    const notice = forbidden ? (
      <p className="text-sm text-dim-aa">
        Uyarı eşikleri ve bildirim kanalları yalnızca <span className="text-slate-300">yöneticilere</span> görünür.
      </p>
    ) : loadError ? (
      <p className="text-sm text-rose-400">⚠ ayarlar alınamadı — sayfayı yenileyin</p>
    ) : (
      <p className="text-sm text-dim-aa">Ayarlar yükleniyor…</p>
    )
    return (
      <div className="grid gap-5 lg:grid-cols-2">
        {eventFeed}
        <div className="flex items-start pt-7">{notice}</div>
      </div>
    )
  }

  const set = <K extends keyof AlertConfig>(k: K, v: AlertConfig[K]) =>
    setCfg((c) => (c ? { ...c, [k]: v } : c))

  const siem = cfg.notifiers.siem ?? { enabled: false, format: '' as const, transport: '' as const, target: '', token: '', insecure: false }
  const patchSiem = (p: Partial<typeof siem>) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, siem: { ...siem, ...p } } })

  return (
    <div className="grid gap-5 lg:grid-cols-2">
      {eventFeed}

      {/* ayar formu — @container: iç grid'ler kartın kendi genişliğine göre
          kırılıyor (viewport'a göre değil), sayfa yerleşimi ne olursa olsun */}
      <div className="@container space-y-3">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-500">Ayarlar</h3>

        <div className="flex items-center gap-4">
          <Toggle checked={cfg.enabled} onChange={(v) => set('enabled', v)} label="Uyarılar açık" />
          <label className="flex items-center gap-1.5 text-xs text-slate-500">
            soğuma
            <input
              type="number"
              min={1}
              value={cfg.cooldown_min}
              onChange={(e) => set('cooldown_min', +e.target.value || 1)}
              className={inputCls + ' !w-16'}
            />
            dk
          </label>
        </div>

        <p className="text-[10.5px] font-semibold uppercase tracking-wider text-dim-aa">
          Yerel yakalama gerektirir <span className="font-normal normal-case text-dim-aa">— hub'ın kendi ağ yakalaması aktifken tetiklenir</span>
        </p>

        <Section title="Bant genişliği zirvesi" status={cfg.bandwidth.enabled ? 'açık' : 'kapalı'}>
          <Toggle checked={cfg.bandwidth.enabled} onChange={(v) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, enabled: v } })} label="Bant genişliği zirvesi" />
          <div className="grid grid-cols-1 gap-2 @sm:grid-cols-3 text-xs text-slate-500">
            <label className={fieldLabelCls}>indirme (Mbps)
              <input type="number" min={1} value={cfg.bandwidth.in_mbps} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, in_mbps: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>gönderme (Mbps)
              <input type="number" min={1} value={cfg.bandwidth.out_mbps} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, out_mbps: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>süre (sn)
              <input type="number" min={1} value={cfg.bandwidth.seconds} onChange={(e) => setCfg((c) => c && { ...c, bandwidth: { ...c.bandwidth, seconds: +e.target.value || 1 } })} className={inputCls} />
            </label>
          </div>
        </Section>

        <Section title="Şüpheli portlar" status={cfg.ports.enabled ? 'açık' : 'kapalı'}>
          <Toggle checked={cfg.ports.enabled} onChange={(v) => setCfg((c) => c && { ...c, ports: { ...c.ports, enabled: v } })} label="Şüpheli portlar" />
          <label className={fieldLabelCls}>portlar (virgülle ayrık)
            <input
              value={cfg.ports.ports.join(', ')}
              onChange={(e) => {
                const raw = e.target.value.split(',').map((s) => s.trim()).filter(Boolean)
                const nums = raw.map((s) => +s).filter((n) => n > 0)
                setPortsDropped(raw.length - nums.length)
                setCfg((c) => c && { ...c, ports: { ...c.ports, ports: nums } })
              }}
              placeholder="23, 4444, 1337"
              className={inputCls}
            />
          </label>
          {portsDropped > 0 && (
            <p className="text-[10px] text-amber-400">⚠ {portsDropped} geçersiz girdi yoksayıldı — port bir sayı olmalı</p>
          )}
        </Section>

        <Section title="Yeni süreç ağa çıkınca" status={cfg.new_proc.enabled ? 'açık' : 'kapalı'}>
          <Toggle checked={cfg.new_proc.enabled} onChange={(v) => setCfg((c) => c && { ...c, new_proc: { ...c.new_proc, enabled: v } })} label="Yeni süreç ağa çıkınca" />
          <label className={fieldLabelCls}>yoksayılacak süreçler (virgülle ayrık)
            <input
              value={cfg.new_proc.ignore.join(', ')}
              onChange={(e) => setCfg((c) => c && { ...c, new_proc: { ...c.new_proc, ignore: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) } })}
              placeholder="yoksayılacak süreçler"
              className={inputCls}
            />
          </label>
        </Section>

        <Section title="Yeni hedefle trafik" status={cfg.new_target.enabled ? 'açık' : 'kapalı'}>
          <Toggle checked={cfg.new_target.enabled} onChange={(v) => setCfg((c) => c && { ...c, new_target: { ...c.new_target, enabled: v } })} label="Yeni hedefle trafik" />
          <label className={fieldLabelCls}>en az transfer (MB)
            <input type="number" min={0} value={cfg.new_target.min_total_mb} onChange={(e) => setCfg((c) => c && { ...c, new_target: { ...c.new_target, min_total_mb: +e.target.value || 0 } })} className={inputCls} />
          </label>
        </Section>

        <Section title="İstatistiksel anomali (z-skoru baseline)" status={cfg.anomaly.enabled ? 'açık' : 'kapalı'}>
          <Toggle
            checked={cfg.anomaly.enabled}
            onChange={(v) => setCfg((c) => c && { ...c, anomaly: { ...c.anomaly, enabled: v } })}
            label="İstatistiksel anomali (z-skoru baseline)"
          />
          <div className="grid grid-cols-1 gap-2 @sm:grid-cols-3 text-xs text-slate-500">
            <label className={fieldLabelCls}>hassasiyet (z-skoru)
              <input type="number" min={0.5} step={0.5} value={cfg.anomaly.sensitivity} onChange={(e) => setCfg((c) => c && { ...c, anomaly: { ...c.anomaly, sensitivity: +e.target.value || 3 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>min örnek
              <input type="number" min={1} value={cfg.anomaly.min_samples} onChange={(e) => setCfg((c) => c && { ...c, anomaly: { ...c.anomaly, min_samples: +e.target.value || 1 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>pencere (dk)
              <input type="number" min={1} value={cfg.anomaly.window_min} onChange={(e) => setCfg((c) => c && { ...c, anomaly: { ...c.anomaly, window_min: +e.target.value || 1 } })} className={inputCls} />
            </label>
          </div>
        </Section>

        <p className="pt-1 text-[10.5px] font-semibold uppercase tracking-wider text-dim-aa">
          Filo/cihaz tabanlı <span className="font-normal normal-case text-dim-aa">— hub yakalaması gerekmez, cihaz poll'undan gelir</span>
        </p>

        <Section title="FortiGate: VPN / SD-WAN" accent="orange">
          <Toggle
            checked={cfg.forti.vpn_down}
            onChange={(v) => setCfg((c) => c && { ...c, forti: { ...c.forti, vpn_down: v } })}
            label="FortiGate: VPN tüneli/kullanıcısı down"
          />
          <div className="grid grid-cols-1 gap-2 @sm:grid-cols-2 text-xs text-slate-500">
            <label className={fieldLabelCls}>sd-wan gecikme eşiği (ms)
              <input type="number" min={0} value={cfg.forti.sdwan_latency_ms} onChange={(e) => setCfg((c) => c && { ...c, forti: { ...c.forti, sdwan_latency_ms: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>sd-wan jitter eşiği (ms)
              <input type="number" min={0} value={cfg.forti.sdwan_jitter_ms} onChange={(e) => setCfg((c) => c && { ...c, forti: { ...c.forti, sdwan_jitter_ms: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>sd-wan kayıp eşiği (%)
              <input type="number" min={0} max={100} value={cfg.forti.sdwan_loss_pct} onChange={(e) => setCfg((c) => c && { ...c, forti: { ...c.forti, sdwan_loss_pct: +e.target.value || 0 } })} className={inputCls} />
            </label>
            <label className={fieldLabelCls}>maks. oturum (0=kapalı)
              <input type="number" min={0} value={cfg.forti.max_sessions} onChange={(e) => setCfg((c) => c && { ...c, forti: { ...c.forti, max_sessions: +e.target.value || 0 } })} className={inputCls} />
            </label>
          </div>
          <p className="text-[10.5px] text-dim-aa">Eşik 0 ise o kontrol kapalıdır (VPN down hariç, ayrı toggle).</p>
        </Section>

        <Section title="Bildirim Kanalları">
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={testChannels}
              disabled={testing}
              className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-2.5 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20 disabled:opacity-40"
            >
              {testing ? 'Test ediliyor…' : 'Kanalları Test Et'}
            </button>
            <span className="text-[10.5px] text-dim-aa">kaydedilmiş yapılandırmaya sınama uyarısı gönderir</span>
          </div>
          <Toggle checked={cfg.notifiers.desktop} onChange={(v) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, desktop: v } })} label="Masaüstü bildirimi" />
          <label className={fieldLabelCls}><span>Generic webhook URL<ChannelDot s={notify.generic} /></span>
            <input value={cfg.notifiers.generic_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, generic_url: e.target.value } })} placeholder="https://…" className={inputCls} />
          </label>
          <label className={fieldLabelCls}><span>Discord webhook URL<ChannelDot s={notify.discord} /></span>
            <input value={cfg.notifiers.discord_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, discord_url: e.target.value } })} placeholder="https://discord.com/api/webhooks/…" className={inputCls} />
          </label>
          <label className={fieldLabelCls}><span>Slack webhook URL<ChannelDot s={notify.slack} /></span>
            <input value={cfg.notifiers.slack_url} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, slack_url: e.target.value } })} placeholder="https://hooks.slack.com/…" className={inputCls} />
          </label>
          <div className="grid grid-cols-1 gap-2 @sm:grid-cols-2">
            <label className={fieldLabelCls}><span>Telegram bot token<ChannelDot s={notify.telegram} /></span>
              <input value={cfg.notifiers.telegram_token} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, telegram_token: e.target.value } })} placeholder="123456:ABC-…" className={inputCls} />
            </label>
            <label className={fieldLabelCls}>Telegram chat ID
              <input value={cfg.notifiers.telegram_chat_id} onChange={(e) => setCfg((c) => c && { ...c, notifiers: { ...c.notifiers, telegram_chat_id: e.target.value } })} placeholder="-100…" className={inputCls} />
            </label>
          </div>
        </Section>

        <Section title="SIEM / ITSM Bağlayıcı" status={siem.enabled ? 'açık' : 'kapalı'}>
          <div className="flex items-center gap-1.5">
            <Toggle checked={siem.enabled} onChange={(v) => patchSiem({ enabled: v })} label="Uyarıları SIEM'e ilet (CEF/LEEF/JSON)" />
            <ChannelDot s={notify.siem} />
          </div>
          <div className="grid grid-cols-1 gap-2 @sm:grid-cols-2">
            <label className={fieldLabelCls}>Mesaj formatı
              <select value={siem.format} onChange={(e) => patchSiem({ format: e.target.value as typeof siem.format })} className={inputCls}>
                <option value="">CEF (ArcSight)</option>
                <option value="cef">CEF (ArcSight)</option>
                <option value="leef">LEEF (QRadar)</option>
                <option value="json">JSON</option>
                <option value="text">Düz metin (klasik syslog)</option>
              </select>
            </label>
            <label className={fieldLabelCls}>Taşıma
              <select value={siem.transport} onChange={(e) => patchSiem({ transport: e.target.value as typeof siem.transport })} className={inputCls}>
                <option value="">syslog UDP</option>
                <option value="syslog-udp">syslog UDP</option>
                <option value="syslog-tcp">syslog TCP</option>
                <option value="http">HTTP POST</option>
              </select>
            </label>
          </div>
          <label className={fieldLabelCls}>Hedef
            <input value={siem.target} onChange={(e) => patchSiem({ target: e.target.value })} placeholder={siem.transport === 'http' ? 'https://siem.kurum.local/services/collector/raw' : 'siem.kurum.local:514'} className={inputCls} />
          </label>
          {siem.transport === 'http' && (
            <>
              <label className={fieldLabelCls}>Authorization başlığı (opsiyonel)
                <input value={siem.token} onChange={(e) => patchSiem({ token: e.target.value })} placeholder="ör. Splunk <HEC-token>" className={inputCls} />
              </label>
              <Toggle checked={siem.insecure} onChange={(v) => patchSiem({ insecure: v })} label="TLS doğrulamasını atla (self-signed toplayıcı)" />
            </>
          )}
          <p className="text-[10.5px] text-dim-aa">CEF/LEEF önem: port 9 · vpn_down 8 · anomali/sdwan 6 · bant/oturum 5 · süreç/hedef 4. syslog facility local0.</p>
        </Section>

        <div className="flex items-center gap-3">
          <button
            onClick={save}
            disabled={saving}
            className="rounded-lg bg-cyan-700 px-4 py-1.5 text-sm font-semibold text-white transition enabled:hover:bg-cyan-400 enabled:hover:text-slate-950 disabled:opacity-40"
          >
            {saving ? 'Kaydediliyor…' : 'Ayarları Kaydet'}
          </button>
          {saveError ? (
            <span className="text-xs text-rose-400">⚠ kaydedilemedi, tekrar deneyin</span>
          ) : (
            savedAt && <span className="text-xs text-emerald-400">kaydedildi · {savedAt}</span>
          )}
        </div>
      </div>
    </div>
  )
}
