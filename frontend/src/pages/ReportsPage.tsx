import { ReportCard } from '../components/ReportCard'
import { Card } from '../components/Card'

export function ReportsPage() {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Raporlar</h1>
        <span className="text-xs text-slate-500">HTML/PDF rapor üretimi</span>
      </div>

      <Card title="Rapor Üretimi" right={<span className="text-xs text-slate-500">HTML · PDF</span>}>
        <ReportCard />
      </Card>
    </div>
  )
}
