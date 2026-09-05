import { useCallback, useEffect, useState } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useLive } from './lib/useLive'
import { Header } from './components/Header'
import { Sidebar } from './components/Sidebar'
import { LoginScreen } from './components/LoginScreen'
import { DashboardPage } from './pages/DashboardPage'
import { AgentsListPage } from './pages/AgentsListPage'
import { AgentDetailPage } from './pages/AgentDetailPage'
import { DevicesPage } from './pages/DevicesPage'
import { DeviceDetailPage } from './pages/DeviceDetailPage'
import { TopologyPage } from './pages/TopologyPage'
import { AlertsPage } from './pages/AlertsPage'
import { ReportsPage } from './pages/ReportsPage'
import { ComplianceOverviewPage } from './pages/ComplianceOverviewPage'
import { RiskRegisterPage } from './pages/RiskRegisterPage'
import { SoaPage } from './pages/SoaPage'
import { PoliciesPage } from './pages/PoliciesPage'
import { AuditsPage } from './pages/AuditsPage'
import { GovernancePage } from './pages/GovernancePage'
import { NotFoundPage } from './pages/NotFoundPage'
import { AdminGuard } from './components/AdminGuard'
import { UsersAdminPage } from './pages/yonetim/UsersAdminPage'
import { TokensAdminPage } from './pages/yonetim/TokensAdminPage'
import { EnrollAdminPage } from './pages/yonetim/EnrollAdminPage'
import { AuditAdminPage } from './pages/yonetim/AuditAdminPage'

export default function App() {
  const [authState, setAuthState] = useState<'loading' | 'open' | 'locked'>('loading')
  const [identity, setIdentity] = useState<{ username: string; role: string } | null>(null)
  // authRequired=false → kimlik doğrulama kapalı (dev modu); sunucuda
  // requirePerm de herkesi geçirir, o yüzden yönetim UI'ı da açılır.
  const [authRequired, setAuthRequired] = useState(true)
  const isAdmin = !authRequired || identity?.role === 'admin'
  const { alertEvents, fleet, connected, reconnect } = useLive(
    useCallback(() => setAuthState('locked'), []),
  )
  const [historyRefresh, setHistoryRefresh] = useState(0)

  // Cihazlar/Topoloji/Uyumluluk gibi sayfalar kendi polling'i olmadan
  // yalnızca refreshKey değiştiğinde yeniden yükleniyor — önceden bu
  // Trafik sayfasındaki elle "geçmişi yenile" düğmesiyle tetikleniyordu;
  // sayfa kaldırılınca yerine periyodik otomatik yenileme kondu.
  useEffect(() => {
    if (authState !== 'open') return
    const id = window.setInterval(() => setHistoryRefresh((k) => k + 1), 20_000)
    return () => window.clearInterval(id)
  }, [authState])

  useEffect(() => {
    fetch('/api/auth/status')
      .then((r) => r.json())
      .then((d: { required: boolean; authenticated: boolean; username?: string; role?: string }) => {
        setAuthRequired(d.required)
        setAuthState(d.required && !d.authenticated ? 'locked' : 'open')
        if (d.authenticated && d.username) {
          setIdentity({ username: d.username, role: d.role ?? 'viewer' })
        }
      })
      .catch(() => setAuthState('open'))
  }, [])

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
      <Sidebar isAdmin={isAdmin} />
      <div className="min-w-0 flex-1">
        <Header connected={connected} onLogout={logout} identity={identity} />

        <Routes>
          <Route path="/" element={<DashboardPage refreshKey={historyRefresh} alertEvents={alertEvents} fleet={fleet} />} />
          <Route path="/agentlar" element={<AgentsListPage />} />
          <Route path="/agentlar/:id" element={<AgentDetailPage />} />
          <Route path="/cihazlar" element={<DevicesPage refreshKey={historyRefresh} />} />
          <Route path="/cihazlar/:id" element={<DeviceDetailPage />} />
          <Route path="/topoloji" element={<TopologyPage refreshKey={historyRefresh} />} />
          <Route path="/uyarilar" element={<AlertsPage alertEvents={alertEvents} />} />
          <Route path="/raporlar" element={<ReportsPage />} />
          <Route path="/uyumluluk" element={<ComplianceOverviewPage refreshKey={historyRefresh} />} />
          <Route path="/uyumluluk/risk" element={<RiskRegisterPage />} />
          <Route path="/uyumluluk/soa" element={<SoaPage />} />
          <Route path="/uyumluluk/politikalar" element={<PoliciesPage />} />
          <Route path="/uyumluluk/denetimler" element={<AuditsPage />} />
          <Route path="/uyumluluk/yonetisim" element={<GovernancePage />} />

          {/* Yönetim (Faz 12) — admin guard; kabuklar S12.2+ ile dolar */}
          <Route path="/yonetim" element={<AdminGuard isAdmin={isAdmin} />}>
            <Route index element={<Navigate to="/yonetim/kullanicilar" replace />} />
            <Route path="kullanicilar" element={<UsersAdminPage />} />
            <Route path="tokenlar" element={<TokensAdminPage />} />
            <Route path="agent-ekle" element={<EnrollAdminPage />} />
            <Route path="denetim" element={<AuditAdminPage />} />
          </Route>

          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </div>
    </div>
  )
}
