import { ComplianceCard } from '../components/ComplianceCard'
import { IsmsCard } from '../components/IsmsCard'
import { Card } from '../components/Card'

export function CompliancePage({ refreshKey }: { refreshKey: number }) {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Uyumluluk</h1>
        <span className="text-xs text-slate-500">5651 + ISO 27001</span>
      </div>

      <Card
        title="Uyumluluk (5651 + ISO 27001)"
        right={<span className="text-xs text-slate-500">imzalı loglar · delil paketi · inceleme</span>}
      >
        <ComplianceCard refreshKey={refreshKey} />
      </Card>

      <Card
        title="ISMS Yönetişimi (ISO 27001)"
        right={<span className="text-xs text-slate-500">Faz 10 · risk · SoA · politika · denetim</span>}
      >
        <IsmsCard refreshKey={refreshKey} />
      </Card>
    </div>
  )
}
