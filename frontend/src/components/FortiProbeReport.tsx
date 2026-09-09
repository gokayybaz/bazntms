// FortiProbeReport — "Bağlantıyı Sına" çıktısı (Faz 27).
//
// FortiOS monitor API'si şema vermez; bu rapor her uca atılan gerçek isteğin
// sonucudur: bağlanabilirlik, HTTP kodu, parser'ın anladığı kayıt sayısı ve
// yanıt zarfından çıkarılan FortiOS sürümü. Cihaz ekleme formu ve cihaz satırı
// ortak kullanır.

import { useState } from 'react'

export interface ProbeEndpoint {
  endpoint: string
  label: string
  http_status: number
  ok: boolean
  count: number
  shape: string
  note?: string
  raw_sample?: string
}

export interface ProbeReport {
  ok: boolean
  version: string
  build: number
  serial: string
  hostname: string
  vdom_mode: string
  vdoms: string[]
  profile_id: string
  error?: string
  endpoints: ProbeEndpoint[]
  caps: Record<string, string>
}

function statusMark(e: ProbeEndpoint): { icon: string; cls: string } {
  if (e.http_status === 401 || e.http_status === 403) return { icon: '✕', cls: 'text-rose-400' }
  if (e.http_status === 0 || e.http_status >= 400) return { icon: '✕', cls: 'text-rose-400' }
  if (e.ok) return { icon: '✓', cls: 'text-emerald-400' }
  return { icon: '~', cls: 'text-amber-400' }
}

export function FortiProbeReport({ report }: { report: ProbeReport }) {
  const [rawOpen, setRawOpen] = useState<string | null>(null)

  if (!report.ok) {
    return (
      <div className="mt-2 border border-rose-500/40 bg-rose-500/10 p-3 font-mono text-[11px] text-rose-300">
        <p className="font-bold uppercase tracking-wider">bağlantı başarısız</p>
        <p className="mt-1 break-words text-rose-200">{report.error || 'system/status ulaşılamadı'}</p>
        <p className="mt-1 text-tui-dim">api_url / port / token / trusthost kontrol edin.</p>
      </div>
    )
  }

  return (
    <div className="mt-2 border border-rule bg-panel p-3 font-mono text-[11px]">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="font-bold uppercase tracking-wider text-emerald-400">bağlandı</span>
        <span className="text-ink-hi">{report.hostname || '—'}</span>
        <span className="border border-orange-500/40 bg-orange-500/10 px-1.5 text-orange-300">
          FortiOS {report.version || '?'}
        </span>
        <span className="text-tui-dim">
          profil: <span className="text-ink">{report.profile_id}</span>
        </span>
        <span className="text-tui-dim">
          vdom: <span className="text-ink">{report.vdom_mode}</span>
          {report.vdoms.length > 0 && ` (${report.vdoms.join(', ')})`}
        </span>
        {report.serial && <span className="text-tui-dim">sn: {report.serial}</span>}
      </div>

      <table className="mt-2 w-full">
        <tbody>
          {report.endpoints.map((e) => {
            const m = statusMark(e)
            return (
              <tr key={e.endpoint} className="border-t border-rule/60 align-top">
                <td className={`py-1 pr-2 ${m.cls}`}>{m.icon}</td>
                <td className="py-1 pr-2 text-ink">{e.label}</td>
                <td className="py-1 pr-2 text-tui-dim">
                  {e.http_status === 0 ? 'ağ hatası' : `HTTP ${e.http_status}`}
                  {e.ok && ` · ${e.count} kayıt`}
                  {e.shape && ` · ${e.shape}`}
                </td>
                <td className="py-1 text-tui-dim">
                  {e.note}
                  {e.raw_sample && (
                    <button
                      type="button"
                      onClick={() => setRawOpen(rawOpen === e.endpoint ? null : e.endpoint)}
                      className="ml-2 text-cyan-400/80 hover:text-cyan-300"
                    >
                      {rawOpen === e.endpoint ? 'ham yanıtı gizle' : 'ham yanıt ▸'}
                    </button>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>

      {rawOpen && (
        <pre className="mt-2 max-h-48 overflow-auto border border-rule bg-ground p-2 text-[10px] leading-relaxed text-tui-dim">
          {report.endpoints.find((e) => e.endpoint === rawOpen)?.raw_sample}
        </pre>
      )}
    </div>
  )
}
