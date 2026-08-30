import type { Connection, RecordInfo, Snapshot, AlertEvent } from '../types'
import { formatNum } from '../lib/format'
import { StatCards } from '../components/StatCards'
import { ThroughputChart } from '../components/ThroughputChart'
import { EndpointsTable } from '../components/EndpointsTable'
import { ProtocolsCard, PortsCard } from '../components/ProtocolsPorts'
import { ConnectionsTable } from '../components/ConnectionsTable'
import { HistoryCard } from '../components/HistoryCard'
import { AICard } from '../components/AICard'
import { DnsCard } from '../components/DnsCard'
import { AlertsCard } from '../components/AlertsCard'
import { PcapCard } from '../components/PcapCard'
import { CompareCard } from '../components/CompareCard'
import { ReportCard } from '../components/ReportCard'
import { AgentsCard } from '../components/AgentsCard'
import { ProcessesCard } from '../components/ProcessesCard'
import { DevicesCard } from '../components/DevicesCard'
import { FlowsCard } from '../components/FlowsCard'
import { SyslogCard } from '../components/SyslogCard'
import { TopologyCard } from '../components/TopologyCard'
import { ComplianceCard } from '../components/ComplianceCard'
import { IsmsCard } from '../components/IsmsCard'
import { Card } from '../components/Card'

// "Tüm Kartlar" — Dashboard'a henüz taşınmamış, klasik tek-sayfa görünüm.
// Sayfalar aşamalı olarak buradan ayrı rotalara bölünecek.

export function AllCardsPage({
  stats,
  connections,
  alertEvents,
  record,
  historyRefresh,
  onHistoryRefresh,
}: {
  stats: Snapshot
  connections: Connection[]
  alertEvents: AlertEvent[]
  record: RecordInfo
  historyRefresh: number
  onHistoryRefresh: () => void
}) {
  return (
    <main className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <StatCards stats={stats} />

      <Card title="Ağ Topolojisi" right={<span className="text-xs text-slate-500">Faz 6 · LLDP/CDP/ARP + agent subnetleri</span>}>
        <TopologyCard refreshKey={historyRefresh} />
      </Card>

      <Card title="Agent Filosu" right={<span className="text-xs text-slate-500">merkezi izleme</span>}>
        <AgentsCard refreshKey={historyRefresh} />
      </Card>

      <Card title="Süreç Trafiği (Agentlar)" right={<span className="text-xs text-slate-500">Faz 2 · atıf</span>}>
        <ProcessesCard />
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Cihazlar (SNMP)" right={<span className="text-xs text-slate-500">router · switch · firewall · ap</span>}>
          <DevicesCard refreshKey={historyRefresh} />
        </Card>
        <div className="space-y-4">
          <Card title="NetFlow v5 Akışları" right={<span className="text-xs text-slate-500">son 15 dk · top 20</span>}>
            <FlowsCard />
          </Card>
          <Card title="Syslog Olayları" right={<span className="text-xs text-slate-500">RFC3164</span>}>
            <SyslogCard />
          </Card>
        </div>
      </div>

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

      <Card title="Aktif Bağlantılar" right={<span className="text-xs text-slate-500">sistem soketleri</span>}>
        <ConnectionsTable connections={connections} />
      </Card>

      <Card title="DNS Sorguları" right={<span className="text-xs text-slate-500">UDP/53 · canlı</span>}>
        <DnsCard domains={stats.top_domains} />
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Geçmiş (Veritabanı)">
          <HistoryCard refreshKey={historyRefresh} />
        </Card>
        <Card
          title="AI Analizi"
          right={
            <button onClick={onHistoryRefresh} className="text-xs text-slate-500 transition hover:text-slate-300">
              geçmişi yenile
            </button>
          }
        >
          <AICard onAnalyzed={onHistoryRefresh} />
        </Card>
      </div>

      <Card title="Karşılaştırmalı Görünüm">
        <CompareCard />
      </Card>

      <Card title="Rapor Üretimi" right={<span className="text-xs text-slate-500">HTML · PDF</span>}>
        <ReportCard />
      </Card>

      <Card title="PCAP Kayıt" right={<span className="text-xs text-slate-500">Wireshark uyumlu</span>}>
        <PcapCard record={record} canRecord={stats.running} />
      </Card>

      <Card title="Uyarılar" right={<span className="text-xs text-slate-500">{formatNum(alertEvents.length)} olay</span>}>
        <AlertsCard events={alertEvents} />
      </Card>

      <Card
        title="Uyumluluk (5651 + ISO 27001)"
        right={<span className="text-xs text-slate-500">imzalı loglar · delil paketi · inceleme</span>}
      >
        <ComplianceCard refreshKey={historyRefresh} />
      </Card>

      <Card
        title="ISMS Yönetişimi (ISO 27001)"
        right={<span className="text-xs text-slate-500">Faz 10 · risk · SoA · politika · denetim</span>}
      >
        <IsmsCard refreshKey={historyRefresh} />
      </Card>

      <footer className="pb-6 pt-2 text-center text-[11px] text-slate-600">
        bazNTMS · Go + Vite · SQLite kayıt · AI analiz · gopacket/libpcap
      </footer>
    </main>
  )
}
