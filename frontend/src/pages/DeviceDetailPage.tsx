import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { Card } from '../components/Card'
import { FortiPanel } from '../components/FortiPanel'

interface Device {
  id: number
  name: string
  host: string
  kind: string
  vendor: string
  snmp_version: number
  poll_seconds: number
  enabled: boolean
  sys_name: string
  sys_descr: string
  api_url: string
  api_verify_tls: boolean
  vdom: string
  added_at: number
  last_poll: number
  last_error: string
}

function relDate(unix: number): string {
  if (!unix) return '—'
  return new Date(unix * 1000).toLocaleDateString('tr-TR')
}

interface IfaceRate {
  if_index: number
  name: string
  alias: string
  speed: number
  oper_status: number
  rx_bps: number
  tx_bps: number
  rx_bytes: number
  tx_bytes: number
  in_errors: number
  out_errors: number
  in_discards: number
  out_discards: number
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

interface SyslogEvent {
  id: number
  ts: number
  host: string
  source_ip: string
  severity: number
  tag: string
  message: string
}

const SEV_NAMES = ['emergency', 'alert', 'critical', 'error', 'warning', 'notice', 'info', 'debug']

function relTime(unix: number): string {
  if (!unix) return 'hiç poll edilmedi'
  const secs = Math.max(0, Math.floor(Date.now() / 1000) - unix)
  if (secs < 60) return `${secs} sn önce`
  const m = Math.floor(secs / 60)
  if (m < 60) return `${m} dk önce`
  return `${Math.floor(m / 60)} sa önce`
}

export function DeviceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [device, setDevice] = useState<Device | null>(null)
  const [ifaces, setIfaces] = useState<IfaceRate[]>([])
  const [flows, setFlows] = useState<FlowRow[]>([])
  const [syslog, setSyslog] = useState<SyslogEvent[]>([])
  const [loaded, setLoaded] = useState(false)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/devices')
        if (res.status === 401) return
        const list: Device[] = await res.json()
        const found = list.find((d) => String(d.id) === id)
        if (!stop) {
          if (!found) setNotFound(true)
          else setDevice(found)
          setLoaded(true)
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const t = window.setInterval(load, 8_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id])

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/devices/${id}/interfaces`)
        if (res.status === 401) return
        if (!stop) setIfaces(await res.json())
      } catch {
        /* yoksay */
      }
    }
    load()
    const t = window.setInterval(load, 10_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [id])

  // NetFlow/syslog cihaz-id ile iliskili degil; kaynak IP / host ile eslesir,
  // burada istemci tarafinda cihazin host'una gore filtrelenir (bkz. deviceFlows/deviceSyslog)
  useEffect(() => {
    let stop = false
    const load = async () => {
      try {
        const [fr, sr] = await Promise.all([
          fetch('/api/v1/flows?minutes=15&limit=200'),
          fetch('/api/v1/syslog?limit=200'),
        ])
        if (fr.status === 401 || sr.status === 401) return
        if (!stop) {
          setFlows(await fr.json())
          setSyslog(await sr.json())
        }
      } catch {
        /* yoksay */
      }
    }
    load()
    const t = window.setInterval(load, 10_000)
    return () => {
      stop = true
      window.clearInterval(t)
    }
  }, [])

  // f.device = NetFlow exporter'ın kaynak IP'si. Bir NAT/proxy arkasındaki hub'da
  // (ör. Docker Desktop) exporter IP'si yeniden yazılır; o yüzden cihazın host'u
  // akışın src/dst'sinde geçiyorsa da bu cihaza ait say.
  const deviceFlows = useMemo(
    () =>
      device
        ? flows.filter((f) => f.device === device.host || f.src === device.host || f.dst === device.host)
        : [],
    [flows, device],
  )
  // syslog kaydı hostname (cihazın verdiği ad) ile gelir; kaynak IP ya da SNMP
  // sys_name cihazın host'una eşleşiyorsa da bu cihaza ait say
  const deviceSyslog = useMemo(
    () =>
      device
        ? syslog.filter(
            (e) =>
              e.source_ip === device.host ||
              e.host === device.host ||
              (!!device.sys_name && e.host === device.sys_name),
          )
        : [],
    [syslog, device],
  )

  if (notFound) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <p className="py-16 text-center text-sm text-slate-600">
          Cihaz bulunamadı. <Link to="/cihazlar" className="text-cyan-400 hover:underline">Cihaz listesine dön</Link>
        </p>
      </div>
    )
  }

  if (!loaded || !device) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <p className="py-16 text-center text-sm text-slate-600">Yükleniyor…</p>
      </div>
    )
  }

  const healthy = device.enabled && device.last_poll > 0 && !device.last_error

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <Link to="/cihazlar" className="text-xs text-slate-500 hover:text-cyan-400">
          ← Cihazlar
        </Link>
      </div>

      {/* başlık */}
      <div className="flex flex-wrap items-center gap-3">
        <span
          className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
            healthy
              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
              : 'border-slate-600 bg-slate-800 text-slate-500'
          }`}
        >
          <span className={`size-1.5 rounded-full ${healthy ? 'bg-emerald-400' : 'bg-slate-500'}`} />
          {healthy ? 'sağlıklı' : 'sorunlu'}
        </span>
        <h1 className="font-mono text-xl font-bold text-slate-100">{device.name}</h1>
        <span className="rounded bg-slate-800 px-2 py-0.5 font-mono text-xs uppercase text-slate-400">{device.kind}</span>
        {device.vendor === 'fortigate' ? (
          <span className="rounded border border-orange-500/40 bg-orange-500/10 px-2 py-0.5 font-mono text-xs uppercase text-orange-300">
            fortigate rest api
          </span>
        ) : (
          <span className="rounded border border-slate-700 px-2 py-0.5 font-mono text-xs uppercase text-slate-500">
            snmp v{device.snmp_version === 3 ? '3' : '2c'}
          </span>
        )}
      </div>

      {device.last_error && (
        <p className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 font-mono text-xs text-rose-400">
          ⚠ {device.last_error}
        </p>
      )}

      {/* özet şeridi */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Host</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200">{device.host}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Sistem Adı</p>
          <p className="mt-1.5 truncate font-mono text-sm text-slate-200" title={device.sys_descr}>
            {device.sys_name || '—'}
          </p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Poll Aralığı</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{device.poll_seconds} sn</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Son Poll</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{relTime(device.last_poll)}</p>
        </div>
        <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">Eklenme</p>
          <p className="mt-1.5 font-mono text-sm text-slate-200">{relDate(device.added_at)}</p>
        </div>
        {device.vendor === 'fortigate' && (
          <>
            <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
              <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">API URL</p>
              <p className="mt-1.5 truncate font-mono text-sm text-slate-200" title={device.api_url}>{device.api_url || '—'}</p>
            </div>
            <div className="rounded-md border border-slate-800 bg-slate-900/70 p-3.5">
              <p className="text-[10px] font-semibold uppercase tracking-widest text-slate-400">VDOM · TLS</p>
              <p className="mt-1.5 font-mono text-sm text-slate-200">
                {device.vdom || 'root'} · {device.api_verify_tls ? 'doğrulanıyor' : 'atlanıyor'}
              </p>
            </div>
          </>
        )}
      </div>

      {/* fortigate derin panel */}
      {device.vendor === 'fortigate' && (
        <Card title="FortiGate Detayı" right={<span className="text-xs text-slate-500">REST API · canlı</span>}>
          <FortiPanel deviceId={device.id} />
        </Card>
      )}

      {/* arayüzler */}
      <Card title="Arayüzler" right={<span className="text-xs text-slate-500">{ifaces.length} arayüz</span>}>
        {ifaces.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Henüz arayüz verisi yok.</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="bg-slate-900/95">
                <tr className="text-left text-[11px] uppercase tracking-wider text-slate-500">
                  <th className="px-3 py-2 font-medium">Arayüz</th>
                  <th className="px-3 py-2 font-medium">Durum</th>
                  <th className="px-3 py-2 text-right font-medium">Hız</th>
                  <th className="px-3 py-2 text-right font-medium">↓</th>
                  <th className="px-3 py-2 text-right font-medium">↑</th>
                  <th className="px-3 py-2 text-right font-medium">Toplam (↓/↑)</th>
                  <th className="px-3 py-2 text-right font-medium">Hata (in/out)</th>
                  <th className="px-3 py-2 text-right font-medium" title="ifInDiscards / ifOutDiscards — kuyruk taşması, QoS drop (hatadan farklı)">
                    Atılan (in/out)
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {ifaces.map((i) => (
                  <tr key={i.if_index} className="hover:bg-slate-800/30">
                    <td className="px-3 py-1.5">
                      <span className="font-mono text-slate-300">{i.name || `if${i.if_index}`}</span>
                      {i.alias && <span className="ml-2 text-[11px] text-slate-600">{i.alias}</span>}
                    </td>
                    <td className="px-3 py-1.5">
                      <span className={`font-mono text-[10px] ${i.oper_status === 1 ? 'text-emerald-400' : 'text-slate-500'}`}>
                        {i.oper_status === 1 ? 'up' : 'down'}
                      </span>
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-slate-500">
                      {i.speed > 0 ? formatBits(i.speed) : '—'}
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-cyan-300/90">{formatBits(i.rx_bps)}</td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-violet-300/90">{formatBits(i.tx_bps)}</td>
                    <td className="px-3 py-1.5 text-right font-mono text-[11px] text-slate-500">
                      {formatBytes(i.rx_bytes)}/{formatBytes(i.tx_bytes)}
                    </td>
                    <td className={`px-3 py-1.5 text-right font-mono text-xs ${i.in_errors + i.out_errors > 0 ? 'text-amber-400/90' : 'text-slate-500'}`}>
                      {i.in_errors}/{i.out_errors}
                    </td>
                    <td className={`px-3 py-1.5 text-right font-mono text-xs ${i.in_discards + i.out_discards > 0 ? 'text-amber-400/90' : 'text-slate-500'}`}>
                      {i.in_discards}/{i.out_discards}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* netflow */}
      <Card title="NetFlow v5 Akışları" right={<span className="text-xs text-slate-500">son 15 dk · bu cihaz · {formatNum(deviceFlows.length)}</span>}>
        {deviceFlows.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Bu cihazdan akış yok.</p>
        ) : (
          <div className="max-h-72 overflow-y-auto rounded-lg border border-slate-800/60">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-slate-900/95">
                <tr className="text-left text-[10px] uppercase tracking-wider text-slate-500">
                  <th className="px-3 py-1.5 font-medium">Saat</th>
                  <th className="px-3 py-1.5 font-medium">Akış</th>
                  <th className="px-3 py-1.5 font-medium">Protokol</th>
                  <th className="px-3 py-1.5 text-right font-medium">Paket</th>
                  <th className="px-3 py-1.5 text-right font-medium">Octet</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {deviceFlows.slice(0, 100).map((f, i) => (
                  <tr key={i} className="hover:bg-slate-800/30">
                    <td className="px-3 py-1.5 font-mono text-[11px] text-slate-500">
                      {f.ts ? new Date(f.ts * 1000).toLocaleTimeString('tr-TR') : '—'}
                    </td>
                    <td className="px-3 py-1.5 font-mono text-xs text-slate-300">
                      {f.src}:{f.src_port} → {f.dst}:{f.dst_port}
                    </td>
                    <td className="px-3 py-1.5">
                      <span className="rounded bg-slate-800 px-1.5 py-0.5 font-mono text-[10px] uppercase text-slate-400">{f.proto}</span>
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-slate-400">{f.packets}</td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-emerald-300/90">{formatBytes(f.octets)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* syslog */}
      <Card title="Syslog Olayları" right={<span className="text-xs text-slate-500">bu cihaz · {formatNum(deviceSyslog.length)}</span>}>
        {deviceSyslog.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-600">Bu cihazdan syslog olayı yok.</p>
        ) : (
          <ul className="max-h-72 space-y-1 overflow-y-auto pr-1">
            {deviceSyslog.map((e) => (
              <li key={e.id} className="flex items-baseline gap-2 rounded px-2 py-1 font-mono text-[11px] hover:bg-slate-800/30">
                <span className="text-slate-600">{new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}</span>
                <span className="rounded px-1 text-[10px] text-slate-400 ring-1 ring-slate-700">{SEV_NAMES[e.severity]}</span>
                <span className="truncate text-slate-300">
                  {e.tag && <span className="text-slate-500">{e.tag}: </span>}
                  {e.message}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}
