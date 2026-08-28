import { useCallback, useEffect, useState } from 'react'
import type { RecordFile, RecordInfo } from '../types'
import { formatBytes, formatNum } from '../lib/format'

export function PcapCard({ record, canRecord }: { record: RecordInfo; canRecord: boolean }) {
  const [files, setFiles] = useState<RecordFile[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const loadFiles = useCallback(async () => {
    try {
      setFiles(await fetch('/api/record/files').then((r) => r.json()))
    } catch {
      /* yoksay */
    }
  }, [])

  useEffect(() => {
    loadFiles()
  }, [loadFiles])

  const start = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      const res = await fetch('/api/record/start', { method: 'POST' })
      const data = await res.json()
      if (!data.ok) throw new Error(data.error)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }, [])

  const stop = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      await fetch('/api/record/stop', { method: 'POST' })
      loadFiles()
    } catch {
      /* yoksay */
    } finally {
      setBusy(false)
    }
  }, [loadFiles])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        {record.recording ? (
          <>
            <span className="inline-flex items-center gap-1.5 rounded-full bg-rose-500/10 px-2.5 py-1 text-xs font-medium text-rose-400 ring-1 ring-rose-500/30">
              <span className="size-1.5 animate-pulse rounded-full bg-rose-500" />
              KAYIT
            </span>
            <span className="font-mono text-xs text-slate-400">{record.file}</span>
            <span className="font-mono text-xs text-slate-600">
              {formatNum(record.packets)} paket · {formatBytes(record.bytes)}
            </span>
          </>
        ) : (
          <span className="text-xs text-slate-500">
            {canRecord ? 'Kayıt kapalı — yakalama açıkken paketleri .pcap dosyasına yazabilirsiniz.' : 'Kayıt için önce yakalamayı başlatın.'}
          </span>
        )}
        <div className="ml-auto flex gap-2">
          {record.recording ? (
            <button
              onClick={stop}
              disabled={busy}
              className="rounded-lg bg-rose-500/90 px-3.5 py-1.5 text-sm font-semibold text-white transition hover:bg-rose-500 disabled:opacity-40"
            >
              Kaydı Durdur
            </button>
          ) : (
            <button
              onClick={start}
              disabled={busy || !canRecord}
              className="rounded-lg bg-rose-600 px-3.5 py-1.5 text-sm font-semibold text-white transition enabled:hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Kaydı Başlat
            </button>
          )}
        </div>
      </div>

      {error && (
        <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-400 ring-1 ring-rose-500/30">{error}</p>
      )}

      <div>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Kayıtlar</h3>
        {files.length === 0 ? (
          <p className="py-4 text-center text-sm text-slate-600">Henüz .pcap dosyası yok.</p>
        ) : (
          <ul className="max-h-56 space-y-1.5 overflow-y-auto pr-1">
            {files.map((f) => (
              <li key={f.name} className="flex items-center gap-3 rounded-lg border border-slate-800 px-3 py-2 text-sm">
                <span className="min-w-0 flex-1 truncate font-mono text-xs text-slate-300">{f.name}</span>
                <span className="font-mono text-xs text-slate-500">{formatBytes(f.bytes)}</span>
                <a
                  href={`/api/record/download?file=${encodeURIComponent(f.name)}`}
                  className="rounded-md bg-cyan-500/10 px-2 py-1 text-xs font-medium text-cyan-400 ring-1 ring-cyan-500/30 transition hover:bg-cyan-500/20"
                >
                  İndir
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
