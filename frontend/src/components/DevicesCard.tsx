import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { formatBits } from '../lib/format'
import { FortiPanel } from './FortiPanel'

interface Device {
  id: number
  name: string
  host: string
  kind: string
  site: string
  vendor: string // snmp | fortigate
  snmp_version: number
  api_url: string
  vdom: string
  poll_seconds: number
  enabled: boolean
  sys_name: string
  sys_descr: string
  last_poll: number
  last_error: string
}

interface IfaceRate {
  if_index: number
  name: string
  alias: string
  speed: number
  oper_status: number
  rx_bps: number
  tx_bps: number
  in_errors: number
  out_errors: number
  in_discards: number
  out_discards: number
}

export function DevicesCard({ refreshKey }: { refreshKey: number }) {
  const [devices, setDevices] = useState<Device[]>([])
  const [loaded, setLoaded] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [error, setError] = useState('')
  const [detail, setDetail] = useState<{ id: number; name: string; ifaces: IfaceRate[] } | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/devices')
      if (res.status === 401) return
      setDevices(await res.json())
      setLoaded(true)
    } catch {
      /* yoksay */
    }
  }, [])

  useEffect(() => {
    load()
  }, [load, refreshKey])

  const remove = useCallback(async (id: number) => {
    await fetch(`/api/v1/devices/${id}`, { method: 'DELETE' }).catch(() => {})
    load()
  }, [load])

  const showIfaces = useCallback(async (id: number, name: string) => {
    const res = await fetch(`/api/v1/devices/${id}/interfaces`)
    const data = await res.json()
    setDetail({ id, name, ifaces: data })
  }, [])

  // vendor rozeti ve detay düğmesi etiketi
  const vendorBadge = (d: Device) =>
    d.vendor === 'fortigate' ? (
      <span className="rounded border border-orange-500/40 bg-orange-500/10 px-1.5 py-0.5 font-mono text-[9px] uppercase text-orange-300">
        rest api
      </span>
    ) : (
      <span className="rounded border border-slate-700 px-1.5 py-0.5 font-mono text-[9px] uppercase text-slate-500">
        snmp v{d.snmp_version === 3 ? '3' : '2c'}
      </span>
    )
  const detailLabel = (d: Device) => (d.vendor === 'fortigate' ? 'fortigate detay' : 'arayüzler')

  return (
    <div>
      <div className="mb-3 flex items-center gap-2">
        <button
          onClick={() => setShowForm((v) => !v)}
          className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          {showForm ? 'Vazgeç' : '+ Cihaz Ekle'}
        </button>
        <span className="text-[11px] text-slate-500">
          SNMPv2c/v3 veya FortiGate REST API ile yoklanır · kimlik bilgileri AES-GCM kasada şifreli
        </span>
      </div>

      {showForm && <DeviceForm onAdded={() => { setShowForm(false); load() }} onError={setError} />}
      {error && <p className="mb-3 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-400">{error}</p>}

      {loaded && devices.length === 0 ? (
        <p className="py-6 text-center text-sm text-slate-600">Cihaz yok — SNMP poller veya FortiGate REST için cihaz ekleyin.</p>
      ) : (
        <div className="space-y-2">
          {devices.map((d) => (
            <div key={d.id} className="rounded-md border border-slate-800 bg-slate-900/50 px-3.5 py-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[10px] uppercase text-slate-400">{d.kind}</span>
                <Link to={`/cihazlar/${d.id}`} className="font-mono text-sm font-semibold text-cyan-300 hover:text-cyan-200 hover:underline">
                  {d.name}
                </Link>
                <span className="font-mono text-xs text-slate-500">{d.host}</span>
                {d.site && <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[9px] text-slate-400">{d.site}</span>}
                {vendorBadge(d)}
                <span className="ml-auto font-mono text-[10px] text-slate-600">
                  {d.last_poll > 0 ? `son poll: ${new Date(d.last_poll * 1000).toLocaleTimeString('tr-TR')}` : 'hiç poll edilmedi'}
                </span>
                <button
                  onClick={() => showIfaces(d.id, d.name)}
                  className="rounded-md border border-slate-700 px-2 py-0.5 text-[11px] text-slate-400 transition hover:border-slate-500 hover:text-slate-200"
                >
                  {detailLabel(d)}
                </button>
                <button
                  onClick={() => remove(d.id)}
                  title="Cihazı sil"
                  className="rounded-md border border-rose-500/30 px-2 py-0.5 text-[11px] text-rose-400/80 transition hover:border-rose-500/60 hover:text-rose-300"
                >
                  sil
                </button>
              </div>
              {d.sys_descr && (
                <p className="mt-1 truncate text-[11px] text-slate-600" title={d.sys_descr}>{d.sys_descr}</p>
              )}
              {d.vendor === 'fortigate' && (d.api_url || d.vdom) && (
                <p className="mt-1 font-mono text-[10px] text-slate-600">
                  {d.api_url}
                  {d.vdom && <span className="ml-2 rounded bg-slate-800 px-1.5 py-0.5 text-slate-400">vdom: {d.vdom}</span>}
                </p>
              )}
              {d.last_error && (
                <p className="mt-1 truncate font-mono text-[11px] text-rose-400/80" title={d.last_error}>⚠ {d.last_error}</p>
              )}
              {detail?.id === d.id && d.vendor === 'fortigate' && (
                <FortiPanel deviceId={d.id} />
              )}
              {detail?.id === d.id && d.vendor !== 'fortigate' && (
                <div className="mt-2 max-h-64 overflow-y-auto rounded border border-slate-800">
                  <table className="w-full text-xs">
                    <thead className="bg-slate-900/80">
                      <tr className="text-left text-[10px] uppercase text-slate-500">
                        <th className="px-2 py-1">Arayüz</th><th className="px-2 py-1">Durum</th>
                        <th className="px-2 py-1 text-right">↓</th><th className="px-2 py-1 text-right">↑</th>
                        <th className="px-2 py-1 text-right">Hata (in/out)</th>
                        <th className="px-2 py-1 text-right" title="ifInDiscards / ifOutDiscards — kuyruk taşması, QoS drop">Atılan (in/out)</th>
                      </tr>
                    </thead>
                    <tbody>
                      {detail.ifaces.map((i) => (
                        <tr key={i.if_index} className="border-t border-slate-800/60">
                          <td className="px-2 py-1 font-mono text-slate-300">{i.name || `if${i.if_index}`}</td>
                          <td className="px-2 py-1">
                            <span className={`font-mono text-[10px] ${i.oper_status === 1 ? 'text-emerald-400' : 'text-slate-500'}`}>
                              {i.oper_status === 1 ? 'up' : 'down'}
                            </span>
                          </td>
                          <td className="px-2 py-1 text-right font-mono text-cyan-300/90">{formatBits(i.rx_bps)}</td>
                          <td className="px-2 py-1 text-right font-mono text-violet-300/90">{formatBits(i.tx_bps)}</td>
                          <td className={`px-2 py-1 text-right font-mono ${i.in_errors + i.out_errors > 0 ? 'text-amber-400/90' : 'text-slate-500'}`}>{i.in_errors}/{i.out_errors}</td>
                          <td className={`px-2 py-1 text-right font-mono ${i.in_discards + i.out_discards > 0 ? 'text-amber-400/90' : 'text-slate-500'}`}>{i.in_discards}/{i.out_discards}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function DeviceForm({ onAdded, onError }: { onAdded: () => void; onError: (s: string) => void }) {
  const [form, setForm] = useState({
    name: '', host: '', kind: 'router', site: '', vendor: 'snmp', snmp_version: 2,
    community: '', v3_user: '', v3_auth_proto: 'SHA', v3_auth_pass: '',
    v3_priv_proto: 'AES', v3_priv_pass: '',
    api_url: '', api_token: '', api_verify_tls: true, vdom: 'root',
    poll_seconds: 60,
  })
  const set = (k: string, v: string | number | boolean) => setForm((f) => ({ ...f, [k]: v }))
  const inputCls =
    'w-full rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-sm outline-none placeholder:text-slate-600 focus:border-cyan-500/60'

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const res = await fetch('/api/v1/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        const t = await res.text()
        throw new Error(t)
      }
      onAdded()
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <form onSubmit={submit} className="mb-3 space-y-2 rounded-md border border-slate-800 bg-slate-900/60 p-3">
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <input value={form.name} onChange={(e) => set('name', e.target.value)} placeholder="ad * (core-sw)" className={inputCls} />
        <input value={form.host} onChange={(e) => set('host', e.target.value)} placeholder="host/ip *" className={inputCls} />
        <select value={form.kind} onChange={(e) => set('kind', e.target.value)} className={inputCls}>
          {['router', 'switch', 'firewall', 'ap', 'other'].map((k) => <option key={k} value={k}>{k}</option>)}
        </select>
        <select value={form.vendor} onChange={(e) => set('vendor', e.target.value)} className={inputCls}>
          <option value="snmp">SNMP</option>
          <option value="fortigate">FortiGate (REST API)</option>
        </select>
        <input value={form.site} onChange={(e) => set('site', e.target.value)} placeholder="site (RBAC scope — ör. sube-a)" className={inputCls} />
      </div>

      {form.vendor === 'fortigate' ? (
        <div className="space-y-2">
          <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
            <input value={form.api_url} onChange={(e) => set('api_url', e.target.value)} placeholder="api_url * (https://10.0.0.1)" className={inputCls} />
            <input value={form.vdom} onChange={(e) => set('vdom', e.target.value)} placeholder="vdom (root veya all)" className={inputCls} />
          </div>
          <input type="password" value={form.api_token} onChange={(e) => set('api_token', e.target.value)} placeholder="REST API token * (kasada şifrelenir; read-only profil önerilir)" className={inputCls} />
          <label className="flex items-center gap-2 text-[11px] text-slate-500">
            <input type="checkbox" checked={form.api_verify_tls} onChange={(e) => set('api_verify_tls', e.target.checked)} className="accent-cyan-500" />
            TLS sertifikasını doğrula (self-signed kurulumlarda kapatın)
          </label>
        </div>
      ) : form.snmp_version === 2 ? (
        <input type="password" value={form.community} onChange={(e) => set('community', e.target.value)} placeholder="community (kasada şifrelenir)" className={inputCls} />
      ) : (
        <div className="grid grid-cols-2 gap-2 md:grid-cols-5">
          <input value={form.v3_user} onChange={(e) => set('v3_user', e.target.value)} placeholder="v3 kullanıcı" className={inputCls} />
          <select value={form.v3_auth_proto} onChange={(e) => set('v3_auth_proto', e.target.value)} className={inputCls}>
            {['SHA', 'SHA256', 'SHA512', 'MD5'].map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
          <input type="password" value={form.v3_auth_pass} onChange={(e) => set('v3_auth_pass', e.target.value)} placeholder="auth pass" className={inputCls} />
          <select value={form.v3_priv_proto} onChange={(e) => set('v3_priv_proto', e.target.value)} className={inputCls}>
            {['AES', 'AES256', 'DES'].map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
          <input type="password" value={form.v3_priv_pass} onChange={(e) => set('v3_priv_pass', e.target.value)} placeholder="priv pass" className={inputCls} />
        </div>
      )}
      {form.vendor === 'snmp' && (
        <div className="flex items-center gap-2">
          <select value={form.snmp_version} onChange={(e) => set('snmp_version', +e.target.value)} className={inputCls + ' max-w-32'}>
            <option value={2}>SNMP v2c</option>
            <option value={3}>SNMP v3</option>
          </select>
        </div>
      )}
      <div className="flex items-center gap-2">
        <button type="submit" className="rounded-md bg-cyan-600 px-3.5 py-1.5 text-sm font-semibold text-white transition hover:bg-cyan-500">
          Kaydet
        </button>
        <span className="text-[11px] text-slate-600">poll aralığı: {form.poll_seconds} sn</span>
      </div>
    </form>
  )
}
