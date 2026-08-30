import { useCallback, useEffect, useState } from 'react'
import { Routes, Route } from 'react-router-dom'
import type { NetInterface } from './types'
import { useLive } from './lib/useLive'
import { Header } from './components/Header'
import { Sidebar } from './components/Sidebar'
import { LoginScreen } from './components/LoginScreen'
import { DashboardPage } from './pages/DashboardPage'
import { AgentsListPage } from './pages/AgentsListPage'
import { AgentDetailPage } from './pages/AgentDetailPage'
import { DevicesPage } from './pages/DevicesPage'
import { TopologyPage } from './pages/TopologyPage'
import { TrafficPage } from './pages/TrafficPage'
import { AlertsPage } from './pages/AlertsPage'
import { ReportsPage } from './pages/ReportsPage'
import { CompliancePage } from './pages/CompliancePage'

export default function App() {
  const [authState, setAuthState] = useState<'loading' | 'open' | 'locked'>('loading')
  const [identity, setIdentity] = useState<{ username: string; role: string } | null>(null)
  const { stats, connections, alertEvents, record, connected, reconnect } = useLive(
    useCallback(() => setAuthState('locked'), []),
  )
  const [interfaces, setInterfaces] = useState<NetInterface[]>([])
  const [selected, setSelected] = useState('')
  const [error, setError] = useState('')
  const [starting, setStarting] = useState(false)
  const [historyRefresh, setHistoryRefresh] = useState(0)

  useEffect(() => {
    fetch('/api/auth/status')
      .then((r) => r.json())
      .then((d: { required: boolean; authenticated: boolean; username?: string; role?: string }) => {
        setAuthState(d.required && !d.authenticated ? 'locked' : 'open')
        if (d.authenticated && d.username) {
          setIdentity({ username: d.username, role: d.role ?? 'viewer' })
        }
      })
      .catch(() => setAuthState('open'))
  }, [])

  useEffect(() => {
    if (authState !== 'open') return
    setError('')
    fetch('/api/interfaces')
      .then((r) => r.json())
      .then((list: NetInterface[]) => {
        setInterfaces(list)
        const pick = list.find((i) => i.up && i.can_sniff && !i.loopback)
        if (pick) setSelected(pick.name)
      })
      .catch(() => setError('Arayüz listesi alınamadı'))
  }, [authState])

  // oturum denetimi: WS handshake aninda dogrulanir; oturum sonradan
  // sona ererse burada fark edilip login ekranina donulur
  useEffect(() => {
    if (authState !== 'open') return
    const id = window.setInterval(() => {
      fetch('/api/auth/status')
        .then((r) => r.json())
        .then((d: { required: boolean; authenticated: boolean }) => {
          if (d.required && !d.authenticated) setAuthState('locked')
        })
        .catch(() => {})
    }, 60_000)
    return () => window.clearInterval(id)
  }, [authState])

  const start = useCallback(async () => {
    if (!selected) return
    setStarting(true)
    setError('')
    try {
      const res = await fetch('/api/capture/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ device: selected }),
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data)
      if (data.error) setError(data.error)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setStarting(false)
    }
  }, [selected])

  const stop = useCallback(async () => {
    await fetch('/api/capture/stop', { method: 'POST' }).catch(() => {})
    setError('')
  }, [])

  const logout = useCallback(async () => {
    await fetch('/api/logout', { method: 'POST' }).catch(() => {})
    setAuthState('locked')
  }, [])

  if (authState === 'loading') {
    return <div className="min-h-screen" />
  }
  if (authState === 'locked') {
    return (
      <LoginScreen
        onSuccess={(ident) => {
          setIdentity(ident)
          setAuthState('open')
          reconnect()
        }}
      />
    )
  }

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <div className="min-w-0 flex-1">
        <Header
          interfaces={interfaces}
          selected={selected}
          onSelect={(n) => {
            setSelected(n)
            setError('')
          }}
          running={stats.running}
          error={error || stats.error}
          connected={connected}
          onStart={start}
          onStop={stop}
          starting={starting}
          onLogout={logout}
          identity={identity}
        />

        <Routes>
          <Route path="/" element={<DashboardPage refreshKey={historyRefresh} alertEvents={alertEvents} />} />
          <Route path="/agentlar" element={<AgentsListPage />} />
          <Route path="/agentlar/:id" element={<AgentDetailPage />} />
          <Route path="/cihazlar" element={<DevicesPage refreshKey={historyRefresh} />} />
          <Route path="/topoloji" element={<TopologyPage refreshKey={historyRefresh} />} />
          <Route
            path="/trafik"
            element={
              <TrafficPage
                stats={stats}
                connections={connections}
                record={record}
                historyRefresh={historyRefresh}
                onHistoryRefresh={() => setHistoryRefresh((k) => k + 1)}
              />
            }
          />
          <Route path="/uyarilar" element={<AlertsPage alertEvents={alertEvents} />} />
          <Route path="/raporlar" element={<ReportsPage />} />
          <Route path="/uyumluluk" element={<CompliancePage refreshKey={historyRefresh} />} />
        </Routes>
      </div>
    </div>
  )
}
