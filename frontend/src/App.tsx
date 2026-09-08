import { useCallback, useEffect, useRef, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useLive } from './lib/useLive'
import { KeymapProvider } from './lib/KeymapContext'
import { DialogProvider } from './lib/dialog'
import { TuiHeader } from './components/TuiHeader'
import { TabBar } from './components/TabBar'
import { FnKeyBar } from './components/FnKeyBar'
import { ShellKeys } from './components/ShellKeys'
import { LoginScreen } from './components/LoginScreen'
import { DashboardPage } from './pages/DashboardPage'
import { AgentsListPage } from './pages/AgentsListPage'
import { AgentDetailPage } from './pages/AgentDetailPage'
import { ProcessDetailPage } from './pages/ProcessDetailPage'
import { DevicesPage } from './pages/DevicesPage'
import { DeviceDetailPage } from './pages/DeviceDetailPage'
import { TrafficFlowPage } from './pages/TrafficFlowPage'
import { GeoPage } from './pages/GeoPage'
import { TopologyPage } from './pages/TopologyPage'
import { AlertsPage } from './pages/AlertsPage'
import { AnomalyPage } from './pages/AnomalyPage'
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
  const [identity, setIdentity] = useState<{ username: string; role: string; site?: string } | null>(null)
  // authRequired=false → kimlik doğrulama kapalı (dev modu); sunucuda
  // requirePerm de herkesi geçirir, o yüzden yönetim UI'ı da açılır.
  const [authRequired, setAuthRequired] = useState(true)
  const [multiSite, setMultiSite] = useState(false)
  // -public-url: agent kurulum sihirbazı enroll komutunda bunu tercih eder
  const [publicUrl, setPublicUrl] = useState('')
  // Yönetim bölümü: admin VEYA site-admin (kendi sahası). Uyumluluk/ISMS
  // (saha-üstü yönetişim) yalnız global admin — S14.B (çoklu-saha).
  const isAdmin = !authRequired || identity?.role === 'admin' || identity?.role === 'site-admin'
  const canGovern = !authRequired || identity?.role === 'admin'
  // site-admin ise yönetim formları bu sahaya kilitli; global admin ise boş.
  const lockedSite = identity?.role === 'site-admin' ? (identity.site ?? '') : ''
  const { alertEvents, fleet, connected, reconnect } = useLive(
    useCallback(() => setAuthState('locked'), []),
  )

  // sayfa geçişinde/açılışta içerik hep baştan başlasın — <main> kendi kaydırma
  // konteyneri olduğu için React Router'ın varsayılan davranışı önceki
  // kaydırma konumunu koruyordu ("ortadan başlıyor")
  const mainRef = useRef<HTMLElement>(null)
  const { pathname } = useLocation()
  useEffect(() => {
    mainRef.current?.scrollTo(0, 0)
    window.scrollTo(0, 0)
  }, [pathname])
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
      .then(
        (d: {
          required: boolean
          authenticated: boolean
          username?: string
          role?: string
          site?: string
          multi_site?: boolean
          public_url?: string
        }) => {
          setAuthRequired(d.required)
          setMultiSite(!!d.multi_site)
          setPublicUrl(d.public_url ?? '')
          setAuthState(d.required && !d.authenticated ? 'locked' : 'open')
          if (d.authenticated && d.username) {
            setIdentity({ username: d.username, role: d.role ?? 'viewer', site: d.site ?? '' })
          }
        },
      )
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
    <KeymapProvider>
      <DialogProvider>
        <div className="grid h-screen grid-rows-[auto_auto_1fr_auto] bg-ground">
          <TuiHeader
            connected={connected}
            fleet={fleet}
            alertEvents={alertEvents}
            identity={identity}
            onLogout={logout}
          />
          <TabBar isAdmin={isAdmin} canGovern={canGovern} />

          <main ref={mainRef} className="min-w-0 overflow-x-hidden overflow-y-auto">
            <Routes>
              <Route path="/" element={<DashboardPage refreshKey={historyRefresh} alertEvents={alertEvents} fleet={fleet} />} />
              <Route path="/agentlar" element={<AgentsListPage />} />
              <Route path="/agentlar/:id" element={<AgentDetailPage />} />
              <Route path="/agentlar/:id/surec/:ad" element={<ProcessDetailPage />} />
              <Route path="/cihazlar" element={<DevicesPage refreshKey={historyRefresh} />} />
              <Route path="/cihazlar/:id" element={<DeviceDetailPage />} />
              <Route path="/akis" element={<TrafficFlowPage isAdmin={isAdmin} />} />
              <Route path="/cografi" element={<GeoPage />} />
              <Route path="/topoloji" element={<TopologyPage refreshKey={historyRefresh} />} />
              <Route path="/uyarilar" element={<AlertsPage alertEvents={alertEvents} />} />
              <Route path="/anomali" element={<AnomalyPage />} />
              <Route path="/raporlar" element={<ReportsPage />} />
              <Route path="/uyumluluk" element={<ComplianceOverviewPage refreshKey={historyRefresh} />} />
              <Route path="/uyumluluk/risk" element={<RiskRegisterPage />} />
              <Route path="/uyumluluk/soa" element={<SoaPage />} />
              <Route path="/uyumluluk/politikalar" element={<PoliciesPage />} />
              <Route path="/uyumluluk/denetimler" element={<AuditsPage />} />
              <Route path="/uyumluluk/yonetisim" element={<GovernancePage />} />

              {/* Yönetim (Faz 12) — admin/site-admin guard. site-admin kendi sahasına
                  kilitli (S14.B): lockedSite dolu ise site alanı sabit, admin rolü gizli. */}
              <Route path="/yonetim" element={<AdminGuard isAdmin={isAdmin} />}>
                <Route index element={<Navigate to="/yonetim/kullanicilar" replace />} />
                <Route path="kullanicilar" element={<UsersAdminPage lockedSite={lockedSite} multiSite={multiSite} />} />
                <Route path="tokenlar" element={<TokensAdminPage lockedSite={lockedSite} multiSite={multiSite} />} />
                <Route path="agent-ekle" element={<EnrollAdminPage lockedSite={lockedSite} multiSite={multiSite} publicUrl={publicUrl} />} />
                <Route path="denetim" element={<AuditAdminPage />} />
              </Route>

              <Route path="*" element={<NotFoundPage />} />
            </Routes>
          </main>

          <FnKeyBar />
        </div>
        <ShellKeys onLogout={logout} onRefresh={() => setHistoryRefresh((k) => k + 1)} />
      </DialogProvider>
    </KeymapProvider>
  )
}
