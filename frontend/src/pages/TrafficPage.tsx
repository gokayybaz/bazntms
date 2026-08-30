import type { Connection, RecordInfo, Snapshot } from '../types'
import { StatCards } from '../components/StatCards'
import { ThroughputChart } from '../components/ThroughputChart'
import { EndpointsTable } from '../components/EndpointsTable'
import { ProtocolsCard, PortsCard } from '../components/ProtocolsPorts'
import { ConnectionsTable } from '../components/ConnectionsTable'
import { HistoryCard } from '../components/HistoryCard'
import { AICard } from '../components/AICard'
import { DnsCard } from '../components/DnsCard'
import { PcapCard } from '../components/PcapCard'
import { CompareCard } from '../components/CompareCard'
import { Card } from '../components/Card'

export function TrafficPage({
  stats,
  connections,
  record,
  historyRefresh,
  onHistoryRefresh,
}: {
  stats: Snapshot
  connections: Connection[]
  record: RecordInfo
  historyRefresh: number
  onHistoryRefresh: () => void
}) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Trafik</h1>
        <span className="text-xs text-slate-500">yerel paket yakalama · uç noktalar · geçmiş · AI analizi</span>
      </div>

      <StatCards stats={stats} />

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

      <Card title="PCAP Kayıt" right={<span className="text-xs text-slate-500">Wireshark uyumlu</span>}>
        <PcapCard record={record} canRecord={stats.running} />
      </Card>
    </div>
  )
}
