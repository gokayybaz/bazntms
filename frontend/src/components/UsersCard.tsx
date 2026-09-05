import { useCallback, useEffect, useRef, useState } from 'react'
import { errText } from '../lib/api'

interface User {
  id: number
  username: string
  role: string
  site: string
  enabled: boolean
  created_at: number
  last_login: number
}

const ROLES = ['admin', 'netops', 'analyst', 'viewer'] as const
const ROLE_LABEL: Record<string, string> = {
  admin: 'Yönetici',
  netops: 'Ağ Operatörü',
  analyst: 'Analist',
  viewer: 'İzleyici',
}

const DELETE_CONFIRM_MS = 4000
const inputCls =
  'rounded-md border border-slate-700/80 bg-slate-950 px-2.5 py-1.5 text-sm text-slate-200 outline-none placeholder:text-slate-600 focus:border-cyan-500/60'

export function UsersCard() {
  const [users, setUsers] = useState<User[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  // ekleme formu
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ username: '', password: '', role: 'viewer', site: '' })
  const [creating, setCreating] = useState(false)

  // satır içi şifre sıfırlama
  const [pwFor, setPwFor] = useState<number | null>(null)
  const [pwValue, setPwValue] = useState('')

  // iki-aşamalı silme
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null)
  const confirmTimer = useRef<number | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/users')
      if (res.status === 401 || res.status === 403) {
        setError('kullanıcı listesi alınamadı (yetki)')
        return
      }
      setUsers(await res.json())
      setLoaded(true)
      setError('')
    } catch {
      setError('kullanıcı listesi alınamadı')
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])
  useEffect(
    () => () => {
      if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    },
    [],
  )

  const flash = (msg: string) => {
    setNotice(msg)
    window.setTimeout(() => setNotice(''), 3000)
  }

  const patch = async (id: number, body: Record<string, unknown>, okMsg: string) => {
    setError('')
    const res = await fetch(`/api/v1/users/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      setError(await errText(res))
      return false
    }
    flash(okMsg)
    await load()
    return true
  }

  const create = async () => {
    setError('')
    if (!form.username || form.password.length < 8) {
      setError('kullanıcı adı zorunlu, şifre en az 8 karakter')
      return
    }
    setCreating(true)
    try {
      const res = await fetch('/api/v1/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        setError(await errText(res))
        return
      }
      flash(`${form.username} eklendi`)
      setForm({ username: '', password: '', role: 'viewer', site: '' })
      setShowForm(false)
      await load()
    } finally {
      setCreating(false)
    }
  }

  const remove = async (u: User) => {
    setError('')
    const res = await fetch(`/api/v1/users/${u.id}`, { method: 'DELETE' })
    if (!res.ok) {
      setError(await errText(res))
      return
    }
    flash(`${u.username} silindi`)
    await load()
  }

  const handleDeleteClick = (u: User) => {
    if (confirmTimer.current) window.clearTimeout(confirmTimer.current)
    if (confirmDeleteId === u.id) {
      setConfirmDeleteId(null)
      void remove(u)
      return
    }
    setConfirmDeleteId(u.id)
    confirmTimer.current = window.setTimeout(() => setConfirmDeleteId(null), DELETE_CONFIRM_MS)
  }

  const submitPw = async (id: number) => {
    if (pwValue.length < 8) {
      setError('şifre en az 8 karakter')
      return
    }
    if (await patch(id, { password: pwValue }, 'şifre güncellendi')) {
      setPwFor(null)
      setPwValue('')
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => setShowForm((v) => !v)}
          className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500/20"
        >
          {showForm ? 'Vazgeç' : '+ Kullanıcı Ekle'}
        </button>
        {notice && <span className="text-xs text-emerald-400">{notice}</span>}
        {error && <span className="text-xs text-rose-400">⚠ {error}</span>}
      </div>

      {showForm && (
        <div className="grid gap-2 rounded-lg border border-slate-800 bg-slate-950/50 p-3 sm:grid-cols-[1fr_1fr_auto_1fr_auto]">
          <input
            className={inputCls}
            placeholder="kullanıcı adı"
            autoComplete="off"
            value={form.username}
            onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
          />
          <input
            className={inputCls}
            type="password"
            placeholder="şifre (≥ 8)"
            autoComplete="new-password"
            value={form.password}
            onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
          />
          <select
            className={inputCls}
            value={form.role}
            onChange={(e) => setForm((f) => ({ ...f, role: e.target.value }))}
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {ROLE_LABEL[r]}
              </option>
            ))}
          </select>
          <input
            className={inputCls}
            placeholder="site (boş = tüm siteler)"
            value={form.site}
            onChange={(e) => setForm((f) => ({ ...f, site: e.target.value }))}
          />
          <button
            onClick={create}
            disabled={creating}
            className="rounded-md bg-cyan-700 px-3 py-1.5 text-sm font-semibold text-white transition enabled:hover:bg-cyan-400 enabled:hover:text-slate-950 disabled:opacity-40"
          >
            {creating ? 'Ekleniyor…' : 'Ekle'}
          </button>
        </div>
      )}

      {!loaded && !error ? (
        <p className="py-6 text-center text-sm text-slate-500">Yükleniyor…</p>
      ) : users.length === 0 ? (
        <p className="py-6 text-center text-sm text-slate-500">
          Henüz RBAC kullanıcısı yok — tek-şifre (bootstrap) modu devrede.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-[11px] uppercase tracking-wider text-slate-500">
                <th className="py-2 pr-3 font-medium">Kullanıcı</th>
                <th className="py-2 pr-3 font-medium">Rol</th>
                <th className="py-2 pr-3 font-medium">Site</th>
                <th className="py-2 pr-3 font-medium">Durum</th>
                <th className="py-2 pr-3 font-medium">Son Giriş</th>
                <th className="py-2 font-medium">İşlem</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-slate-800/60 last:border-0">
                  <td className="py-2 pr-3 font-mono text-slate-200">{u.username}</td>
                  <td className="py-2 pr-3">
                    <select
                      value={u.role}
                      onChange={(e) => patch(u.id, { role: e.target.value }, 'rol güncellendi')}
                      className="rounded border border-slate-700/80 bg-slate-950 px-1.5 py-1 text-xs text-slate-300 focus:border-cyan-500/60"
                    >
                      {ROLES.map((r) => (
                        <option key={r} value={r}>
                          {ROLE_LABEL[r]}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="py-2 pr-3 text-slate-400">{u.site || <span className="text-slate-600">tümü</span>}</td>
                  <td className="py-2 pr-3">
                    <button
                      onClick={() => patch(u.id, { enabled: !u.enabled }, u.enabled ? 'pasifleştirildi' : 'etkinleştirildi')}
                      className={`rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ring-1 transition ${
                        u.enabled
                          ? 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/20 hover:bg-emerald-500/20'
                          : 'bg-slate-500/10 text-slate-400 ring-slate-500/20 hover:bg-slate-500/20'
                      }`}
                    >
                      {u.enabled ? 'etkin' : 'pasif'}
                    </button>
                  </td>
                  <td className="py-2 pr-3 font-mono text-[11px] text-slate-500">
                    {u.last_login > 0 ? new Date(u.last_login * 1000).toLocaleString('tr-TR') : 'hiç'}
                  </td>
                  <td className="py-2">
                    {pwFor === u.id ? (
                      <span className="inline-flex items-center gap-1.5">
                        <input
                          className={inputCls + ' !py-1 !text-xs'}
                          type="password"
                          placeholder="yeni şifre"
                          autoComplete="new-password"
                          value={pwValue}
                          onChange={(e) => setPwValue(e.target.value)}
                        />
                        <button
                          onClick={() => submitPw(u.id)}
                          className="rounded border border-cyan-500/40 px-2 py-1 text-[11px] text-cyan-300 hover:bg-cyan-500/10"
                        >
                          Kaydet
                        </button>
                        <button
                          onClick={() => {
                            setPwFor(null)
                            setPwValue('')
                          }}
                          className="text-[11px] text-slate-500 hover:text-slate-300"
                        >
                          vazgeç
                        </button>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-2">
                        <button
                          onClick={() => {
                            setPwFor(u.id)
                            setPwValue('')
                          }}
                          className="text-[11px] text-slate-400 hover:text-slate-200"
                        >
                          şifre sıfırla
                        </button>
                        <button
                          onClick={() => handleDeleteClick(u)}
                          className={`text-[11px] transition ${
                            confirmDeleteId === u.id
                              ? 'font-semibold text-rose-400'
                              : 'text-slate-500 hover:text-rose-400'
                          }`}
                        >
                          {confirmDeleteId === u.id ? 'emin misiniz?' : 'sil'}
                        </button>
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
