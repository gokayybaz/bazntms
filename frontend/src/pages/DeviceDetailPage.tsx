import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { formatBits, formatBytes, formatNum } from '../lib/format'
import { SEV_NAMES, SEV_STYLES } from '../lib/syslogSeverity'
import { Panel } from '../components/Panel'
import { FortiPanel } from '../components/FortiPanel'
import { StatusPill, type StatusTone } from '../components/StatusPill'

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
  // Faz 23-C
  rx_util_pct: number // <0 → hesaplanamadı
  tx_util_pct: number
  class: string
  speed_bps: number
  speed_source: string
}

// UtilCell — arayüz kullanım yüzdesi + eşik renkli mini çubuk (htop dili).
// pct < 0 → "-" (güvenilir hız yok). Eşikler alert IfaceConfig varsayılanıyla
// hizalı: <70 emerald, <90 amber, >=90 rose.
function UtilCell({ pct }: { pct: number }) {
  if (pct == null || pct < 0) return <span className="font-mono text-xs text-tui-dim">—</span>
  const tone = pct < 70 ? 'bg-emerald-500' : pct < 90 ? 'bg-amber-500' : 'bg-rose-500'
  const txt = pct < 70 ? 'text-emerald-400' : pct < 90 ? 'text-amber-400' : 'text-rose-400'
  return (
    <span className="flex items-center justify-end gap-1.5">
      <span className="h-1 w-10 bg-panel-2">
        <span className={`block h-full ${tone}`} style={{ width: `${Math.min(100, Math.max(2, pct))}%` }} />
      </span>
      <span className={`w-9 text-right font-mono text-xs ${txt}`}>{pct >= 1 ? `%${Math.round(pct)}` : '%<1'}</span>
    </span>
  )
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
  // 3 fetch effect'i de 401'i sessizce yutuyordu — sayfa donmuş bir oturumla
  // sonsuza kadar son bilinen "sağlıklı" durumu göstermeye devam ediyordu.
  // Artık en az bir istek 401 dönerse görünür bir banner tetikleniyor.
  const [sessionExpired, setSessionExpired] = useState(false)

  useEffect(() => {
    if (!id) return
    let stop = false
    const load = async () => {
      try {
        const res = await fetch('/api/v1/devices')
        if (res.status === 401) {
          if (!stop) setSessionExpired(true)
          return
        }
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
        if (res.status === 401) {
          if (!stop) setSessionExpired(true)
          return
        }
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
        if (fr.status === 401 || sr.status === 401) {
          if (!stop) setSessionExpired(true)
          return
        }
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

  // 401, sayfa hiç "loaded" durumuna geçemeden (ilk fetch turu) da
  // gelebilir — bu yüzden bildirim, aşağıdaki üç dönüş yolunun (notFound /
  // yükleniyor / ana içerik) hepsinde ayrı ayrı, en üstte gösteriliyor
  const sessionBanner = sessionExpired && (
    <p className="border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-400">
      ⚠ Oturum sona ermiş olabilir — veriler güncellenmiyor. Sayfayı yenileyip tekrar giriş yapın.
    </p>
  )

  if (notFound) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        {sessionBanner}
        <p className="py-16 text-center text-sm text-tui-dim">
          Cihaz bulunamadı. <Link to="/cihazlar" className="text-rx hover:underline">Cihaz listesine dön</Link>
        </p>
      </div>
    )
  }

  if (!loaded || !device) {
    return (
      <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        {sessionBanner}
        <p className="py-16 text-center text-sm text-tui-dim">Yükleniyor…</p>
      </div>
    )
  }

  // eskiden last_poll===0 (hiç poll edilmemiş, yeni eklenmiş) "sorunlu" ile
  // aynı gri rozete düşüyordu — üç ayrı durum artık ayrıştırılıyor: gerçekten
  // sorunlu (kapalı veya son hata var) / ilk poll'u bekliyor / sağlıklı
  const healthStatus: 'healthy' | 'pending' | 'problem' =
    !device.enabled || device.last_error ? 'problem' : device.last_poll === 0 ? 'pending' : 'healthy'
  const HEALTH_META: Record<typeof healthStatus, { tone: StatusTone; label: string }> = {
    healthy: { tone: 'emerald', label: 'sağlıklı' },
    pending: { tone: 'amber', label: 'ilk poll bekleniyor' },
    problem: { tone: 'rose', label: 'sorunlu' },
  }

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <Link to="/cihazlar" className="text-xs text-tui-dim hover:text-rx">
          ← Cihazlar
        </Link>
      </div>

      {sessionBanner}

      {/* başlık */}
      <div className="flex flex-wrap items-center gap-3">
        <StatusPill tone={HEALTH_META[healthStatus].tone} label={HEALTH_META[healthStatus].label} />
        <h1 className="text-[13px] font-bold uppercase tracking-[0.04em] text-ink-hi">{device.name}</h1>
        <span className="bg-panel-2 px-2 py-0.5 font-mono text-xs uppercase text-tui-dim">{device.kind}</span>
        {device.vendor === 'fortigate' ? (
          <span className="border border-orange-500/40 bg-orange-500/10 px-2 py-0.5 font-mono text-xs uppercase text-orange-300">
            fortigate rest api
          </span>
        ) : (
          <span className="border border-rule-hi px-2 py-0.5 font-mono text-xs uppercase text-tui-dim">
            snmp v{device.snmp_version === 3 ? '3' : '2c'}
          </span>
        )}
      </div>

      {device.last_error && (
        <p className="border border-rose-500/30 bg-rose-500/10 px-3 py-2 font-mono text-xs text-rose-400">
          ⚠ {device.last_error}
        </p>
      )}

      {/* özet şeridi */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div className="border border-rule bg-panel p-3.5">
          <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">Host</p>
          <p className="mt-1.5 truncate font-mono text-sm text-ink-hi">{device.host}</p>
        </div>
        <div className="border border-rule bg-panel p-3.5">
          <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">Sistem Adı</p>
          <p className="mt-1.5 truncate font-mono text-sm text-ink-hi" title={device.sys_descr}>
            {device.sys_name || '—'}
          </p>
        </div>
        <div className="border border-rule bg-panel p-3.5">
          <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">Poll Aralığı</p>
          <p className="mt-1.5 font-mono text-sm text-ink-hi">{device.poll_seconds} sn</p>
        </div>
        <div className="border border-rule bg-panel p-3.5">
          <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">Son Poll</p>
          <p className="mt-1.5 font-mono text-sm text-ink-hi">{relTime(device.last_poll)}</p>
        </div>
        <div className="border border-rule bg-panel p-3.5">
          <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">Eklenme</p>
          <p className="mt-1.5 font-mono text-sm text-ink-hi">{relDate(device.added_at)}</p>
        </div>
        {device.vendor === 'fortigate' && (
          <>
            <div className="border border-rule bg-panel p-3.5">
              <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">API URL</p>
              <p className="mt-1.5 truncate font-mono text-sm text-ink-hi" title={device.api_url}>{device.api_url || '—'}</p>
            </div>
            <div className="border border-rule bg-panel p-3.5">
              <p className="text-[10px] font-medium uppercase tracking-[0.04em] text-tui-dim">VDOM · TLS</p>
              <p className="mt-1.5 font-mono text-sm text-ink-hi">
                {device.vdom || 'root'} · {device.api_verify_tls ? 'doğrulanıyor' : 'atlanıyor'}
              </p>
            </div>
          </>
        )}
      </div>

      {/* fortigate derin panel */}
      {device.vendor === 'fortigate' && (
        <Panel title="FortiGate Detayı" right={<span className="text-xs text-tui-dim">REST API · canlı</span>}>
          <FortiPanel deviceId={device.id} />
        </Panel>
      )}

      {/* arayüzler */}
      <Panel title="Arayüzler" right={<span className="text-xs text-tui-dim">{ifaces.length} arayüz</span>}>
        {ifaces.length === 0 ? (
          <p className="py-6 text-center text-sm text-tui-dim">Henüz arayüz verisi yok.</p>
        ) : (
          <div className="overflow-x-auto border border-rule">
            <table className="w-full text-sm">
              <thead className="sticky top-0">
                <tr className="bg-rx text-left text-[10px] uppercase tracking-[0.04em] text-ground">
                  <th scope="col" className="px-3 py-2 font-medium">Arayüz</th>
                  <th scope="col" className="px-3 py-2 font-medium">Tür</th>
                  <th scope="col" className="px-3 py-2 font-medium">Durum</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium" title="Etkin hız — ifHighSpeed varsa o, yoksa ifSpeed; güvenilir değilse '-'">Hız</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">↓</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">↑</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium" title="Kullanım ↓ — yalnız güvenilir hız + oper=up iken">Util ↓</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium" title="Kullanım ↑">Util ↑</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Toplam (↓/↑)</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">Hata (in/out)</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium" title="ifInDiscards / ifOutDiscards — kuyruk taşması, QoS drop (hatadan farklı)">
                    Atılan (in/out)
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-rule">
                {ifaces.map((i) => (
                  <tr key={i.if_index} className="hover:bg-panel-2/40">
                    <td className="px-3 py-1.5">
                      <span className="font-mono text-ink">{i.name || `if${i.if_index}`}</span>
                      {i.alias && <span className="ml-2 text-[11px] text-tui-dim">{i.alias}</span>}
                    </td>
                    <td className="px-3 py-1.5 font-mono text-[10px] text-tui-dim">{i.class && i.class !== 'unknown' ? i.class : '—'}</td>
                    <td className="px-3 py-1.5">
                      <span className={`font-mono text-[10px] ${i.oper_status === 1 ? 'text-emerald-400' : 'text-tui-dim'}`}>
                        {i.oper_status === 1 ? 'up' : 'down'}
                      </span>
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-tui-dim" title={i.speed_source}>
                      {i.speed_bps > 0 ? formatBits(i.speed_bps) : '—'}
                    </td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-rx">{formatBits(i.rx_bps)}</td>
                    <td className="px-3 py-1.5 text-right font-mono text-xs text-tx">{formatBits(i.tx_bps)}</td>
                    <td className="px-3 py-1.5 text-right"><UtilCell pct={i.rx_util_pct} /></td>
                    <td className="px-3 py-1.5 text-right"><UtilCell pct={i.tx_util_pct} /></td>
                    <td className="px-3 py-1.5 text-right font-mono text-[11px] text-tui-dim">
                      {formatBytes(i.rx_bytes)}/{formatBytes(i.tx_bytes)}
                    </td>
                    <td className={`px-3 py-1.5 text-right font-mono text-xs ${i.in_errors + i.out_errors > 0 ? 'text-amber-400' : 'text-tui-dim'}`}>
                      {i.in_errors}/{i.out_errors}
                    </td>
                    <td className={`px-3 py-1.5 text-right font-mono text-xs ${i.in_discards + i.out_discards > 0 ? 'text-amber-400' : 'text-tui-dim'}`}>
                      {i.in_discards}/{i.out_discards}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {/* netflow */}
      <Panel title="NetFlow v5 Akışları" right={<span className="text-xs text-tui-dim">son 15 dk · bu cihaz · {formatNum(deviceFlows.length)}</span>}>
        {deviceFlows.length === 0 ? (
          <p className="py-6 text-center text-sm text-tui-dim">Bu cihazdan akış yok.</p>
        ) : (
          <>
            {deviceFlows.length > 100 && (
              <p className="mb-1.5 text-[10px] text-amber-400">
                ilk 100 / {formatNum(deviceFlows.length)} akış gösteriliyor
              </p>
            )}
            <div className="max-h-72 overflow-y-auto border border-rule">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-panel">
                  <tr className="bg-rx text-left text-[10px] uppercase tracking-[0.04em] text-ground">
                    <th scope="col" className="px-3 py-1.5 font-medium">Saat</th>
                    <th scope="col" className="px-3 py-1.5 font-medium">Akış</th>
                    <th scope="col" className="px-3 py-1.5 font-medium">Protokol</th>
                    <th scope="col" className="px-3 py-1.5 text-right font-medium">Paket</th>
                    <th scope="col" className="px-3 py-1.5 text-right font-medium">Octet</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-rule">
                  {deviceFlows.slice(0, 100).map((f, i) => (
                    <tr key={i} className="hover:bg-panel-2/40">
                      <td className="px-3 py-1.5 font-mono text-[11px] text-tui-dim">
                        {f.ts ? new Date(f.ts * 1000).toLocaleTimeString('tr-TR') : '—'}
                      </td>
                      <td className="px-3 py-1.5 font-mono text-xs text-ink">
                        {f.src}:{f.src_port} → {f.dst}:{f.dst_port}
                      </td>
                      <td className="px-3 py-1.5">
                        <span className="bg-panel-2 px-1.5 py-0.5 font-mono text-[10px] uppercase text-tui-dim">{f.proto}</span>
                      </td>
                      <td className="px-3 py-1.5 text-right font-mono text-xs text-tui-dim">{f.packets}</td>
                      <td className="px-3 py-1.5 text-right font-mono text-xs text-emerald-400">{formatBytes(f.octets)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Panel>

      {/* syslog */}
      <Panel title="Syslog Olayları" right={<span className="text-xs text-tui-dim">bu cihaz · {formatNum(deviceSyslog.length)}</span>}>
        {deviceSyslog.length === 0 ? (
          <p className="py-6 text-center text-sm text-tui-dim">Bu cihazdan syslog olayı yok.</p>
        ) : (
          <ul className="max-h-72 space-y-1 overflow-y-auto pr-1">
            {deviceSyslog.map((e) => (
              <li key={e.id} className="flex items-baseline gap-2 px-2 py-1 font-mono text-[11px] hover:bg-panel-2/40">
                <span className="text-tui-dim">{new Date(e.ts * 1000).toLocaleTimeString('tr-TR')}</span>
                <span className={`px-1 text-[10px] ${SEV_STYLES[e.severity]}`}>{SEV_NAMES[e.severity]}</span>
                <span className="truncate text-ink">
                  {e.tag && <span className="text-tui-dim">{e.tag}: </span>}
                  {e.message}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Panel>
    </div>
  )
}
