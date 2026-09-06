import { ReportCard } from '../components/ReportCard'
import { EnterpriseReportCard } from '../components/EnterpriseReportCard'
import { Panel } from '../components/Panel'

export function ReportsPage() {
  return (
    <div className="mx-auto max-w-[1600px] space-y-3 px-4 py-3 font-mono">
      <div className="flex items-baseline gap-2">
        <h1 className="text-[13px] font-bold uppercase tracking-[0.06em] text-ink-hi">Raporlar</h1>
        <span className="hidden truncate text-[10px] text-tui-dim sm:inline">HTML/PDF rapor üretimi</span>
      </div>

      <Panel title="Kurumsal Rapor" right={<span className="text-[10px] text-tui-dim">SLA · kapasite · banding</span>}>
        <EnterpriseReportCard />
      </Panel>

      <Panel title="Uyumluluk Raporu" right={<span className="text-[10px] text-tui-dim">ISO 27001 + 5651</span>}>
        <div className="space-y-3">
          <a
            href="/api/report?type=compliance"
            target="_blank"
            rel="noreferrer"
            className="inline-block border border-rx/40 bg-rx/10 px-2.5 py-0.5 text-[11px] uppercase tracking-[0.04em] text-rx transition hover:bg-rx/20"
          >
            HTML Görüntüle
          </a>
          <p className="text-[11px] leading-relaxed text-tui-dim">
            ISO 27001 Annex A kontrol haritası + 5651 log imzalama durumu (son saatlik checkpoint, son günlük mühür, imzalı
            kayıt sayısı). Ham delil paketi (PII maskeleme, offline doğrulama) için{' '}
            <a href="/uyumluluk" className="text-rx hover:underline">
              Uyumluluk sayfasına
            </a>{' '}
            bakın.
          </p>
        </div>
      </Panel>

      <Panel title="Ağ Trafiği Raporu" right={<span className="text-[10px] text-tui-dim">agent filosu · NetFlow · süreç trafiği</span>}>
        <ReportCard />
      </Panel>
    </div>
  )
}
