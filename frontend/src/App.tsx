import { useCallback, useEffect, useState } from 'react'
import type { NetInterface } from './types'
import { useLive } from './lib/useLive'
import { formatNum } from './lib/format'
import { Header } from './components/Header'
import { StatCards } from './components/StatCards'
import { ThroughputChart } from './components/ThroughputChart'
import { EndpointsTable } from './components/EndpointsTable'
import { ProtocolsCard, PortsCard } from './components/ProtocolsPorts'
import { ConnectionsTable } from './components/ConnectionsTable'
import { HistoryCard } from './components/HistoryCard'
import { AICard } from './components/AICard'
import { DnsCard } from './components/DnsCard'
import { AlertsCard } from './components/AlertsCard'
import { PcapCard } from './components/PcapCard'
import { CompareCard } from './components/CompareCard'
import { ReportCard } from './components/ReportCard'
import { LoginScreen } from './components/LoginScreen'
import { AgentsCard } from './components/AgentsCard'
import { ProcessesCard } from './components/ProcessesCard'
import { Card } from './components/Card'

export default function App() {
  const [authState, setAuthState] = useState<'loading' | 'open' | 'locked'>('loading')
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
      .then((d: { required: boolean; authenticated: boolean }) => {
        setAuthState(d.required && !d.authenticated ? 'locked' : 'open')
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
        onSuccess={() => {
          setAuthState('open')
          reconnect()
        }}
      />
    )
  }

  return (
    <div className="min-h-screen">
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
      />

      <main className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        <StatCards stats={stats} />

        <Card title="Agent Filosu" right={<span className="text-xs text-slate-500">merkezi izleme</span>}>
          <AgentsCard refreshKey={historyRefresh} />
        </Card>

        <Card title="Süreç Trafiği (Agentlar)" right={<span className="text-xs text-slate-500">Faz 2 · atıf</span>}>
          <ProcessesCard />
        </Card>

        <Card title="Ağ Verimi">
          <ThroughputChart history={stats.history} running={stats.running} />
        </Card>

        <div className="grid gap-4 lg:grid-cols-3">
          <Card
            title="En Çok Trafik Üreten Uç Noktalar"
            className="lg:col-span-2"
            right={<span className="text-xs text-slate-500">{stats.top_endpoints.length} hedef</span>}
          >
            <EndpointsTable endpoints={stats.top_endpoints} />
          </Card>

          <div className="space-y-4">
            <Card title="Protokol Dağılımı">
              <ProtocolsCard stats={stats} />
            </Card>
            <Card title="En Yoğun Portlar">
              <PortsCard ports={stats.top_ports} />
            </Card>
          </div>
        </div>

        <Card
          title="Aktif Bağlantılar"
          right={<span className="text-xs text-slate-500">sistem soketleri</span>}
        >
          <ConnectionsTable connections={connections} />
        </Card>

        <Card
          title="DNS Sorguları"
          right={<span className="text-xs text-slate-500">UDP/53 · canlı</span>}
        >
          <DnsCard domains={stats.top_domains} />
        </Card>

        <div className="grid gap-4 lg:grid-cols-2">
          <Card title="Geçmiş (Veritabanı)">
            <HistoryCard refreshKey={historyRefresh} />
          </Card>
          <Card
            title="AI Analizi"
            right={
              <button
                onClick={() => setHistoryRefresh((k) => k + 1)}
                className="text-xs text-slate-500 transition hover:text-slate-300"
              >
                geçmişi yenile
              </button>
            }
          >
            <AICard onAnalyzed={() => setHistoryRefresh((k) => k + 1)} />
          </Card>
        </div>

        <Card title="Karşılaştırmalı Görünüm">
          <CompareCard />
        </Card>

        <Card title="Rapor Üretimi" right={<span className="text-xs text-slate-500">HTML · PDF</span>}>
          <ReportCard />
        </Card>

        <Card
          title="PCAP Kayıt"
          right={<span className="text-xs text-slate-500">Wireshark uyumlu</span>}
        >
          <PcapCard record={record} canRecord={stats.running} />
        </Card>

        <Card
          title="Uyarılar"
          right={<span className="text-xs text-slate-500">{formatNum(alertEvents.length)} olay</span>}
        >
          <AlertsCard events={alertEvents} />
        </Card>

        <footer className="pb-6 pt-2 text-center text-[11px] text-slate-600">
          bazNTMS · Go + Vite · SQLite kayıt · AI analiz · gopacket/libpcap
        </footer>
      </main>
    </div>
  )
}
