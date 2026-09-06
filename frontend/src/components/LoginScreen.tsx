import { useState } from 'react'

export function LoginScreen({
  onSuccess,
}: {
  onSuccess: (identity: { username: string; role: string; site?: string }) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!password) return
    setBusy(true)
    setError('')
    try {
      // username bos → legacy tek-sifre (admin); dolu → kullanici girisi (RBAC)
      const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username || undefined, password }),
      })
      const data = await res.json()
      if (!res.ok || !data.ok) throw new Error(data.error ?? 'giriş başarısız')
      onSuccess({ username: data.username ?? 'admin', role: data.role ?? 'admin', site: data.site ?? '' })
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="min-h-screen bg-ground p-6 font-mono text-[13px] text-ink">
      <pre className="text-[11px] leading-tight text-rx">{`
 _                 _   _ _____ __  __ ___
| |__   __ _ ____ | \\ | |_   _|  \\/  / __|
| '_ \\ / _\` |_  / |  \\| | | | | |\\/| \\__ \\
|_.__/ \\__,_/___| |_|\\__| |_| |_|  |_|___/`}</pre>
      <p className="mt-2 text-[11px] text-tui-dim">bazNTMS — Network Traffic Monitoring System · tty1</p>

      <form onSubmit={submit} className="mt-6 max-w-md space-y-2">
        <label className="flex items-center gap-2">
          <span className="text-tui-dim">bazntms login:</span>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="(boş = admin)"
            autoComplete="username"
            className="min-w-0 flex-1 border-b border-rule-hi bg-transparent px-1 py-0.5 text-ink-hi outline-none placeholder:text-tui-dim focus:border-rx"
          />
        </label>
        <label className="flex items-center gap-2">
          <span className="text-tui-dim">Password:</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
            className="min-w-0 flex-1 border-b border-rule-hi bg-transparent px-1 py-0.5 text-ink-hi outline-none focus:border-rx"
          />
        </label>

        {error && <p className="border border-rose-500/40 bg-rose-500/10 px-2 py-1 text-[11px] text-rose-400">✗ {error}</p>}

        <button
          type="submit"
          disabled={busy || !password}
          className="mt-2 border border-rx bg-rx px-3 py-1 text-[11px] font-bold uppercase tracking-[0.04em] text-ground transition enabled:hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {busy ? 'kimlik doğrulanıyor…' : 'Giriş [Enter]'}
        </button>
        <p className="pt-3 text-[10px] text-tui-dim">
          Oturumlar 7 gün geçerlidir · sunucu yeniden başlarsa tekrar giriş gerekir · SSO/kurumsal hesap için kullanıcı adı girin
        </p>
      </form>
    </div>
  )
}
