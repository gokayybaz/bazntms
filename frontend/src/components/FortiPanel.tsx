// FortiPanel — FortiGate cihaz detayı (Faz 8.6): kaynak kullanımı, VPN
// tünelleri, SD-WAN sağlık örnekleri ve politika hit trendleri.

import { useCallback, useEffect, useState } from 'react'
import { formatBits } from '../lib/format'

interface ResourceRow {
  ts: number
  cpu_pct: number
  mem_pct: number
  disk_pct: number
  sessions: number
}
interface VPNRow {
  vdom: string
  kind: string
  name: string
  peer: string
  status: string
  uptime: number
  rx_bytes: number
  tx_bytes: number
  ts: number
}
interface SDWANRow {
  ts: number
  vdom: string
  member: string
  health_check: string
  latency_ms: number
  jitter_ms: number
  packet_loss_pct: number
  state: string
}
interface PolicyRow {
  vdom: string
  policy_id: number
  name: string
  action: string
  hits: number
  bytes: number
}

function fmtUptime(s: number) {
  if (s <= 0) return '-'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return `${d}g ${h}sa`
  if (h > 0) return `${h}sa ${m}dk`
  return `${m}dk`
}

function statusColor(status: string) {
  if (status === 'up') return 'text-emerald-400'
  if (status === 'down') return 'text-rose-400'
  if (status === 'connecting') return 'text-amber-400'
  return 'text-dim-aa'
}

function fmtAgo(ts: number) {
  if (!ts) return '-'
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts)
  if (s < 60) return `${s} sn önce`
  if (s < 3600) return `${Math.floor(s / 60)} dk önce`
  if (s < 86400) return `${Math.floor(s / 3600)} sa önce`
  return `${Math.floor(s / 86400)} g önce`
}

// ResourceSparkline — CPU/bellek/disk zaman serisini (0-100%) tek küçük SVG'de
// üst üste çizer. minutes penceresinin tamamı gösterilir (FortiPanel yalnızca
// son noktayı Gauge'larda gösteriyordu; trend buradan okunur).
function ResourceSparkline({ rows }: { rows: ResourceRow[] }) {
  if (rows.length < 2) return null
  const W = 520
  const H = 44
  const t0 = rows[0].ts
  const t1 = rows[rows.length - 1].ts
  const span = Math.max(1, t1 - t0)
  const x = (ts: number) => ((ts - t0) / span) * W
  const y = (pct: number) => H - (Math.min(100, Math.max(0, pct)) / 100) * H
  const path = (pick: (r: ResourceRow) => number) =>
    rows.map((r, i) => `${i === 0 ? 'M' : 'L'}${x(r.ts).toFixed(1)},${y(pick(r)).toFixed(1)}`).join(' ')
  // seri renkleri: cpu rx-cyan'ı korur (DESIGN.md'de cyan'ın "birincil marka
  // vurgusu" olarak genel-amaçlı payı var); bellek/disk eskiden amber/violet
  // kullanıyordu — DESIGN.md'de bu ikisi sabit anlamlar taşıyor (amber=uyarı
  // eşiği, violet=her zaman cyan ile eşleşmesi gereken tx-trafik) ve burada
  // eşik/trafikle ilgisi olmayan çıplak kategori etiketi olarak kullanılıyordu
  // — nötr slate tonlarına taşındı (kullanıcı kararı)
  const series: [string, string, (r: ResourceRow) => number][] = [
    ['cpu', '#22d3ee', (r) => r.cpu_pct],
    ['bellek', '#94a3b8', (r) => r.mem_pct],
    ['disk', '#64748b', (r) => r.disk_pct],
  ]
  return (
    <div className="min-w-40 flex-1 rounded border border-slate-800 bg-slate-900/70 px-3 py-2">
      <div className="mb-1 flex items-center gap-2.5 text-[9px] uppercase tracking-wider text-dim-aa">
        <span>son {Math.round(span / 60)} dk trendi</span>
        {series.map(([label, color]) => (
          <span key={label} className="flex items-center gap-1">
            <span className="inline-block size-1.5 rounded-full" style={{ backgroundColor: color }} />
            {label}
          </span>
        ))}
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        preserveAspectRatio="none"
        style={{ height: H }}
        role="img"
        aria-label={`Son ${Math.round(span / 60)} dakika CPU/bellek/disk trendi`}
      >
        {series.map(([label, color, pick]) => (
          <path key={label} d={path(pick)} fill="none" stroke={color} strokeWidth={1.2} strokeOpacity={0.85} />
        ))}
      </svg>
    </div>
  )
}

function Gauge({ label, pct }: { label: string; pct: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  // normal durum eskiden cyan kullanıyordu (rx-trafik/marka rengi, "sağlıklı"
  // değil) — dosyanın kendi statusColor()'ıyla (up=emerald) hizalandı; rose/
  // amber eşikleri DESIGN.md'nin belgelediği -400 tonlarına taşındı
  const color = clamped > 85 ? 'bg-rose-400' : clamped > 60 ? 'bg-amber-400' : 'bg-emerald-400'
  return (
    <div className="min-w-40 flex-1 rounded border border-slate-800 bg-slate-900/70 px-3 py-2">
      <div className="flex items-baseline justify-between">
        <span className="text-[10px] uppercase tracking-wider text-dim-aa">{label}</span>
        <span className="font-mono text-sm text-slate-200">{pct.toFixed(0)}%</span>
      </div>
      <div className="mt-1.5 h-1.5 rounded bg-slate-800">
        <div className={`h-1.5 rounded ${color}`} style={{ width: `${clamped}%` }} />
      </div>
    </div>
  )
}

export function FortiPanel({ deviceId }: { deviceId: number }) {
  const [resources, setResources] = useState<ResourceRow[]>([])
  const [vpn, setVpn] = useState<VPNRow[]>([])
  const [sdwan, setSdwan] = useState<SDWANRow[]>([])
  const [policies, setPolicies] = useState<PolicyRow[]>([])
  // pollError: yalnızca EN SON pollun başarısız olduğunu gösterir, mevcut
  // veriyi ekrandan hiç kaldırmaz — eskiden tek bir hata paneli tüm
  // gauge/tablo/sparkline'ın yerini kalıcı olarak alıyordu (bir sonraki poll
  // başarılı olsa bile `error` hiç temizlenmiyordu). `loaded`, "hiç
  // yüklenmedi" ile "en az bir kez yüklendi" durumlarını ayırır.
  const [pollError, setPollError] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    try {
      const [res, vpn, sdwan, pol] = await Promise.all([
        fetch(`/api/v1/devices/${deviceId}/resources?minutes=180`),
        fetch(`/api/v1/devices/${deviceId}/vpn`),
        fetch(`/api/v1/devices/${deviceId}/sdwan?minutes=30`),
        fetch(`/api/v1/devices/${deviceId}/policies?minutes=180&limit=15`),
      ])
      if (!res.ok) throw new Error('veri alınamadı')
      setResources(await res.json())
      setVpn(await vpn.json())
      setSdwan(await sdwan.json())
      setPolicies(await pol.json())
      setPollError('')
    } catch (e) {
      setPollError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoaded(true)
    }
  }, [deviceId])

  useEffect(() => {
    load()
    const id = window.setInterval(load, 60_000)
    return () => window.clearInterval(id)
  }, [load])

  if (!loaded) return <p className="mt-2 text-xs text-dim-aa">yükleniyor…</p>

  const last = resources.length > 0 ? resources[resources.length - 1] : null
  const latestSdwan = new Map<string, SDWANRow>()
  for (const r of sdwan) {
    const key = `${r.health_check}|${r.member}`
    const cur = latestSdwan.get(key)
    if (!cur || r.ts > cur.ts) latestSdwan.set(key, r)
  }

  return (
    <div className="mt-2 space-y-3 rounded-md border border-orange-500/20 bg-slate-950/40 p-3">
      <p className="font-mono text-[10px] uppercase tracking-wider text-orange-300/80">
        fortigate rest api · canlı veri
      </p>
      {pollError && (
        <p className="text-xs text-rose-400">⚠ {pollError} — gösterilen veri son başarılı polldan, yeniden deneniyor</p>
      )}

      {/* kaynaklar */}
      <div className="flex flex-wrap gap-2">
        {last ? (
          <>
            <Gauge label="cpu" pct={last.cpu_pct} />
            <Gauge label="bellek" pct={last.mem_pct} />
            <Gauge label="disk" pct={last.disk_pct} />
            <div className="min-w-40 flex-1 rounded border border-slate-800 bg-slate-900/70 px-3 py-2">
              <span className="text-[10px] uppercase tracking-wider text-dim-aa">oturum</span>
              <div className="font-mono text-sm text-slate-200">{last.sessions.toLocaleString('tr-TR')}</div>
            </div>
          </>
        ) : (
          <p className="text-xs text-dim-aa">kaynak verisi bekleniyor (ilk poll sonrası görünür)</p>
        )}
      </div>

      {/* kaynak trendi (zaman serisi — Gauge'lar yalnızca son noktayı gösterir) */}
      {resources.length >= 2 && (
        <div className="flex">
          <ResourceSparkline rows={resources} />
        </div>
      )}

      {/* vpn */}
      {vpn.length > 0 && (
        <div>
          <p className="mb-1 text-[10px] uppercase tracking-wider text-dim-aa">vpn tünelleri / kullanıcıları</p>
          <div className="overflow-x-auto rounded border border-slate-800">
            <table className="w-full min-w-[720px] text-xs">
              <thead className="bg-slate-900/80">
                <tr className="text-left text-[10px] uppercase text-dim-aa">
                  <th className="px-2 py-1">VDOM</th><th className="px-2 py-1">Tür</th><th className="px-2 py-1">Ad</th>
                  <th className="px-2 py-1">Peer</th><th className="px-2 py-1">Durum</th>
                  <th className="px-2 py-1">Uptime</th>
                  <th className="px-2 py-1 text-right">↓</th><th className="px-2 py-1 text-right">↑</th>
                  <th className="px-2 py-1 text-right">Son Görülme</th>
                </tr>
              </thead>
              <tbody>
                {vpn.map((v, i) => (
                  <tr key={i} className="border-t border-slate-800/60">
                    <td className="px-2 py-1 font-mono text-dim-aa">{v.vdom || 'root'}</td>
                    <td className="px-2 py-1 font-mono text-slate-400">{v.kind}</td>
                    <td className="px-2 py-1 font-mono text-slate-300">{v.name}</td>
                    <td className="px-2 py-1 font-mono text-dim-aa">{v.peer || '-'}</td>
                    <td className={`px-2 py-1 font-mono text-[10px] ${statusColor(v.status)}`}>{v.status}</td>
                    <td className="px-2 py-1 font-mono text-dim-aa">{fmtUptime(v.uptime)}</td>
                    <td className="px-2 py-1 text-right font-mono text-cyan-300/90">{formatBits(v.rx_bytes * 8)}</td>
                    <td className="px-2 py-1 text-right font-mono text-violet-300/90">{formatBits(v.tx_bytes * 8)}</td>
                    <td className="px-2 py-1 text-right font-mono text-[10px] text-dim-aa">{fmtAgo(v.ts)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* sd-wan */}
      {latestSdwan.size > 0 && (
        <div>
          <p className="mb-1 text-[10px] uppercase tracking-wider text-dim-aa">sd-wan sağlık kontrolleri (son 30 dk)</p>
          <div className="overflow-x-auto rounded border border-slate-800">
            <table className="w-full min-w-[560px] text-xs">
              <thead className="bg-slate-900/80">
                <tr className="text-left text-[10px] uppercase text-dim-aa">
                  <th className="px-2 py-1">VDOM</th><th className="px-2 py-1">Health-Check</th><th className="px-2 py-1">Member</th>
                  <th className="px-2 py-1 text-right">Gecikme</th><th className="px-2 py-1 text-right">Jitter</th>
                  <th className="px-2 py-1 text-right">Kayıp</th><th className="px-2 py-1">Durum</th>
                </tr>
              </thead>
              <tbody>
                {[...latestSdwan.values()].map((w, i) => (
                  <tr key={i} className="border-t border-slate-800/60">
                    <td className="px-2 py-1 font-mono text-dim-aa">{w.vdom || 'root'}</td>
                    <td className="px-2 py-1 font-mono text-slate-400">{w.health_check}</td>
                    <td className="px-2 py-1 font-mono text-slate-300">{w.member}</td>
                    <td className="px-2 py-1 text-right font-mono text-slate-300">{w.latency_ms.toFixed(1)} ms</td>
                    <td className="px-2 py-1 text-right font-mono text-slate-400">{w.jitter_ms.toFixed(1)} ms</td>
                    <td className="px-2 py-1 text-right font-mono text-slate-400">{w.packet_loss_pct.toFixed(1)}%</td>
                    <td className={`px-2 py-1 font-mono text-[10px] ${statusColor(w.state)}`}>{w.state}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* politika hit'leri */}
      {policies.length > 0 && (
        <div>
          <p className="mb-1 text-[10px] uppercase tracking-wider text-dim-aa">en aktif politika hit'leri (son 3 sa)</p>
          <div className="overflow-x-auto rounded border border-slate-800">
            <table className="w-full min-w-[520px] text-xs">
              <thead className="bg-slate-900/80">
                <tr className="text-left text-[10px] uppercase text-dim-aa">
                  <th className="px-2 py-1">VDOM</th><th className="px-2 py-1">ID</th><th className="px-2 py-1">Politika</th>
                  <th className="px-2 py-1">Aksiyon</th>
                  <th className="px-2 py-1 text-right">Hit Δ</th><th className="px-2 py-1 text-right">Bayt Δ</th>
                </tr>
              </thead>
              <tbody>
                {policies.map((p) => (
                  <tr key={`${p.vdom}-${p.policy_id}`} className="border-t border-slate-800/60">
                    <td className="px-2 py-1 font-mono text-dim-aa">{p.vdom || 'root'}</td>
                    <td className="px-2 py-1 font-mono text-dim-aa">{p.policy_id}</td>
                    <td className="px-2 py-1 font-mono text-slate-300">{p.name}</td>
                    <td className="px-2 py-1">
                      <span className={`font-mono text-[10px] ${p.action === 'accept' ? 'text-emerald-400' : 'text-rose-400/80'}`}>{p.action}</span>
                    </td>
                    <td className="px-2 py-1 text-right font-mono text-cyan-300/90">{p.hits.toLocaleString('tr-TR')}</td>
                    <td className="px-2 py-1 text-right font-mono text-slate-400">{formatBits(p.bytes * 8)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {vpn.length === 0 && latestSdwan.size === 0 && policies.length === 0 && !last && (
        <p className="text-xs text-dim-aa">
          FortiGate verisi henüz yok — poll tamamlandığında bu panel dolacak (arayüzler sekmesi SNMP ile aynı uçtan izlenir).
        </p>
      )}
    </div>
  )
}
