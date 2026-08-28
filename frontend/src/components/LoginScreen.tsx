import { useState } from 'react'

export function LoginScreen({ onSuccess }: { onSuccess: () => void }) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password) return
    setBusy(true)
    setError('')
    try {
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) throw new Error(data.error ?? 'giriş başarısız')
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid min-h-screen place-items-center px-4">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-lg border border-slate-800 bg-slate-900/70 p-7 shadow-2xl shadow-black/40 backdrop-blur"
      >
        <div className="mb-6 flex items-center gap-3">
          <div className="grid size-11 place-items-center rounded-md bg-slate-900 border border-cyan-500/50">
            <svg viewBox="0 0 24 24" className="size-6 text-white" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="4" y="10" width="16" height="10" rx="2" />
              <path d="M8 10V7a4 4 0 0 1 8 0v3" strokeLinecap="round" />
            </svg>
          </div>
          <div>
            <h1 className="font-mono text-lg font-bold leading-tight text-white">bazNTMS</h1>
            <p className="text-xs text-slate-500">devam etmek için şifrenizi girin</p>
          </div>
        </div>

        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Şifre"
          autoFocus
          className="w-full rounded-lg border border-slate-700/80 bg-slate-950 px-3.5 py-2.5 text-sm text-slate-200 outline-none placeholder:text-slate-600 focus:border-cyan-500/60"
        />
        {error && (
          <p className="mt-2 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-400 ring-1 ring-rose-500/30">{error}</p>
        )}
        <button
          type="submit"
          disabled={busy || !password}
          className="mt-4 w-full rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold text-white transition enabled:hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {busy ? 'Giriş yapılıyor…' : 'Giriş Yap'}
        </button>
        <p className="mt-4 text-center text-[11px] text-slate-600">
          Oturumlar 7 gün geçerlidir · sunucu yeniden başlarsa tekrar giriş gerekir
        </p>
      </form>
    </div>
  )
}
