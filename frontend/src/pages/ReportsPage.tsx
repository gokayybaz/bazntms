import { ReportCard } from '../components/ReportCard'
import { EnterpriseReportCard } from '../components/EnterpriseReportCard'
import { Card } from '../components/Card'

export function ReportsPage() {
  return (
    <div className="mx-auto max-w-7xl space-y-4 px-4 py-5">
      <div className="flex items-center gap-2">
        <h1 className="text-[13px] font-semibold uppercase tracking-widest text-slate-300">Raporlar</h1>
        <span className="text-xs text-slate-500">HTML/PDF rapor üretimi</span>
      </div>

      <Card
        title="Kurumsal Rapor"
        right={<span className="text-xs text-slate-500">SLA · kapasite · banding</span>}
      >
        <EnterpriseReportCard />
      </Card>

      <Card
        title="Uyumluluk Raporu"
        right={<span className="text-xs text-slate-500">ISO 27001 + 5651</span>}
      >
        <div className="space-y-3">
          <a
            href="/api/report?type=compliance"
            target="_blank"
            rel="noreferrer"
            className="inline-block rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-3.5 py-1.5 text-sm font-medium text-cyan-300 transition hover:bg-cyan-500/20"
          >
            HTML Görüntüle
          </a>
          <p className="text-xs text-slate-500">
            ISO 27001 Annex A kontrol haritası + 5651 log imzalama durumu (son saatlik checkpoint, son günlük mühür, imzalı
            kayıt sayısı). Ham delil paketi (PII maskeleme, offline doğrulama) için{' '}
            <a href="/uyumluluk" className="text-cyan-400 hover:underline">
              Uyumluluk sayfasına
            </a>{' '}
            bakın.
          </p>
        </div>
      </Card>

      <Card
        title="Ağ Trafiği Raporu"
        right={<span className="text-xs text-slate-500">agent filosu · NetFlow · süreç trafiği</span>}
      >
        <ReportCard />
      </Card>
    </div>
  )
}
